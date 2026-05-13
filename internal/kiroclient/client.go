// This file is derived from github.com/d-kuro/kirocc
// Original commit: 5633c47f0d65aaef748728bae1c68160b0ea538d
// Copyright (c) 2026 d-kuro. Licensed under Apache License, Version 2.0.
// Modifications (c) 2026 kiroxy contributors.

package kiroclient

import (
	"bytes"
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/google/uuid"
	"local/kiroxy/internal/kiroproto"
	"local/kiroxy/internal/logging"
	"local/kiroxy/internal/tracing"
)

const (
	// amzTargetCodeWhisperer is the X-Amz-Target the Kiro desktop/CLI clients use
	// when authenticated via social (kiro-cli) auth that provides a profileArn.
	amzTargetCodeWhisperer = "AmazonCodeWhispererStreamingService.GenerateAssistantResponse"

	// amzTargetAmazonQ is the X-Amz-Target that works for Builder ID accounts
	// which have no profileArn (the CodeWhisperer target rejects them with
	// "profileArn is required for this request"). Quorinex/Kiro-Go proves this
	// fallback works; kiroxy auto-switches to it when payload.ProfileARN == "".
	amzTargetAmazonQ = "AmazonQDeveloperStreamingService.SendMessage"

	maxRetries     = 3
	baseRetryDelay = 1 * time.Second

	// KiroIDE User-Agent (aws-sdk-js shape). Matches Quorinex/Kiro-Go's proven
	// header format. The kirocc-native Rust SDK UA is appropriate for kiro-cli
	// accounts but breaks Builder ID accounts at the gateway ("credential is
	// invalid"). Kiro IDE UA accepts both credential sources.
	kiroIDEVersion      = "0.11.107"
	kiroStreamingSDKVer = "1.0.34"
	userAgentValue      = "aws-sdk-js/1.0.34 ua/2.1 os/darwin#24.6.0 lang/js md/nodejs#22.22.0 api/codewhispererstreaming#1.0.34 m/E KiroIDE-0.11.107"
	amzUserAgentValue   = "aws-sdk-js/1.0.34 KiroIDE-0.11.107"
)

// Client is the interface for calling the Kiro API.
type Client interface {
	GenerateAssistantResponse(ctx context.Context, token string, payload *kiroproto.Payload, region string) (*Response, error)
}

// Response wraps the HTTP response from the Kiro API.
type Response struct {
	StatusCode   int
	Body         io.ReadCloser
	Header       http.Header
	PromptTokens int // pre-counted from serialized payload via tiktoken
}

// TokenRefresher is called when a 403 is received to get a fresh token.
type TokenRefresher func(ctx context.Context) (newToken string, err error)

// ErrBodyReadIdle is returned when the Kiro response body has not produced
// any data within the configured idle timeout. This guards against silent
// hangs where the server sends eventstream headers but never delivers frames.
var ErrBodyReadIdle = errors.New("kiroclient: body read idle timeout")

const defaultBodyReadIdleTimeout = 180 * time.Second

// HTTPClient is the production implementation of Client.
type HTTPClient struct {
	httpClient     *http.Client
	baseURL        string // override for tests; empty = use region-based URL
	otel           bool
	otelBodyLimit  int
	tokenRefresher TokenRefresher
	countTokens    func([]byte) (int, error) // nil = skip token counting
	bodyReadIdle   time.Duration             // idle timeout for response body reads; 0 = use default
}

// HTTPClientOption configures an HTTPClient.
type HTTPClientOption func(*HTTPClient)

// WithBaseURL sets a custom base URL (for testing).
func WithBaseURL(url string) HTTPClientOption {
	return func(c *HTTPClient) { c.baseURL = url }
}

// WithTokenRefresher sets the token refresh callback for 403 retry.
func WithTokenRefresher(fn TokenRefresher) HTTPClientOption {
	return func(c *HTTPClient) { c.tokenRefresher = fn }
}

// WithTokenCounter sets a function to count prompt tokens from the serialized payload.
func WithTokenCounter(fn func([]byte) (int, error)) HTTPClientOption {
	return func(c *HTTPClient) { c.countTokens = fn }
}

// WithBodyReadIdleTimeout sets the idle read deadline applied to the
// response body of a successful 200 eventstream response. If no byte is
// read within the given duration, the body Read returns ErrBodyReadIdle.
//
// NOTE: The idle reader calls Close() to unblock a pending Read. This is
// guaranteed to work for net/http.Response.Body but is NOT a general
// guarantee for arbitrary io.ReadCloser implementations.
func WithBodyReadIdleTimeout(d time.Duration) HTTPClientOption {
	return func(c *HTTPClient) { c.bodyReadIdle = d }
}

// WithOTel enables OpenTelemetry tracing on outgoing requests.
func WithOTel(bodyLimit int) HTTPClientOption {
	return func(c *HTTPClient) {
		c.otel = true
		c.otelBodyLimit = bodyLimit
	}
}

// NewHTTPClient creates a new HTTPClient.
func NewHTTPClient(opts ...HTTPClientOption) *HTTPClient {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.MaxIdleConns = 100
	transport.MaxIdleConnsPerHost = 10
	transport.IdleConnTimeout = 90 * time.Second
	transport.ResponseHeaderTimeout = 30 * time.Second

	c := &HTTPClient{}
	for _, opt := range opts {
		opt(c)
	}

	var rt http.RoundTripper = transport
	if c.otel {
		rt = tracing.WrapTransport(transport, c.otelBodyLimit)
	}
	c.httpClient = &http.Client{Transport: rt}
	return c
}

func (c *HTTPClient) bodyReadIdleTimeout() time.Duration {
	if c.bodyReadIdle > 0 {
		return c.bodyReadIdle
	}
	return defaultBodyReadIdleTimeout
}

// idleReader moved to idle_reader.go.

func (c *HTTPClient) recordError(ctx context.Context, err error) {
	if c.otel {
		tracing.RecordError(ctx, err)
	}
}

// endpointURL returns the Kiro API URL for a region.
//
// AWS deprecates legacy `q.<region>.amazonaws.com` on 2026-05-15 in favour
// of `runtime.<region>.kiro.dev`. Defaults to new endpoint; legacy is
// available via KIROXY_USE_LEGACY_ENDPOINT=1 until 2026-08-15.
//
// Reference: jwadow/kiro-gateway#146 (deadline), PR #155 (migration).
func (c *HTTPClient) endpointURL(region string) string {
	if c.baseURL != "" {
		return c.baseURL
	}
	if os.Getenv("KIROXY_USE_LEGACY_ENDPOINT") == "1" {
		return fmt.Sprintf("https://q.%s.amazonaws.com/", region)
	}
	return fmt.Sprintf("https://runtime.%s.kiro.dev/", region)
}

// GenerateAssistantResponse sends a request to the Kiro API with retry logic.
func (c *HTTPClient) GenerateAssistantResponse(ctx context.Context, token string, payload *kiroproto.Payload, region string) (*Response, error) {
	endpoint := c.endpointURL(region)

	if c.otel {
		var span trace.Span
		ctx, span = tracing.Tracer().Start(ctx, "kiro.GenerateAssistantResponse")
		defer span.End()
		span.SetAttributes(
			attribute.String("kiro.region", region),
			attribute.String("kiro.endpoint", endpoint),
		)
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal payload: %w", err)
	}

	var promptTokens int
	if c.countTokens != nil {
		n, err := c.countTokens(body)
		if err != nil {
			slog.Debug("tokencount: failed to count prompt tokens", "err", err)
		} else {
			promptTokens = n
		}
	}

	currentToken := token
	invocationID := uuid.New().String()
	traceID, short := logging.TraceIDs(ctx)

	for attempt := 0; attempt <= maxRetries; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
		if err != nil {
			return nil, fmt.Errorf("create request: %w", err)
		}

		req.Header.Set("Authorization", "Bearer "+currentToken)
		req.Header.Set("Content-Type", "application/x-amz-json-1.0")
		req.Header.Set("Accept", "*/*")
		req.Header.Set("X-Amz-Target", chooseAmzTarget(payload))
		req.Header.Set("User-Agent", userAgentValue)
		req.Header.Set("x-amz-user-agent", amzUserAgentValue)
		req.Header.Set("x-amzn-codewhisperer-optout", "false")
		req.Header.Set("amz-sdk-invocation-id", invocationID)
		req.Header.Set("amz-sdk-request", fmt.Sprintf("attempt=%d; max=%d", attempt+1, maxRetries+1))

		slog.DebugContext(ctx, "kiro request headers",
			"trace_id", traceID,
			"session_id", logging.SessionIDFromContext(ctx),
			"headers", logging.SafeHeaders{H: req.Header},
		)

		// TEMP Phase Track-1 BUG-1 outbound tap. Reverted in commit c3.
		if os.Getenv("KIROXY_TAP") == "1" {
			fmt.Fprintln(os.Stderr, "=== TAP: outbound kiro request ===")
			fmt.Fprintf(os.Stderr, "  URL: %s %s\n", req.Method, req.URL.String())
			for k, vs := range req.Header {
				v := ""
				if len(vs) > 0 {
					v = vs[0]
				}
				if k == "Authorization" && len(v) > 12 {
					v = v[:12] + "...(redacted)"
				}
				fmt.Fprintf(os.Stderr, "  %s: %s\n", k, v)
			}
			fmt.Fprintf(os.Stderr, "  Body (%d bytes):\n%s\n", len(body), string(body))
		}
		resp, err := c.httpClient.Do(req)
		if err != nil {
			if attempt < maxRetries {
				delay := backoffDelay(attempt)
				slog.WarnContext(ctx, "kiro: request error, retrying",
					"trace_id", short, "attempt", attempt+1, "max", maxRetries+1,
					"delay", delay, "err", err)
				if waitErr := retryWait(ctx, delay); waitErr != nil {
					return nil, waitErr
				}
				continue
			}
			c.recordError(ctx, err)
			return nil, fmt.Errorf("do request: %w", err)
		}

		switch {
		case resp.StatusCode == http.StatusOK:
			slog.DebugContext(ctx, "kiro response headers",
				"trace_id", traceID,
				"session_id", logging.SessionIDFromContext(ctx),
				"status", resp.StatusCode,
				"headers", logging.SafeHeaders{H: resp.Header},
			)
			// Kiro sometimes returns 200 with Content-Type application/json
			// (AWS exception envelope such as ThrottlingException or
			// InternalServerException) instead of the expected
			// application/vnd.amazon.eventstream. Detect and surface that
			// explicitly — otherwise the eventstream parser reads a
			// non-framed body and eventually errors with a confusing
			// "reading prelude" message, masking the real upstream error.
			if ct := resp.Header.Get("Content-Type"); !isEventStreamContentType(ct) {
				errBody := readLimitedBody(resp.Body, upstreamBodyLimit)
				exType := resolveAWSException(errBody, resp.Header)
				// Retry transient AWS exceptions (throttling / internal / 5xx-equivalent)
				// even though the HTTP status is 200.
				if attempt < maxRetries && isRetryableAWSException(exType) {
					delay := backoffDelay(attempt)
					slog.WarnContext(ctx, "kiro: 200 with non-eventstream exception, retrying",
						"trace_id", short, "content_type", ct, "exception", exType,
						"attempt", attempt+1, "max", maxRetries+1,
						"delay", delay, "body", errBody)
					if waitErr := retryWait(ctx, delay); waitErr != nil {
						return nil, waitErr
					}
					continue
				}
				ue := &UpstreamError{
					Status:      resp.StatusCode,
					ContentType: ct,
					Exception:   exType,
					Body:        errBody,
				}
				c.recordError(ctx, ue)
				return nil, ue
			}
			body := io.ReadCloser(&idleReader{rc: resp.Body, idle: c.bodyReadIdleTimeout()})
			return &Response{
				StatusCode:   resp.StatusCode,
				Body:         body,
				Header:       resp.Header,
				PromptTokens: promptTokens,
			}, nil

		case resp.StatusCode == http.StatusForbidden:
			// TEMP Phase Track-1 BUG-1: capture 403 body before close. Reverted in c3.
			if os.Getenv("KIROXY_TAP") == "1" {
				tapBody := readLimitedBody(resp.Body, 4096)
				fmt.Fprintf(os.Stderr, "=== TAP: upstream 403 response ===\n")
				for k, vs := range resp.Header {
					v := ""
					if len(vs) > 0 {
						v = vs[0]
					}
					fmt.Fprintf(os.Stderr, "  %s: %s\n", k, v)
				}
				fmt.Fprintf(os.Stderr, "  Body (%d bytes): %q\n", len(tapBody), string(tapBody))
			}
			_ = resp.Body.Close()
			if attempt < maxRetries && c.tokenRefresher != nil {
				newToken, err := c.tokenRefresher(ctx)
				if err != nil {
					slog.WarnContext(ctx, "kiro: token refresh failed",
						"trace_id", short, "err", err)
				} else {
					currentToken = newToken
					slog.InfoContext(ctx, "kiro: 403 received, token refreshed",
						"trace_id", short, "attempt", attempt+1, "max", maxRetries+1)
					continue
				}
			}
			ue := &UpstreamError{Status: resp.StatusCode, ContentType: resp.Header.Get("Content-Type")}
			c.recordError(ctx, ue)
			return nil, ue

		case resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500:
			errBody := readLimitedBody(resp.Body, upstreamBodyLimit)
			if attempt < maxRetries {
				delay := backoffDelay(attempt)
				slog.WarnContext(ctx, "kiro: upstream error, retrying",
					"trace_id", short, "status", resp.StatusCode,
					"attempt", attempt+1, "max", maxRetries+1,
					"delay", delay, "body", errBody)
				if waitErr := retryWait(ctx, delay); waitErr != nil {
					return nil, waitErr
				}
				continue
			}
			ue := &UpstreamError{
				Status:      resp.StatusCode,
				ContentType: resp.Header.Get("Content-Type"),
				Exception:   resolveAWSException(errBody, resp.Header),
				Body:        errBody,
			}
			c.recordError(ctx, ue)
			return nil, ue

		default:
			errBody := readLimitedBody(resp.Body, upstreamBodyLimit)
			ue := &UpstreamError{
				Status:      resp.StatusCode,
				ContentType: resp.Header.Get("Content-Type"),
				Exception:   resolveAWSException(errBody, resp.Header),
				Body:        errBody,
			}
			c.recordError(ctx, ue)
			return nil, ue
		}
	}

	err = fmt.Errorf("kiro api: max retries exceeded")
	c.recordError(ctx, err)
	return nil, err
}
