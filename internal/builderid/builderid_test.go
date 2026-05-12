package builderid

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

type fakeOIDC struct {
	registerCalls  atomic.Int32
	deviceCalls    atomic.Int32
	tokenCalls     atomic.Int32
	tokenBehaviour func(call int32) (int, map[string]any)
}

func newFakeOIDC(behaviour func(call int32) (int, map[string]any)) *httptest.Server {
	f := &fakeOIDC{tokenBehaviour: behaviour}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/client/register":
			f.registerCalls.Add(1)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"clientId":     "fake-client-id",
				"clientSecret": "fake-client-secret",
			})
		case "/device_authorization":
			f.deviceCalls.Add(1)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"deviceCode":              "fake-device-code",
				"userCode":                "AB12-CD34",
				"verificationUri":         "https://view.awsapps.com/start",
				"verificationUriComplete": "https://view.awsapps.com/start?user_code=AB12-CD34",
				"interval":                1,
				"expiresIn":               600,
			})
		case "/token":
			n := f.tokenCalls.Add(1)
			code, body := f.tokenBehaviour(n)
			w.WriteHeader(code)
			_ = json.NewEncoder(w).Encode(body)
		default:
			http.NotFound(w, r)
		}
	}))
}

func newClient(endpoint string) *Client {
	return &Client{
		HTTP:     &http.Client{Timeout: 2 * time.Second},
		Endpoint: endpoint,
		Region:   "us-east-1",
	}
}

func TestStart_ReturnsSessionWithUserCode(t *testing.T) {
	ts := newFakeOIDC(func(_ int32) (int, map[string]any) {
		return 200, map[string]any{"accessToken": "at", "refreshToken": "rt", "expiresIn": 3600}
	})
	defer ts.Close()
	c := newClient(ts.URL)

	sess, err := c.Start(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if sess.ClientID != "fake-client-id" || sess.ClientSecret != "fake-client-secret" {
		t.Errorf("client id/secret not captured: %+v", sess)
	}
	if sess.UserCode != "AB12-CD34" {
		t.Errorf("user code: got %q", sess.UserCode)
	}
	if sess.Interval != time.Second {
		t.Errorf("interval should be 1s, got %v", sess.Interval)
	}
	if sess.VerificationURI == "" {
		t.Error("verification uri missing")
	}
}

func TestPoll_HappyPathReturnsResult(t *testing.T) {
	ts := newFakeOIDC(func(_ int32) (int, map[string]any) {
		return 200, map[string]any{"accessToken": "at-123", "refreshToken": "rt-456", "expiresIn": 3600}
	})
	defer ts.Close()

	c := newClient(ts.URL)
	sess, err := c.Start(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	r, err := c.Poll(context.Background(), sess)
	if err != nil {
		t.Fatal(err)
	}
	if r.AccessToken != "at-123" || r.RefreshToken != "rt-456" {
		t.Fatalf("wrong tokens: %+v", r)
	}
}

func TestPoll_StateMachine_PendingThenSuccess(t *testing.T) {
	ts := newFakeOIDC(func(call int32) (int, map[string]any) {
		if call < 3 {
			return 400, map[string]any{"error": "authorization_pending"}
		}
		return 200, map[string]any{"accessToken": "final-at", "refreshToken": "final-rt", "expiresIn": 3600}
	})
	defer ts.Close()

	c := newClient(ts.URL)
	sess, err := c.Start(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	sess.Interval = 5 * time.Millisecond

	start := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	r, err := c.WaitForCompletion(ctx, sess, nil)
	if err != nil {
		t.Fatalf("wait: %v", err)
	}
	if r.AccessToken != "final-at" {
		t.Fatal("wrong access token")
	}
	elapsed := time.Since(start)
	if elapsed < 10*time.Millisecond {
		t.Errorf("polling looked instant; did we actually poll? elapsed=%v", elapsed)
	}
}

func TestPoll_SlowDownExtendsInterval(t *testing.T) {
	var callTimes []time.Time
	ts := newFakeOIDC(func(call int32) (int, map[string]any) {
		callTimes = append(callTimes, time.Now())
		if call == 1 {
			return 400, map[string]any{"error": "slow_down"}
		}
		return 200, map[string]any{"accessToken": "at", "refreshToken": "rt", "expiresIn": 3600}
	})
	defer ts.Close()

	c := newClient(ts.URL)
	sess, err := c.Start(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	sess.Interval = 1 * time.Millisecond

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, err = c.WaitForCompletion(ctx, sess, nil)
	if err != nil {
		t.Fatalf("wait: %v", err)
	}
	if len(callTimes) < 2 {
		t.Fatalf("want >=2 calls, got %d", len(callTimes))
	}
	gap := callTimes[1].Sub(callTimes[0])
	if gap < 5*time.Second {
		t.Errorf("slow_down should bump interval to >=5s, actual gap %v", gap)
	}
}

func TestPoll_AccessDeniedIsTerminal(t *testing.T) {
	ts := newFakeOIDC(func(_ int32) (int, map[string]any) {
		return 400, map[string]any{"error": "access_denied"}
	})
	defer ts.Close()

	c := newClient(ts.URL)
	sess, err := c.Start(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.Poll(context.Background(), sess)
	if !errors.Is(err, ErrAccessDenied) {
		t.Fatalf("want ErrAccessDenied, got %v", err)
	}
}

func TestPoll_ExpiredTokenIsTerminal(t *testing.T) {
	ts := newFakeOIDC(func(_ int32) (int, map[string]any) {
		return 400, map[string]any{"error": "expired_token"}
	})
	defer ts.Close()
	c := newClient(ts.URL)
	sess, err := c.Start(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.Poll(context.Background(), sess)
	if !errors.Is(err, ErrDeviceCodeExpired) {
		t.Fatalf("want ErrDeviceCodeExpired, got %v", err)
	}
}

func TestPoll_SessionExpiryRejectedLocally(t *testing.T) {
	ts := newFakeOIDC(func(_ int32) (int, map[string]any) {
		t.Fatal("server should not be called after local expiry")
		return 500, nil
	})
	defer ts.Close()

	c := newClient(ts.URL)
	sess, err := c.Start(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	sess.ExpiresAt = time.Now().Add(-1 * time.Hour)
	_, err = c.Poll(context.Background(), sess)
	if !errors.Is(err, ErrDeviceCodeExpired) {
		t.Fatalf("want ErrDeviceCodeExpired, got %v", err)
	}
}

func TestWaitForCompletion_ContextCancelReturns(t *testing.T) {
	ts := newFakeOIDC(func(_ int32) (int, map[string]any) {
		return 400, map[string]any{"error": "authorization_pending"}
	})
	defer ts.Close()
	c := newClient(ts.URL)
	sess, err := c.Start(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	sess.Interval = 50 * time.Millisecond

	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	_, err = c.WaitForCompletion(ctx, sess, nil)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("want ctx deadline, got %v", err)
	}
}
