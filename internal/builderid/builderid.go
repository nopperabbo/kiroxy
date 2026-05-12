// Package builderid implements the AWS Builder ID Device Code OAuth flow for
// Kiro / CodeWhisperer access. Derived in structure from github.com/Quorinex/Kiro-Go
// @ 940dc782 (auth/builderid.go, MIT); rewritten in this file to decouple from
// Quorinex's session registry and to surface every error as a typed Go value
// rather than a multi-return string tuple.
//
// The three-step flow:
//  1. Register an OIDC client (POST /client/register) -> (clientID, clientSecret)
//  2. Request a device authorization  (POST /device_authorization)
//     -> (deviceCode, userCode, verificationURI, interval, expiresIn)
//  3. Poll POST /token with the deviceCode until the user approves in a browser.
//     On 200 we receive (accessToken, refreshToken).
//
// The flow never asks the user for a password: they type the userCode into the
// verification URI in a browser where they are already signed in.
package builderid

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Scopes are the CodeWhisperer OAuth scopes used by the Kiro desktop and CLI
// clients. Matches Quorinex upstream.
var Scopes = []string{
	"codewhisperer:completions",
	"codewhisperer:analysis",
	"codewhisperer:conversations",
	"codewhisperer:transformations",
	"codewhisperer:taskassist",
}

// Session is the in-flight device authorization. The caller holds it across
// the polling loop.
type Session struct {
	ClientID        string
	ClientSecret    string
	DeviceCode      string
	UserCode        string
	VerificationURI string
	Interval        time.Duration
	ExpiresAt       time.Time
	Region          string
	OIDCEndpoint    string
}

// Result is what Poll returns on success.
type Result struct {
	AccessToken  string
	RefreshToken string
	ExpiresIn    int
}

// errorCode is the string returned inside the JSON body on 4xx responses.
// Separated so tests can match without parsing the whole body.
type errorCode string

const (
	codeAuthPending errorCode = "authorization_pending"
	codeSlowDown    errorCode = "slow_down"
	codeExpired     errorCode = "expired_token"
	codeDenied      errorCode = "access_denied"
)

// Errors returned during the flow.
var (
	ErrAuthorizationPending = errors.New("builderid: authorization pending; keep polling")
	ErrSlowDown             = errors.New("builderid: slow down; increase polling interval")
	ErrDeviceCodeExpired    = errors.New("builderid: device code expired; restart the flow")
	ErrAccessDenied         = errors.New("builderid: user denied the authorization")
)

// Client talks to the AWS OIDC endpoint. Default is a real http.Client hitting
// https://oidc.{region}.amazonaws.com. Tests override Endpoint.
type Client struct {
	HTTP     *http.Client
	Endpoint string // if empty, derived from Region
	Region   string // default "us-east-1"
}

// NewClient returns a production-ready Client with a 30-second HTTP timeout.
func NewClient(region string) *Client {
	if region == "" {
		region = "us-east-1"
	}
	return &Client{
		HTTP:   &http.Client{Timeout: 30 * time.Second},
		Region: region,
	}
}

func (c *Client) endpoint() string {
	if c.Endpoint != "" {
		return c.Endpoint
	}
	return fmt.Sprintf("https://oidc.%s.amazonaws.com", c.Region)
}

// Start performs steps 1+2 of the device flow and returns a Session ready for
// polling. The caller is expected to display UserCode + VerificationURI to the
// user, then call Poll() every Session.Interval.
func (c *Client) Start(ctx context.Context) (*Session, error) {
	reg, err := c.register(ctx)
	if err != nil {
		return nil, fmt.Errorf("register client: %w", err)
	}
	dev, err := c.deviceAuthorize(ctx, reg.ClientID, reg.ClientSecret)
	if err != nil {
		return nil, fmt.Errorf("device authorize: %w", err)
	}

	interval := time.Duration(dev.Interval) * time.Second
	if interval <= 0 {
		interval = 5 * time.Second
	}
	expiresIn := dev.ExpiresIn
	if expiresIn <= 0 {
		expiresIn = 600
	}
	verificationURI := dev.VerificationURIComplete
	if verificationURI == "" {
		verificationURI = dev.VerificationURI
	}

	return &Session{
		ClientID:        reg.ClientID,
		ClientSecret:    reg.ClientSecret,
		DeviceCode:      dev.DeviceCode,
		UserCode:        dev.UserCode,
		VerificationURI: verificationURI,
		Interval:        interval,
		ExpiresAt:       time.Now().Add(time.Duration(expiresIn) * time.Second),
		Region:          c.Region,
		OIDCEndpoint:    c.endpoint(),
	}, nil
}

// Poll performs one /token exchange. Returns (*Result, nil) on success,
// (nil, ErrAuthorizationPending) or (nil, ErrSlowDown) when the user hasn't
// completed the flow, (nil, ErrDeviceCodeExpired|ErrAccessDenied) on fatal
// terminals, or (nil, err) on transport/parse failures.
func (c *Client) Poll(ctx context.Context, s *Session) (*Result, error) {
	if time.Now().After(s.ExpiresAt) {
		return nil, ErrDeviceCodeExpired
	}

	body, _ := json.Marshal(map[string]string{
		"clientId":     s.ClientID,
		"clientSecret": s.ClientSecret,
		"grantType":    "urn:ietf:params:oauth:grant-type:device_code",
		"deviceCode":   s.DeviceCode,
	})
	req, err := http.NewRequestWithContext(ctx, "POST", s.OIDCEndpoint+"/token", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("token request: %w", err)
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == http.StatusOK {
		var r struct {
			AccessToken  string `json:"accessToken"`
			RefreshToken string `json:"refreshToken"`
			ExpiresIn    int    `json:"expiresIn"`
		}
		if err := json.Unmarshal(raw, &r); err != nil {
			return nil, fmt.Errorf("parse token response: %w (body=%q)", err, raw)
		}
		return &Result{AccessToken: r.AccessToken, RefreshToken: r.RefreshToken, ExpiresIn: r.ExpiresIn}, nil
	}

	if resp.StatusCode == http.StatusBadRequest {
		var e struct {
			Error errorCode `json:"error"`
		}
		_ = json.Unmarshal(raw, &e)
		switch e.Error {
		case codeAuthPending:
			return nil, ErrAuthorizationPending
		case codeSlowDown:
			return nil, ErrSlowDown
		case codeExpired:
			return nil, ErrDeviceCodeExpired
		case codeDenied:
			return nil, ErrAccessDenied
		default:
			return nil, fmt.Errorf("builderid: authorization error %q (body=%q)", e.Error, raw)
		}
	}

	return nil, fmt.Errorf("builderid: unexpected %d response (body=%q)", resp.StatusCode, raw)
}

// WaitForCompletion drives the polling loop until Poll returns a Result, a
// fatal error, or ctx deadline. Interval grows when the server emits slow_down.
func (c *Client) WaitForCompletion(ctx context.Context, s *Session, onTick func()) (*Result, error) {
	interval := s.Interval
	for {
		if onTick != nil {
			onTick()
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(interval):
		}
		result, err := c.Poll(ctx, s)
		switch {
		case err == nil:
			return result, nil
		case errors.Is(err, ErrAuthorizationPending):
			// keep interval the same
		case errors.Is(err, ErrSlowDown):
			interval += 5 * time.Second
		default:
			return nil, err
		}
	}
}

// ---- internal wire types ----

type registerResp struct {
	ClientID     string `json:"clientId"`
	ClientSecret string `json:"clientSecret"`
}

type deviceAuthorizeResp struct {
	DeviceCode              string `json:"deviceCode"`
	UserCode                string `json:"userCode"`
	VerificationURI         string `json:"verificationUri"`
	VerificationURIComplete string `json:"verificationUriComplete"`
	Interval                int    `json:"interval"`
	ExpiresIn               int    `json:"expiresIn"`
}

func (c *Client) register(ctx context.Context) (*registerResp, error) {
	body, _ := json.Marshal(map[string]any{
		"clientName": "kiroxy",
		"clientType": "public",
		"scopes":     Scopes,
		"grantTypes": []string{"urn:ietf:params:oauth:grant-type:device_code", "refresh_token"},
		"issuerUrl":  "https://view.awsapps.com/start",
	})
	req, err := http.NewRequestWithContext(ctx, "POST", c.endpoint()+"/client/register", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d: %s", resp.StatusCode, raw)
	}
	var r registerResp
	if err := json.Unmarshal(raw, &r); err != nil {
		return nil, fmt.Errorf("parse: %w", err)
	}
	return &r, nil
}

func (c *Client) deviceAuthorize(ctx context.Context, clientID, clientSecret string) (*deviceAuthorizeResp, error) {
	body, _ := json.Marshal(map[string]string{
		"clientId":     clientID,
		"clientSecret": clientSecret,
		"startUrl":     "https://view.awsapps.com/start",
	})
	req, err := http.NewRequestWithContext(ctx, "POST", c.endpoint()+"/device_authorization", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d: %s", resp.StatusCode, raw)
	}
	var r deviceAuthorizeResp
	if err := json.Unmarshal(raw, &r); err != nil {
		return nil, fmt.Errorf("parse: %w", err)
	}
	return &r, nil
}
