package kiroclient

import (
	"crypto/tls"
	"net/http/httptrace"
	"time"
)

// PhaseTimings captures connection-level timing phases for a single HTTP request.
type PhaseTimings struct {
	DNS     time.Duration
	TLS     time.Duration
	Connect time.Duration
	TTFB    time.Duration // time from request written to first response byte
}

// PhaseObserver is called with phase timings after a successful response.
type PhaseObserver func(PhaseTimings)

func WithPhaseObserver(fn PhaseObserver) HTTPClientOption {
	return func(c *HTTPClient) { c.phaseObserver = fn }
}

func newPhaseTrace() (*httptrace.ClientTrace, *phaseState) {
	s := &phaseState{}
	trace := &httptrace.ClientTrace{
		DNSStart:             func(_ httptrace.DNSStartInfo) { s.dnsStart = time.Now() },
		DNSDone:              func(_ httptrace.DNSDoneInfo) { s.dnsDone = time.Now() },
		ConnectStart:         func(_, _ string) { s.connectStart = time.Now() },
		ConnectDone:          func(_, _ string, _ error) { s.connectDone = time.Now() },
		TLSHandshakeStart:    func() { s.tlsStart = time.Now() },
		TLSHandshakeDone:     func(_ tls.ConnectionState, _ error) { s.tlsDone = time.Now() },
		GotFirstResponseByte: func() { s.firstByte = time.Now() },
		WroteRequest:         func(_ httptrace.WroteRequestInfo) { s.wroteRequest = time.Now() },
	}
	return trace, s
}

type phaseState struct {
	dnsStart     time.Time
	dnsDone      time.Time
	connectStart time.Time
	connectDone  time.Time
	tlsStart     time.Time
	tlsDone      time.Time
	wroteRequest time.Time
	firstByte    time.Time
}

func (s *phaseState) timings() PhaseTimings {
	var t PhaseTimings
	if !s.dnsStart.IsZero() && !s.dnsDone.IsZero() {
		t.DNS = s.dnsDone.Sub(s.dnsStart)
	}
	if !s.connectStart.IsZero() && !s.connectDone.IsZero() {
		t.Connect = s.connectDone.Sub(s.connectStart)
	}
	if !s.tlsStart.IsZero() && !s.tlsDone.IsZero() {
		t.TLS = s.tlsDone.Sub(s.tlsStart)
	}
	if !s.wroteRequest.IsZero() && !s.firstByte.IsZero() {
		t.TTFB = s.firstByte.Sub(s.wroteRequest)
	}
	return t
}
