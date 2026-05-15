// kiroxy addition (not derived from upstream).
//
// This file isolates the request-shape selection that distinguishes the
// "native" Kiro IDE upstream call from the "legacy" CodeWhisperer-runtime
// shape kiroxy historically used. Both shapes work today; native produces
// better behavior because the upstream gateway treats requests bearing the
// native fingerprint (q.us-east-1.amazonaws.com endpoint, application/json
// content-type, x-amzn-kiro-agent-mode: vibe, machine_id-suffixed UA) as
// genuine Kiro IDE traffic instead of generic CodeWhisperer SDK traffic.
//
// Shape verified by direct comparison with Quorinex/Kiro-Go (MIT, Go, 611
// stars, github.com/Quorinex/Kiro-Go) plus live curl probes against an
// active pool token in this repository's session log:
//
//	HTTP/2 200 + eventstream "Hi there! How can I help?"
//
// Toggle via KIROXY_NATIVE_HEADERS env:
//   - "" or "1" (default): native shape — primary endpoint
//   - "0": legacy shape — preserves old runtime.kiro.dev behavior for
//     debugging or when upstream rolls back the q.<region> path.
//
// Reference shapes are documented inline at endpointURL() and in the
// header-builder helper to anchor anti-regression: future contributors
// must NOT silently drop x-amzn-kiro-agent-mode or flip codewhisperer-optout
// back to "false" thinking they're harmless cosmetic adjustments — they
// genuinely change upstream's quality / quota / fingerprint disposition.

package kiroclient

import (
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/nopperabbo/kiroxy/internal/kiroproto"
)

// nativeHeadersEnabled returns false ONLY when KIROXY_NATIVE_HEADERS=0 is set.
// Default (unset or any other value including "1"/"true") means native ON.
// This makes the native path opt-out rather than opt-in so freshly-deployed
// kiroxy installs benefit from the better upstream behavior automatically.
func nativeHeadersEnabled() bool {
	v := os.Getenv("KIROXY_NATIVE_HEADERS")
	return v != "0"
}

// nativeEndpointURL returns the native Kiro IDE upstream URL — the path that
// genuine Kiro IDE talks to (q.<region>.amazonaws.com/generateAssistantResponse).
// This URL accepts modern model names (claude-sonnet-4.6, claude-opus-4.7,
// claude-haiku-4.5) and rejects the legacy CLAUDE_3_7_SONNET_V1_0 schema —
// kiroxy already emits modern names so the swap is drop-in.
func nativeEndpointURL(region string) string {
	return fmt.Sprintf("https://q.%s.amazonaws.com/generateAssistantResponse", region)
}

// legacyEndpointURL returns the runtime.<region>.kiro.dev URL kiroxy used
// before the native-shape rebase. Kept for opt-out via
// KIROXY_NATIVE_HEADERS=0. URL form is root path with no operation suffix
// because the legacy path uses X-Amz-Target to dispatch.
func legacyEndpointURL(region string) string {
	return fmt.Sprintf("https://runtime.%s.kiro.dev/", region)
}

// applyNativeHeaders sets the request header set that matches what native
// Kiro IDE sends — verified against Quorinex/Kiro-Go source plus live
// upstream probe in the session log. Caller must NOT also set X-Amz-Target
// when invoking this helper (the native primary path omits that header).
//
// machineID is appended to both User-Agent and x-amz-user-agent so the
// per-account fingerprint propagates upstream. Empty machineID degrades
// gracefully — UA stays at the bare "KiroIDE-<ver>" form, matching
// pre-machine-id kiroxy installs.
//
// invocationID/attempt/maxAttempts mirror the AWS SDK Pascal-case header
// names ("Amz-Sdk-Invocation-Id", "Amz-Sdk-Request") that native traffic
// uses; legacy lowercase variants exist only for backwards compatibility
// with the old kiroxy code path.
func applyNativeHeaders(req *http.Request, token, machineID, invocationID string, attempt, maxAttempts int) {
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "*/*")
	req.Header.Set("User-Agent", nativeUserAgent(machineID))
	req.Header.Set("x-amz-user-agent", nativeAmzUserAgent(machineID))
	req.Header.Set("x-amzn-codewhisperer-optout", "true")
	req.Header.Set("x-amzn-kiro-agent-mode", "vibe")
	req.Header.Set("Amz-Sdk-Invocation-Id", invocationID)
	req.Header.Set("Amz-Sdk-Request", fmt.Sprintf("attempt=%d; max=%d", attempt+1, maxAttempts+1))
}

// applyLegacyHeaders sets the pre-native-shape header block kiroxy used
// historically (lowercase amz-sdk-*, X-Amz-Target, codewhisperer-optout=false,
// no kiro-agent-mode, no machineID). Used when KIROXY_NATIVE_HEADERS=0.
func applyLegacyHeaders(req *http.Request, token, invocationID string, attempt, maxAttempts int, payload *kiroproto.Payload) {
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/x-amz-json-1.0")
	req.Header.Set("Accept", "*/*")
	req.Header.Set("X-Amz-Target", chooseAmzTarget(payload))
	req.Header.Set("User-Agent", userAgentValue)
	req.Header.Set("x-amz-user-agent", amzUserAgentValue)
	req.Header.Set("x-amzn-codewhisperer-optout", "false")
	req.Header.Set("amz-sdk-invocation-id", invocationID)
	req.Header.Set("amz-sdk-request", fmt.Sprintf("attempt=%d; max=%d", attempt+1, maxAttempts+1))
}

// nativeUserAgent returns the streaming-endpoint UA with machine_id appended.
// Format mirrors Quorinex's `aws-sdk-js/<sdk> ua/2.1 os/<os>#<ver> lang/js
// md/nodejs#<ver> api/codewhispererstreaming#<sdk> m/E KiroIDE-<ver>-<mid>`.
// Empty machineID drops the trailing dash + suffix so the UA stays valid.
func nativeUserAgent(machineID string) string {
	base := userAgentValue
	if machineID == "" {
		return base
	}
	return base + "-" + machineID
}

// nativeAmzUserAgent mirrors nativeUserAgent for the abbreviated x-amz-
// user-agent header. Same machine_id suffix logic.
func nativeAmzUserAgent(machineID string) string {
	base := amzUserAgentValue
	if machineID == "" {
		return base
	}
	return base + "-" + machineID
}

// trimMachineID enforces the upstream-acceptable shape for the machine_id
// component of the UA suffix: lowercase hex/uuid/digits, max 64 chars, no
// whitespace. Defensive — vault metadata is operator-controlled but a
// hand-edited bad value should NOT corrupt outbound headers.
func trimMachineID(raw string) string {
	raw = strings.TrimSpace(raw)
	if len(raw) > 64 {
		raw = raw[:64]
	}
	return raw
}
