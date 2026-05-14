// This file is derived from github.com/d-kuro/kirocc
// Original commit: 5633c47f0d65aaef748728bae1c68160b0ea538d
// Copyright (c) 2026 d-kuro. Licensed under Apache License, Version 2.0.
// Modifications (c) 2026 kiroxy contributors.

package kiroclient

import (
	"encoding/json/v2"
	"fmt"
	"mime"
	"net/http"
	"strings"
)

// UpstreamError is returned when the Kiro API responds with an HTTP error
// (any non-success status) or an unexpected Content-Type on a 200 response.
// Callers can use errors.As to extract structured fields for logging.
type UpstreamError struct {
	Status      int    // HTTP status code
	ContentType string // Content-Type header value
	Exception   string // AWS exception class (normalized, may be "")
	Reason      string // Sub-reason (Kiro-specific). e.g. "INSUFFICIENT_MODEL_CAPACITY"
	Body        string // response body (up to 8 KiB)
}

func (e *UpstreamError) Error() string {
	if e.Reason != "" {
		return fmt.Sprintf("kiro api: status=%d content_type=%q exception=%q reason=%q: %s",
			e.Status, e.ContentType, e.Exception, e.Reason, e.Body)
	}
	return fmt.Sprintf("kiro api: status=%d content_type=%q exception=%q: %s",
		e.Status, e.ContentType, e.Exception, e.Body)
}

// parseAWSExceptionType extracts the AWS exception type from an error body.
// AWS JSON 1.0 errors encode the exception class as "__type", optionally
// prefixed by a shape name ("com.amazonaws...#ThrottlingException").
// Returns "" if the body cannot be parsed.
func parseAWSExceptionType(body string) string {
	exType, _ := parseAWSExceptionFields(body)
	return exType
}

// parseAWSExceptionFields extracts both the AWS exception type AND a Kiro-specific
// `reason` field from the error body. Kiro decorates ThrottlingException with a
// `reason` field that distinguishes server-side capacity issues
// ("INSUFFICIENT_MODEL_CAPACITY") from real per-account rate limiting. Without this
// distinction, kiroxy treats both identically (cooldown the account for an hour),
// which causes mass cooldown stampedes when the upstream model fleet is overloaded.
//
// Example body:
//
//	{
//	  "__type": "com.amazon.kiro.runtimeservice#ThrottlingException",
//	  "message": "I am experiencing high traffic, please try again shortly.",
//	  "reason": "INSUFFICIENT_MODEL_CAPACITY"
//	}
//
// Returns ("", "") if the body cannot be parsed.
func parseAWSExceptionFields(body string) (exType, reason string) {
	if body == "" {
		return "", ""
	}
	var m struct {
		Type1  string `json:"__type"`
		Type2  string `json:"type"`
		Code   string `json:"code"`
		Reason string `json:"reason"`
	}
	if err := json.Unmarshal([]byte(body), &m); err != nil {
		return "", ""
	}
	t := m.Type1
	if t == "" {
		t = m.Type2
	}
	if t == "" {
		t = m.Code
	}
	return normalizeAWSExceptionType(t), m.Reason
}

// IsRetryableAWSException reports whether an AWS exception type is transient
// and worth retrying (modeled after the AWS SDK retry policy).
func IsRetryableAWSException(exType string) bool {
	switch exType {
	case "ThrottlingException",
		"TooManyRequestsException",
		"ServiceUnavailableException",
		"InternalServerException",
		"InternalFailureException",
		"InternalServerError":
		return true
	}
	return false
}

// normalizeAWSExceptionType strips namespace prefixes and hostname suffixes
// from an AWS exception type string. AWS uses two formats:
//   - JSON __type: "com.amazon.coral.service#ThrottlingException"
//   - Header X-Amzn-ErrorType: "ThrottlingException:http://example.com"
//
// This function handles both by stripping after '#' and before ':'.
func normalizeAWSExceptionType(raw string) string {
	if raw == "" {
		return ""
	}
	// Strip namespace prefix (e.g. "com.amazon.coral.service#ThrottlingException").
	if i := strings.LastIndexByte(raw, '#'); i >= 0 {
		raw = raw[i+1:]
	}
	// Strip hostname suffix (e.g. "ThrottlingException:http://example.com").
	if colon, _, ok := strings.Cut(raw, ":"); ok {
		raw = colon
	}
	return raw
}

// resolveAWSException determines the AWS exception type from the response,
// checking the body first, then falling back to the X-Amzn-ErrorType header.
func resolveAWSException(body string, header http.Header) string {
	if exType := parseAWSExceptionType(body); exType != "" {
		return exType
	}
	return normalizeAWSExceptionType(header.Get("X-Amzn-ErrorType"))
}

// resolveAWSExceptionFields is the same as resolveAWSException but also returns
// the Kiro-specific `reason` field from the response body. Header fallback only
// supplies the exception type — `reason` is body-only.
func resolveAWSExceptionFields(body string, header http.Header) (exType, reason string) {
	exType, reason = parseAWSExceptionFields(body)
	if exType == "" {
		exType = normalizeAWSExceptionType(header.Get("X-Amzn-ErrorType"))
	}
	return exType, reason
}

// isEventStreamContentType reports whether ct matches the AWS event stream
// content type (with or without parameters such as "; charset=...").
func isEventStreamContentType(ct string) bool {
	const want = "application/vnd.amazon.eventstream"
	mt, _, err := mime.ParseMediaType(ct)
	if err != nil {
		return false
	}
	return strings.EqualFold(mt, want)
}
