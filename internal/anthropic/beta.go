// This file is derived from github.com/d-kuro/kirocc
// Original commit: 5633c47f0d65aaef748728bae1c68160b0ea538d
// Copyright (c) 2026 d-kuro. Licensed under Apache License, Version 2.0.
// Modifications (c) 2026 kiroxy contributors.

package anthropic

import (
	"net/http"
	"strings"
)

// HasContext1MBeta reports whether the Anthropic-Beta header set contains a
// context-1m flag. Matches any value with the "context-1m" prefix (e.g.
// "context-1m-2025-10-22").
func HasContext1MBeta(h http.Header) bool {
	for _, v := range h["Anthropic-Beta"] {
		for beta := range strings.SplitSeq(v, ",") {
			if strings.HasPrefix(strings.TrimSpace(beta), "context-1m") {
				return true
			}
		}
	}
	return false
}
