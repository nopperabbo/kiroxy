package server

import (
	"bytes"
	"encoding/json/v2"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/google/uuid"

	"local/kiroxy/internal/openai"
)

// handleChatCompletions implements POST /v1/chat/completions. It parses the
// OpenAI request, translates to an Anthropic request, delegates to the
// existing messages.Service via a synthetic http.Request, and translates
// the response back to OpenAI shape on the fly.
//
// When /v1/messages is unavailable (no auth configured), we return the same
// 503 shape the Anthropic surface returns but wrapped in OpenAI's error
// envelope so clients see a coherent error.
func (s *Server) handleChatCompletions(w http.ResponseWriter, r *http.Request) {
	if s.msgSvc == nil {
		openai.WriteError(w, http.StatusServiceUnavailable, openai.ErrTypeAuthentication,
			"no Kiro account configured; see /dashboard or kiroxy add-account", "", "")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 4<<20)
	var req openai.ChatCompletionRequest
	if err := json.UnmarshalRead(r.Body, &req); err != nil {
		openai.WriteError(w, http.StatusBadRequest, openai.ErrTypeInvalidRequest,
			"invalid JSON: "+err.Error(), "", "")
		return
	}

	anthrReq, err := openai.TranslateRequest(&req)
	if err != nil {
		if openai.WriteValidationError(w, err) {
			return
		}
		openai.WriteError(w, http.StatusBadRequest, openai.ErrTypeInvalidRequest, err.Error(), "", "")
		return
	}

	body, err := json.Marshal(anthrReq)
	if err != nil {
		openai.WriteError(w, http.StatusInternalServerError, openai.ErrTypeAPI,
			"failed to serialize translated request", "", "")
		return
	}

	syntheticReq := buildSyntheticMessagesRequest(r, body)

	if req.Stream {
		s.streamChatCompletion(w, syntheticReq, &req)
		return
	}
	s.bufferedChatCompletion(w, syntheticReq, &req)
}

// handleListModels implements GET /v1/models.
func (s *Server) handleListModels(w http.ResponseWriter, _ *http.Request) {
	list := openai.ListModels()
	w.Header().Set("Content-Type", "application/json")
	if err := json.MarshalWrite(w, list); err != nil {
		slog.Error("openai: encode model list", "err", err)
	}
}

// buildSyntheticMessagesRequest constructs the internal /v1/messages request
// that messages.Service.HandleMessages expects. It copies authn/context
// headers from the client's request and injects a session ID when the
// client did not supply one (OpenAI SDKs do not set X-Claude-Code-Session-Id).
func buildSyntheticMessagesRequest(client *http.Request, body []byte) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(body))
	req = req.WithContext(client.Context())
	// Copy headers that influence pipeline behavior: auth, content-type,
	// traceability, session, anthropic-beta.
	for _, h := range []string{
		"Content-Type",
		"Authorization",
		"X-Api-Key",
		"X-Claude-Code-Session-Id",
		"Anthropic-Beta",
		"User-Agent",
		"X-Request-Id",
	} {
		for _, v := range client.Header.Values(h) {
			req.Header.Add(h, v)
		}
	}
	if req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if req.Header.Get("X-Claude-Code-Session-Id") == "" {
		req.Header.Set("X-Claude-Code-Session-Id", "openai-"+uuid.New().String())
	}
	return req
}

// bufferedChatCompletion handles the non-streaming path. The Anthropic
// pipeline always writes non-streaming responses as a single JSON blob; we
// capture that blob and translate before writing to the real client.
func (s *Server) bufferedChatCompletion(w http.ResponseWriter, syntheticReq *http.Request, origReq *openai.ChatCompletionRequest) {
	buf := httptest.NewRecorder()
	s.msgSvc.HandleMessages(buf, syntheticReq)

	if buf.Code >= 400 {
		relayErrorResponse(w, buf)
		return
	}

	body := buf.Body.Bytes()
	resp, err := openai.TranslateResponse(body, origReq.Model)
	if err != nil {
		slog.ErrorContext(syntheticReq.Context(), "openai: translate response", "err", err)
		openai.WriteError(w, http.StatusBadGateway, openai.ErrTypeAPI,
			"upstream response was not well-formed", "", "")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.MarshalWrite(w, resp); err != nil {
		slog.ErrorContext(syntheticReq.Context(), "openai: write response", "err", err)
	}
}

// streamChatCompletion handles the streaming path. We use a pipe so the
// Anthropic pipeline can write SSE bytes (via the intercepting writer's
// Write/Flush) while the translator concurrently reads and emits OpenAI
// chunks to the real client.
func (s *Server) streamChatCompletion(w http.ResponseWriter, syntheticReq *http.Request, origReq *openai.ChatCompletionRequest) {
	pr, pw := io.Pipe()

	translator := openai.NewStreamTranslator(w, origReq.Model)

	// writer intercepts bytes written by the Anthropic pipeline and streams
	// them into the pipe. We capture the first write's status code so we
	// can decide on a buffered error response if the pipeline 4xx'd before
	// it emitted any SSE bytes.
	iw := &interceptingWriter{
		pipeWriter: pw,
		upstream:   w,
		header:     http.Header{},
		statusCode: http.StatusOK,
	}

	done := make(chan error, 1)
	go func() {
		defer close(done)
		done <- translator.Translate(syntheticReq.Context(), pr)
	}()

	// Run the pipeline. It writes to iw; iw proxies to pw (for the translator)
	// unless the pipeline signals a non-2xx status before any bytes are
	// written, in which case iw switches to buffering mode and we handle
	// the error after HandleMessages returns.
	s.msgSvc.HandleMessages(iw, syntheticReq)

	// Closing the pipe signals EOF to the translator so it can emit [DONE].
	_ = pw.Close()
	<-done

	if iw.bufferedErr {
		relayErrorResponseRaw(w, iw.statusCode, iw.header, iw.buffer.Bytes())
	}
}

// interceptingWriter is a tiny http.ResponseWriter that forwards SSE writes
// from the Anthropic pipeline into a pipe for the StreamTranslator. If the
// pipeline writes a non-2xx status before any bytes, we flip to buffering
// so the handler can emit an OpenAI-shape error to the real client instead
// of a partial SSE stream.
type interceptingWriter struct {
	pipeWriter  *io.PipeWriter
	upstream    http.ResponseWriter
	header      http.Header
	statusCode  int
	wroteHeader bool

	// When bufferedErr is true, writes go to buffer instead of pipeWriter.
	bufferedErr bool
	buffer      bytes.Buffer
}

// Header implements http.ResponseWriter.
func (w *interceptingWriter) Header() http.Header { return w.header }

// WriteHeader implements http.ResponseWriter. A non-2xx status switches the
// writer into error-buffering mode so the caller can emit a coherent
// OpenAI error after the handler unwinds.
func (w *interceptingWriter) WriteHeader(code int) {
	if w.wroteHeader {
		return
	}
	w.wroteHeader = true
	w.statusCode = code
	if code >= 400 {
		w.bufferedErr = true
	}
}

// Write implements http.ResponseWriter.
func (w *interceptingWriter) Write(p []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	if w.bufferedErr {
		return w.buffer.Write(p)
	}
	return w.pipeWriter.Write(p)
}

// Flush implements http.Flusher.
func (w *interceptingWriter) Flush() {
	// No-op: the pipe semantics push to the reader immediately when data is
	// available. The real client's Flush is driven by the translator.
}

// relayErrorResponse copies an Anthropic error response into OpenAI shape.
// This runs on the non-streaming path when the pipeline wrote a 4xx/5xx
// before producing a body.
func relayErrorResponse(w http.ResponseWriter, rec *httptest.ResponseRecorder) {
	relayErrorResponseRaw(w, rec.Code, rec.Header(), rec.Body.Bytes())
}

// relayErrorResponseRaw is the core of relayErrorResponse, factored out so
// the streaming path (which has its own header/buffer capture) can reuse it.
func relayErrorResponseRaw(w http.ResponseWriter, status int, hdr http.Header, body []byte) {
	// Try to pull a human-friendly message out of the Anthropic error JSON.
	msg := deriveAnthropicErrorMessage(body)
	openai.WriteError(w, status, openai.StatusToType(status), msg, "", "")
	_ = hdr // reserved for future header pass-through
}

// deriveAnthropicErrorMessage extracts the inner error.message field from an
// Anthropic-shape error JSON body, falling back to the raw body or a
// default string. Never returns an empty string.
func deriveAnthropicErrorMessage(body []byte) string {
	body = bytes.TrimSpace(body)
	if len(body) == 0 {
		return "upstream error"
	}
	var wrapper struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &wrapper); err == nil && wrapper.Error.Message != "" {
		return wrapper.Error.Message
	}
	// As a last resort, echo the body if it looks like plain text.
	if s := strings.TrimSpace(string(body)); s != "" {
		return s
	}
	return "upstream error"
}
