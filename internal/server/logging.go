package server

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"
)

type ctxKey int

const ctxKeyRequestID ctxKey = iota + 1

func requestIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(ctxKeyRequestID).(string); ok {
		return v
	}
	return ""
}

type loggingMiddleware struct {
	logger *slog.Logger
}

func newLoggingMiddleware(logger *slog.Logger) *loggingMiddleware {
	return &loggingMiddleware{logger: logger}
}

func (m *loggingMiddleware) wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqID := strings.TrimSpace(r.Header.Get("X-Request-Id"))
		if reqID == "" {
			reqID = newULID()
		}
		ctx := context.WithValue(r.Context(), ctxKeyRequestID, reqID)
		r = r.WithContext(ctx)
		w.Header().Set("X-Request-Id", reqID)

		lw := &loggingResponseWriter{ResponseWriter: w, status: 200}
		start := time.Now()
		next.ServeHTTP(lw, r)
		latency := time.Since(start)

		if r.URL.Path == "/healthz" && lw.status == http.StatusOK {
			return
		}

		m.logger.LogAttrs(ctx, slog.LevelInfo, "http request",
			slog.String("request_id", reqID),
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.Int("status", lw.status),
			slog.Int64("latency_ms", latency.Milliseconds()),
			slog.Int64("bytes_out", lw.written),
			slog.String("remote_ip", clientIP(r)),
			slog.String("user_agent", r.Header.Get("User-Agent")),
		)
	})
}

type loggingResponseWriter struct {
	http.ResponseWriter
	status   int
	written  int64
	flushed  bool
	muStatus sync.Mutex
}

func (w *loggingResponseWriter) WriteHeader(status int) {
	w.muStatus.Lock()
	w.status = status
	w.muStatus.Unlock()
	w.ResponseWriter.WriteHeader(status)
}

func (w *loggingResponseWriter) Write(b []byte) (int, error) {
	n, err := w.ResponseWriter.Write(b)
	w.written += int64(n)
	return n, err
}

func (w *loggingResponseWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
		w.flushed = true
	}
}

// newULID returns a 26-char Crockford base32 ULID. We avoid pulling in a full
// ULID library for a handful of ids; this is a correct minimal implementation:
// 48 bits of unix-ms timestamp + 80 bits of randomness.
func newULID() string {
	now := uint64(time.Now().UnixMilli())
	var id [16]byte
	binary.BigEndian.PutUint64(id[0:8], now<<16)
	if _, err := rand.Read(id[6:]); err != nil {
		return hex.EncodeToString(id[:])
	}
	binary.BigEndian.PutUint64(id[0:8], now<<16)
	return crockfordBase32Encode(id[:])
}

const crockfordAlphabet = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

func crockfordBase32Encode(data []byte) string {
	var out strings.Builder
	out.Grow(26)
	bits := 0
	value := uint64(0)
	for _, b := range data {
		value = (value << 8) | uint64(b)
		bits += 8
		for bits >= 5 {
			bits -= 5
			idx := (value >> uint(bits)) & 0x1f
			out.WriteByte(crockfordAlphabet[idx])
		}
	}
	if bits > 0 {
		idx := (value << uint(5-bits)) & 0x1f
		out.WriteByte(crockfordAlphabet[idx])
	}
	s := out.String()
	if len(s) > 26 {
		s = s[:26]
	}
	return s
}

func clientIP(r *http.Request) string {
	if v := r.Header.Get("X-Forwarded-For"); v != "" {
		if i := strings.IndexByte(v, ','); i > 0 {
			return strings.TrimSpace(v[:i])
		}
		return strings.TrimSpace(v)
	}
	host, _, err := splitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func splitHostPort(hp string) (host, port string, err error) {
	i := strings.LastIndexByte(hp, ':')
	if i < 0 {
		return hp, "", nil
	}
	return hp[:i], hp[i+1:], nil
}
