package messages

import (
	"context"
	"errors"
	"net"
	"net/http"
	"testing"
	"time"

	"golang.org/x/net/http2"

	"github.com/nopperabbo/kiroxy/internal/kiroclient"
)

func TestPoolRotationBackoff(t *testing.T) {
	if d := poolRotationBackoff(0); d != 0 {
		t.Fatalf("attempt 0 should be 0, got %v", d)
	}
	d1 := poolRotationBackoff(1)
	if d1 < poolRotationBaseDelay*3/4 || d1 > poolRotationBaseDelay*5/4 {
		t.Fatalf("attempt 1 out of jitter range: %v", d1)
	}
	d2 := poolRotationBackoff(2)
	if d2 < poolRotationBaseDelay*3/2 || d2 > poolRotationBaseDelay*5/2 {
		t.Fatalf("attempt 2 out of range: %v", d2)
	}
}

func TestRotationSleep_Cancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := rotationSleep(ctx, time.Second)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

func TestRotationSleep_Zero(t *testing.T) {
	if err := rotationSleep(context.Background(), 0); err != nil {
		t.Fatalf("expected nil for zero delay, got %v", err)
	}
}

func TestIsRotatableUpstreamError(t *testing.T) {
	tests := []struct {
		name    string
		err     error
		rotable bool
	}{
		{"nil error", nil, false},
		{"non-upstream error", errors.New("random"), false},
		{"ThrottlingException", &kiroclient.UpstreamError{Exception: "ThrottlingException", Status: 429}, true},
		{"InternalServerException", &kiroclient.UpstreamError{Exception: "InternalServerException", Status: 500}, true},
		{"502 no exception", &kiroclient.UpstreamError{Status: http.StatusBadGateway}, true},
		{"503 no exception", &kiroclient.UpstreamError{Status: http.StatusServiceUnavailable}, true},
		{"504 no exception", &kiroclient.UpstreamError{Status: http.StatusGatewayTimeout}, true},
		{"401 unauthorized", &kiroclient.UpstreamError{Status: http.StatusUnauthorized}, true},
		{"403 forbidden", &kiroclient.UpstreamError{Status: http.StatusForbidden}, true},
		{"402 payment required", &kiroclient.UpstreamError{Status: http.StatusPaymentRequired}, true},
		{"ServiceQuotaExceededException", &kiroclient.UpstreamError{Exception: "ServiceQuotaExceededException", Status: 400}, true},
		{"ValidationException", &kiroclient.UpstreamError{Exception: "ValidationException", Status: 400}, false},
		{"400 unknown exception", &kiroclient.UpstreamError{Exception: "SomeOtherException", Status: 400}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, got := isRotatableUpstreamError(tt.err)
			if got != tt.rotable {
				t.Errorf("isRotatableUpstreamError() = %v, want %v", got, tt.rotable)
			}
		})
	}
}

func TestClassifyTransportError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		tag  string
		ok   bool
	}{
		{"nil", nil, "", false},
		{"random error", errors.New("oops"), "", false},
		{"GOAWAY", http2.GoAwayError{LastStreamID: 1}, "Http2:GOAWAY", true},
		{"REFUSED_STREAM", http2.StreamError{Code: http2.ErrCodeRefusedStream}, "Http2:REFUSED_STREAM", true},
		{"ENHANCE_YOUR_CALM", http2.StreamError{Code: http2.ErrCodeEnhanceYourCalm}, "Http2:ENHANCE_YOUR_CALM", true},
		{"CANCEL", http2.StreamError{Code: http2.ErrCodeCancel}, "Http2:CANCEL", true},
		{"INTERNAL_ERROR", http2.StreamError{Code: http2.ErrCodeInternal}, "Http2:INTERNAL_ERROR", true},
		{"deadline exceeded", context.DeadlineExceeded, "TransportTimeout", true},
		{"net timeout", &netTimeoutErr{}, "TransportTimeout", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tag, ok := classifyTransportError(tt.err)
			if ok != tt.ok || tag != tt.tag {
				t.Errorf("got (%q, %v), want (%q, %v)", tag, ok, tt.tag, tt.ok)
			}
		})
	}
}

type netTimeoutErr struct{}

func (e *netTimeoutErr) Error() string   { return "i/o timeout" }
func (e *netTimeoutErr) Timeout() bool   { return true }
func (e *netTimeoutErr) Temporary() bool { return true }

var _ net.Error = (*netTimeoutErr)(nil)

func TestIsCapacityShortage(t *testing.T) {
	tests := []struct {
		name string
		ue   *kiroclient.UpstreamError
		want bool
	}{
		{"nil", nil, false},
		{"explicit reason", &kiroclient.UpstreamError{Reason: "INSUFFICIENT_MODEL_CAPACITY"}, true},
		{"heuristic body", &kiroclient.UpstreamError{Exception: "ThrottlingException", Body: "experiencing high traffic right now"}, true},
		{"throttling without capacity", &kiroclient.UpstreamError{Exception: "ThrottlingException", Reason: "RATE_LIMIT"}, false},
		{"other exception", &kiroclient.UpstreamError{Exception: "InternalServerException"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isCapacityShortage(tt.ue); got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsQuotaFailure(t *testing.T) {
	tests := []struct {
		name string
		ue   *kiroclient.UpstreamError
		want bool
	}{
		{"ThrottlingException", &kiroclient.UpstreamError{Exception: "ThrottlingException"}, true},
		{"TooManyRequestsException", &kiroclient.UpstreamError{Exception: "TooManyRequestsException"}, true},
		{"ServiceQuotaExceededException", &kiroclient.UpstreamError{Exception: "ServiceQuotaExceededException"}, true},
		{"429 status", &kiroclient.UpstreamError{Status: 429}, true},
		{"402 status", &kiroclient.UpstreamError{Status: 402}, true},
		{"capacity shortage excluded", &kiroclient.UpstreamError{Exception: "ThrottlingException", Reason: "INSUFFICIENT_MODEL_CAPACITY"}, false},
		{"500 not quota", &kiroclient.UpstreamError{Status: 500, Exception: "InternalServerException"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isQuotaFailure(tt.ue); got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsStructuralError(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		structural bool
	}{
		{"nil", nil, false},
		{"random error", errors.New("oops"), false},
		{"UnknownOperationException", &kiroclient.UpstreamError{Exception: "UnknownOperationException"}, true},
		{"AccessDeniedException", &kiroclient.UpstreamError{Exception: "AccessDeniedException"}, true},
		{"ValidationException excluded", &kiroclient.UpstreamError{Exception: "ValidationException"}, false},
		{"ThrottlingException excluded", &kiroclient.UpstreamError{Exception: "ThrottlingException"}, false},
		{"InternalServerException excluded", &kiroclient.UpstreamError{Exception: "InternalServerException"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, got := isStructuralError(tt.err)
			if got != tt.structural {
				t.Errorf("got %v, want %v", got, tt.structural)
			}
		})
	}
}

func TestRotationFailureReason(t *testing.T) {
	tests := []struct {
		name string
		ue   *kiroclient.UpstreamError
		want string
	}{
		{"exception+reason", &kiroclient.UpstreamError{Exception: "Throttling", Reason: "RATE"}, "upstream_exception:Throttling/RATE"},
		{"exception only", &kiroclient.UpstreamError{Exception: "Throttling"}, "upstream_exception:Throttling"},
		{"status only", &kiroclient.UpstreamError{Status: 502}, "upstream_status:Bad Gateway"},
		{"empty", &kiroclient.UpstreamError{}, "upstream_error"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := rotationFailureReason(tt.ue); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}
