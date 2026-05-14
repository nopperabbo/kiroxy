// This file is derived from github.com/d-kuro/kirocc
// Original commit: 5633c47f0d65aaef748728bae1c68160b0ea538d
// Copyright (c) 2026 d-kuro. Licensed under Apache License, Version 2.0.
// Modifications (c) 2026 kiroxy contributors.

package kiroclient

import (
	"io"
	"time"
)

// idleReader wraps an io.ReadCloser and enforces an idle timeout per Read call.
// If no data is read within the configured duration, Read returns ErrBodyReadIdle.
//
// The timeout recovery works by calling Close on the underlying reader, which
// is guaranteed to unblock a pending Read for net/http.Response.Body. This is
// NOT a general guarantee for arbitrary io.ReadCloser implementations.
type idleReader struct {
	rc   io.ReadCloser
	idle time.Duration
}

func (r *idleReader) Read(p []byte) (int, error) {
	type result struct {
		n   int
		err error
	}
	ch := make(chan result, 1) // buffered: sender never blocks even if we time out
	go func() {
		n, err := r.rc.Read(p)
		ch <- result{n, err}
	}()
	t := time.NewTimer(r.idle)
	defer t.Stop()
	select {
	case res := <-ch:
		return res.n, res.err
	case <-t.C:
		// Close unblocks the pending Read but does not synchronize. We MUST
		// wait for the producer to return before yielding control: otherwise
		// the caller (typically bufio.Reader) reuses p while the producer is
		// still writing into it, causing a data race on the byte slice.
		_ = r.rc.Close()
		<-ch
		return 0, ErrBodyReadIdle
	}
}

func (r *idleReader) Close() error { return r.rc.Close() }
