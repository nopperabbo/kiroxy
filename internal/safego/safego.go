// Package safego provides panic-safe goroutine spawn helpers. Every
// long-lived worker goroutine in the proxy MUST be started via this
// package so that an unexpected panic is logged with full stack trace
// (via slog at ERROR level) rather than silently terminating the
// goroutine and leaving the system in a degraded state with no
// visible signal.
//
// The standard panic-recovery middleware in internal/server protects
// HTTP handler goroutines (one goroutine per request, spawned by
// net/http). It does NOT protect goroutines spawned at startup
// (usage poller, stickiness pruner, SSE keepalive emitter, OpenAI
// translator, idle reader I/O). A panic in any of these silently
// kills the worker without crashing the process — by design, because
// net/http per-conn recover catches them, but no log line, no metric,
// no diagnosis. safego closes that gap.
//
// The package intentionally has zero dependencies on metrics, config,
// or anything else: when something panics, the recovery path itself
// must not panic. Callers wanting metric instrumentation can pass a
// counter callback via SetOnPanic at startup.
package safego

import (
	"log/slog"
	"runtime/debug"
	"sync/atomic"
)

// onPanicFn, if non-nil, is invoked from recoverPanic with the worker
// name AFTER the slog event is emitted. Set once at startup via
// SetOnPanic. Read concurrent-safe via atomic.Pointer.
var onPanicFn atomic.Pointer[func(name string)]

// SetOnPanic installs a global hook called whenever a goroutine
// guarded by Go or Run recovers from a panic. Typically used to
// increment a Prometheus counter. Safe to call multiple times
// (idempotent: last call wins). Pass nil to disable.
func SetOnPanic(fn func(name string)) {
	if fn == nil {
		onPanicFn.Store(nil)
		return
	}
	onPanicFn.Store(&fn)
}

// Go spawns fn in a new goroutine guarded by deferred recover().
// On panic: logs ERROR with worker name, recovered value, and full
// stack trace, then returns to allow the goroutine to exit cleanly.
//
// name should be a short stable identifier (e.g. "sse-keepalive",
// "usage-poller") that makes panic origin trivially attributable in
// logs and dashboards. NEVER include per-request data in name —
// that would explode metric label cardinality.
func Go(name string, fn func()) {
	go func() {
		defer recoverPanic(name)
		fn()
	}()
}

// Run executes fn synchronously in the calling goroutine guarded by
// deferred recover(). Useful when caller needs to ensure post-fn
// cleanup happens (channel close, defer chain) even if fn panics —
// e.g. the OpenAI translator goroutine where the parent blocks on
// `<-done` and a silent panic deadlocks forever.
//
// Caller is responsible for any synchronization (channels, etc).
// Run converts panics to logs, NOT to errors — callers must not
// rely on its return for error signaling.
func Run(name string, fn func()) {
	defer recoverPanic(name)
	fn()
}

// recoverPanic is the shared recover implementation. Extracted so
// both Go and Run share identical behavior (and so adding fields to
// the slog event happens in exactly one place).
func recoverPanic(name string) {
	r := recover()
	if r == nil {
		return
	}
	slog.Error("safego: goroutine panic recovered",
		slog.String("worker", name),
		slog.Any("recovered", r),
		slog.String("stack", string(debug.Stack())),
	)
	if hook := onPanicFn.Load(); hook != nil {
		// Hook must never panic — a buggy metrics call would
		// otherwise turn a recovery into a process kill via
		// the unprotected stack frame above us. Guard
		// explicitly: silently swallow any hook panic.
		func() {
			defer func() { _ = recover() }()
			(*hook)(name)
		}()
	}
}
