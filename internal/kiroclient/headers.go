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

// kiroEndpoint represents one of the three accepted upstream paths for the
// streaming /generateAssistantResponse operation. Each path is fronted by a
// different Amazon service (or absent target — Kiro IDE's "native" primary
// flow), but all three accept the same JSON request body and return the same
// AWS EventStream response shape.
//
// The shape was reverse-engineered by Quorinex/Kiro-Go (MIT, 611★) and
// reproduced here for fail-over symmetry: when the primary returns HTTP 429
// (per-account quota exhausted at THIS gateway), kiroxy walks down the list
// and re-issues the same request on the next service. Each service tracks
// quota independently — the same account can usually answer through
// CodeWhisperer or AmazonQ even when q.<region>.amazonaws.com (Kiro IDE)
// reports exhaustion.
//
// Field semantics:
//   - URL: the full operation URL. amazonaws.com (slot 0+2) and
//     codewhisperer.us-east-1.amazonaws.com (slot 1) for us-east-1; for
//     other regions kiroxy substitutes the region into the host segment but
//     codewhisperer.<region>.amazonaws.com may not exist outside us-east-1.
//   - Origin: the `ConversationState.CurrentMessage.UserInputMessage.Origin`
//     payload field that upstream uses to attribute the request to a client
//     class. All three slots use AI_EDITOR — Kiro IDE marks itself as such,
//     and the fallback gateways accept the same value because they assume
//     they're proxying for an editor.
//   - AmzTarget: the X-Amz-Target header. Empty for the Kiro IDE primary
//     because native Kiro IDE does NOT send this header (the URL path itself
//     identifies the operation). Non-empty for the fallback services
//     because they're standard AWS gateways requiring the header.
//   - Name: human-readable label for logs / metrics.
type kiroEndpoint struct {
	URL       string
	Origin    string
	AmzTarget string
	Name      string
}

// kiroEndpoints lists the three accepted upstream paths in fail-over order.
// Order proven optimal by Quorinex/Kiro-Go's production telemetry — Kiro IDE
// primary has the highest success rate; CodeWhisperer fallback handles ~80%
// of primary 429s; AmazonQ catches the remainder.
//
// The Kiro IDE slot is duplicated host-wise with AmazonQ because both
// nominally point at q.<region>.amazonaws.com — they're different services
// behind the same hostname, dispatched via X-Amz-Target presence/value.
var kiroEndpoints = []kiroEndpoint{
	{
		// Kiro IDE primary: omits X-Amz-Target so the gateway routes by URL path.
		URL:       "https://q.%s.amazonaws.com/generateAssistantResponse",
		Origin:    "AI_EDITOR",
		AmzTarget: "",
		Name:      "Kiro IDE",
	},
	{
		// CodeWhisperer fallback: dedicated host + classic streaming target.
		URL:       "https://codewhisperer.%s.amazonaws.com/generateAssistantResponse",
		Origin:    "AI_EDITOR",
		AmzTarget: "AmazonCodeWhispererStreamingService.GenerateAssistantResponse",
		Name:      "CodeWhisperer",
	},
	{
		// AmazonQ Developer fallback: same q.<region> host, different target.
		URL:       "https://q.%s.amazonaws.com/generateAssistantResponse",
		Origin:    "AI_EDITOR",
		AmzTarget: "AmazonQDeveloperStreamingService.SendMessage",
		Name:      "AmazonQ",
	},
}

// resolveKiroEndpoints returns the per-region URL list of native endpoints.
// Each kiroEndpoint.URL is a printf format string with one %s for the region;
// this helper substitutes and returns concrete kiroEndpoint values.
func resolveKiroEndpoints(region string) []kiroEndpoint {
	out := make([]kiroEndpoint, len(kiroEndpoints))
	for i, ep := range kiroEndpoints {
		out[i] = kiroEndpoint{
			URL:       fmt.Sprintf(ep.URL, region),
			Origin:    ep.Origin,
			AmzTarget: ep.AmzTarget,
			Name:      ep.Name,
		}
	}
	return out
}

// preferredEndpointName reads KIROXY_PREFERRED_ENDPOINT and returns the slot
// label the operator wants tried FIRST. Recognized values (case-insensitive):
//   - "kiro" / "kiroide" / "" / unset → slot 0 (Kiro IDE primary)
//   - "codewhisperer" / "cw"          → slot 1 (CodeWhisperer fallback)
//   - "amazonq" / "q"                  → slot 2 (AmazonQ fallback)
//
// Anything else falls through to the default. Operators usually leave this
// unset; the override exists for emergency upstream-incident routing without
// a redeploy.
func preferredEndpointName() string {
	v := strings.ToLower(strings.TrimSpace(os.Getenv("KIROXY_PREFERRED_ENDPOINT")))
	switch v {
	case "codewhisperer", "cw":
		return "codewhisperer"
	case "amazonq", "q":
		return "amazonq"
	default:
		return "kiro"
	}
}

// failoverEnabled reports whether endpoint fail-over should engage on 429.
// Default: true. Operators disable via KIROXY_ENDPOINT_FAILOVER=0 if they
// want predictable single-endpoint behavior (e.g., for traffic-shape
// experiments or when one fallback is misbehaving).
func failoverEnabled() bool {
	v := os.Getenv("KIROXY_ENDPOINT_FAILOVER")
	return v != "0"
}

// orderedKiroEndpoints returns the per-region native endpoint list reordered
// so the operator's preferred slot is first. When fail-over is disabled the
// returned slice has length 1 (only the preferred endpoint, no fallbacks).
//
// Upstream-incident playbook: if a specific service slot is throwing 5xx
// across the entire pool, set KIROXY_PREFERRED_ENDPOINT=codewhisperer (or
// amazonq) to bypass the broken slot without losing fail-over to the
// remaining two. Setting KIROXY_ENDPOINT_FAILOVER=0 plus the preferred name
// pins traffic to one slot for incident investigation.
func orderedKiroEndpoints(region string) []kiroEndpoint {
	all := resolveKiroEndpoints(region)
	preferred := preferredEndpointName()

	var primary int
	switch preferred {
	case "codewhisperer":
		primary = 1
	case "amazonq":
		primary = 2
	default:
		primary = 0
	}

	if !failoverEnabled() {
		return []kiroEndpoint{all[primary]}
	}

	if primary == 0 {
		return all
	}

	// Preferred non-default + fail-over enabled: preferred first, then the
	// remaining two in original order. Symmetric with Quorinex's algorithm.
	out := make([]kiroEndpoint, 0, len(all))
	out = append(out, all[primary])
	for i, ep := range all {
		if i != primary {
			out = append(out, ep)
		}
	}
	return out
}

// applyNativeHeaders sets the request header set that matches what native
// Kiro IDE sends — verified against Quorinex/Kiro-Go source plus live
// upstream probe in the session log.
//
// amzTarget controls the X-Amz-Target header: empty string means the caller
// is targeting the Kiro IDE primary endpoint which omits this header (URL
// path identifies the operation). Non-empty values are set verbatim for
// fallback endpoints (CodeWhisperer / AmazonQ) which require the header.
//
// machineID is appended to both User-Agent and x-amz-user-agent so the
// per-account fingerprint propagates upstream. Empty machineID degrades
// gracefully — UA stays at the bare "KiroIDE-<ver>" form, matching
// pre-machine-id kiroxy installs.
//
// invocationID/attempt/maxAttempts mirror the AWS SDK Pascal-case header
// names ("Amz-Sdk-Invocation-Id", "Amz-Sdk-Request") that native traffic
// uses; legacy lowercase variants exist only for backwards compatibility
// with the old kiroxy code path. The header value max=3 mirrors the native
// SDK retry budget exposed to upstream — it's a hint, not a binding limit;
// kiroxy's actual per-endpoint retry count is governed by maxRetries.
func applyNativeHeaders(req *http.Request, token, machineID, amzTarget, invocationID string, attempt, maxAttempts int) {
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "*/*")
	req.Header.Set("User-Agent", nativeUserAgent(machineID))
	req.Header.Set("x-amz-user-agent", nativeAmzUserAgent(machineID))
	req.Header.Set("x-amzn-codewhisperer-optout", "true")
	req.Header.Set("x-amzn-kiro-agent-mode", "vibe")
	req.Header.Set("Amz-Sdk-Invocation-Id", invocationID)
	req.Header.Set("Amz-Sdk-Request", fmt.Sprintf("attempt=%d; max=%d", attempt+1, maxAttempts+1))
	if amzTarget != "" {
		req.Header.Set("X-Amz-Target", amzTarget)
	}
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
