// This file is derived from github.com/d-kuro/kirocc
// Original commit: 5633c47f0d65aaef748728bae1c68160b0ea538d
// Copyright (c) 2026 d-kuro. Licensed under Apache License, Version 2.0.
// Modifications (c) 2026 kiroxy contributors (kiroxy-specific target routing).

package kiroclient

import "github.com/nopperabbo/kiroxy/internal/kiroproto"

// chooseAmzTarget picks the AWS RPC target header based on whether the caller
// supplied a profileArn. The CodeWhisperer target rejects requests without
// profileArn; the AmazonQ target accepts them. Builder ID OAuth accounts don't
// obtain a profileArn so they need the AmazonQ target. kiro-cli social-auth
// accounts do carry profileArn and continue to use CodeWhisperer.
//
// This mirrors the behavior of Quorinex/Kiro-Go's dual-endpoint fallback, but
// collapsed to a single decision up front because we already know which
// credential source each account uses.
func chooseAmzTarget(p *kiroproto.Payload) string {
	if p != nil && p.ProfileARN != "" {
		return amzTargetCodeWhisperer
	}
	return amzTargetAmazonQ
}
