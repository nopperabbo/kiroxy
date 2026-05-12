// OpenAI-compatible error envelope. The OpenAI shape is fundamentally
// different from Anthropic's {type:"error", error:{...}} wrapper used by
// httpx.WriteError, so we write our own on OpenAI routes.

package openai

import (
	"encoding/json/v2"
	"log/slog"
	"net/http"
)

// Error is the outer body written on OpenAI error responses.
type Error struct {
	Error ErrorDetail `json:"error"`
}

// ErrorDetail is the inner error object.
type ErrorDetail struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Param   string `json:"param,omitempty"`
	Code    string `json:"code,omitempty"`
}

// WriteError writes an OpenAI-compatible JSON error response. errType should
// be one of ErrTypeInvalidRequest, ErrTypeAPI, ErrTypeAuthentication. param
// and code are optional; pass empty strings to omit.
func WriteError(w http.ResponseWriter, status int, errType, message, param, code string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	body := Error{Error: ErrorDetail{
		Message: message,
		Type:    errType,
		Param:   param,
		Code:    code,
	}}
	if err := json.MarshalWrite(w, body); err != nil {
		slog.Error("openai: write error response failed", "err", err)
		return
	}
	_, _ = w.Write([]byte("\n"))
}

// StatusToType maps an HTTP status code to an OpenAI error type. Falls back
// to api_error for unrecognized statuses.
func StatusToType(status int) string {
	switch {
	case status == http.StatusUnauthorized || status == http.StatusForbidden:
		return ErrTypeAuthentication
	case status >= 400 && status < 500:
		return ErrTypeInvalidRequest
	default:
		return ErrTypeAPI
	}
}
