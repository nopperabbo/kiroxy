package messages

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandleUpstreamError(t *testing.T) {
	tests := []struct {
		name          string
		isException   bool
		invalidReason string
		wantRetry     string
		wantStatus    int
	}{
		{"exception returns 502", true, "", "", http.StatusBadGateway},
		{"retryable CONTENT_LENGTH", false, "CONTENT_LENGTH_EXCEEDS_THRESHOLD", "CONTENT_LENGTH_EXCEEDS_THRESHOLD", 0},
		{"retryable INVALID_CONVERSATION_STATE", false, "INVALID_CONVERSATION_STATE", "INVALID_CONVERSATION_STATE", 0},
		{"retryable STALE_CONVERSATION", false, "STALE_CONVERSATION", "STALE_CONVERSATION", 0},
		{"non-retryable reason returns 400", false, "SOME_OTHER_REASON", "", http.StatusBadRequest},
		{"empty reason returns 400", false, "", "", http.StatusBadRequest},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			got := handleUpstreamError(w, tt.isException, tt.invalidReason)
			if got != tt.wantRetry {
				t.Errorf("retry reason = %q, want %q", got, tt.wantRetry)
			}
			if tt.wantStatus != 0 && w.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", w.Code, tt.wantStatus)
			}
		})
	}
}
