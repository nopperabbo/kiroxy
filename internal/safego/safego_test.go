package safego

import (
	"sync"
	"sync/atomic"
	"testing"
)

func TestGo_RecoversAndContinues(t *testing.T) {
	var hookCalled atomic.Int32
	var capturedName string
	var mu sync.Mutex

	hookDone := make(chan struct{})
	SetOnPanic(func(name string) {
		hookCalled.Add(1)
		mu.Lock()
		capturedName = name
		mu.Unlock()
		close(hookDone)
	})
	t.Cleanup(func() { SetOnPanic(nil) })

	Go("test-worker", func() {
		panic("intentional test panic")
	})
	<-hookDone

	if hookCalled.Load() != 1 {
		t.Fatalf("expected hook called once, got %d", hookCalled.Load())
	}
	mu.Lock()
	got := capturedName
	mu.Unlock()
	if got != "test-worker" {
		t.Fatalf("expected name=test-worker, got %q", got)
	}
}

func TestRun_RecoversAndAllowsCleanup(t *testing.T) {
	cleanupRan := false
	Run("test-run", func() {
		defer func() { cleanupRan = true }()
		panic("intentional test panic")
	})

	if !cleanupRan {
		t.Fatal("deferred cleanup did not run after panic")
	}
}

func TestGo_NoPanic_HookNotInvoked(t *testing.T) {
	var hookCalled atomic.Int32
	SetOnPanic(func(name string) { hookCalled.Add(1) })
	t.Cleanup(func() { SetOnPanic(nil) })

	done := make(chan struct{})
	Go("test", func() {
		close(done)
	})
	<-done

	if hookCalled.Load() != 0 {
		t.Fatalf("hook should not be called on clean exit, got %d", hookCalled.Load())
	}
}

func TestSetOnPanic_NilDisables(t *testing.T) {
	var hookCalled atomic.Int32
	SetOnPanic(func(name string) { hookCalled.Add(1) })
	SetOnPanic(nil)

	done := make(chan struct{})
	Go("test", func() {
		defer close(done)
		panic("test")
	})
	<-done

	if hookCalled.Load() != 0 {
		t.Fatalf("hook should be disabled, got %d", hookCalled.Load())
	}
}

func TestRecoverPanic_HookPanicDoesNotPropagate(t *testing.T) {
	SetOnPanic(func(name string) {
		panic("hook itself panics")
	})
	t.Cleanup(func() { SetOnPanic(nil) })

	done := make(chan struct{})
	Go("test", func() {
		defer close(done)
		panic("worker panic")
	})
	<-done
}
