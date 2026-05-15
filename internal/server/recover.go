package server

import (
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/nopperabbo/kiroxy/internal/httpx"
)

// recoverMiddleware catches panics from any downstream handler, logs them
// with a full stack trace and the request_id (if assigned by the logging
// middleware), and returns an Anthropic-shaped error envelope when the
// response has not yet started.
//
// Without this, Go's net/http per-connection recover swallows panics with
// no stack trace, no request_id, and the client sees a connection RST
// instead of a typed error. SSE streams that already wrote 200 OK are
// truncated mid-stream with no error frame.
type recoverMiddleware struct {
	logger *slog.Logger
}

func newRecoverMiddleware(logger *slog.Logger) *recoverMiddleware {
	if logger == nil {
		logger = slog.Default()
	}
	return &recoverMiddleware{logger: logger}
}

func (m *recoverMiddleware) wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ww := &recordingResponseWriter{ResponseWriter: w}
		defer func() {
			rec := recover()
			if rec == nil {
				return
			}
			stack := debug.Stack()
			m.logger.LogAttrs(r.Context(), slog.LevelError, "panic recovered",
				slog.Any("panic", rec),
				slog.String("path", r.URL.Path),
				slog.String("method", r.Method),
				slog.String("request_id", requestIDFromContext(r.Context())),
				slog.String("stack", string(stack)),
			)
			if !ww.headerSent {
				httpx.WriteError(w, http.StatusInternalServerError,
					"api_error", "internal server error")
			}
		}()
		next.ServeHTTP(ww, r)
	})
}

// recordingResponseWriter tracks whether headers have been written so the
// recover handler knows whether it's safe to emit an error envelope.
type recordingResponseWriter struct {
	http.ResponseWriter
	headerSent bool
}

func (w *recordingResponseWriter) WriteHeader(status int) {
	w.headerSent = true
	w.ResponseWriter.WriteHeader(status)
}

func (w *recordingResponseWriter) Write(b []byte) (int, error) {
	w.headerSent = true
	return w.ResponseWriter.Write(b)
}

func (w *recordingResponseWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (w *recordingResponseWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}
