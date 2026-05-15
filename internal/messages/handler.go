// This file is derived from github.com/d-kuro/kirocc
// Original commit: 5633c47f0d65aaef748728bae1c68160b0ea538d
// Copyright (c) 2026 d-kuro. Licensed under Apache License, Version 2.0.
// Modifications (c) 2026 kiroxy contributors.

package messages

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/google/uuid"

	"github.com/nopperabbo/kiroxy/internal/anthropic"
	"github.com/nopperabbo/kiroxy/internal/auth"
	"github.com/nopperabbo/kiroxy/internal/httpx"
	"github.com/nopperabbo/kiroxy/internal/logging"
	"github.com/nopperabbo/kiroxy/internal/metrics"
	"github.com/nopperabbo/kiroxy/internal/models"
	"github.com/nopperabbo/kiroxy/internal/reqconv"
	"github.com/nopperabbo/kiroxy/internal/toolsearch"
)

const headerCCSessionID = "X-Claude-Code-Session-Id"

func (s *Service) HandleMessages(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	traceID, short := logging.TraceIDs(ctx)

	// Metrics scaffolding: wrap the writer so the final status code can be
	// recovered in the deferred finalize, and start a tracker whose lifetime
	// matches the handler. Both are zero-cost when no metrics sink is wired.
	lw := &statusCapturingWriter{ResponseWriter: w}
	w = lw
	rm := s.newRequestMetrics()
	defer func() { rm.finalize(lw.currentStatus()) }()

	req, err := parseAndValidateRequest(ctx, w, r)
	if err != nil {
		slog.WarnContext(ctx, "invalid request", "trace_id", short, "err", err)
		rm.errKind(metrics.RequestKindInvalidRequest)
		httpx.WriteError(w, http.StatusBadRequest, errTypeInvalidRequest, err.Error())
		return
	}

	ccSessionID := r.Header.Get(headerCCSessionID)
	if ccSessionID == "" {
		// Non-Claude-Code clients (e.g. plain Anthropic BYOK) don't emit this
		// header. Synthesize one so session stickiness and logging still work,
		// matching the pattern used by the OpenAI-compat endpoint.
		ccSessionID = "byok-" + uuid.New().String()
		r.Header.Set(headerCCSessionID, ccSessionID)
	}
	ctx = logging.WithSessionID(ctx, ccSessionID)
	r = r.WithContext(ctx)

	slog.DebugContext(ctx, "client request headers",
		"trace_id", traceID,
		"session_id", ccSessionID,
		"headers", logging.SafeHeaders{H: r.Header},
	)

	creds, err := s.auth.GetToken(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "auth error", "trace_id", short, "err", err)
		rm.errKind(metrics.RequestKindAuth)
		httpx.WriteError(w, http.StatusUnauthorized, ErrTypeAuthentication, "authentication failed")
		return
	}

	kiroModel, thinking, contextWindowSize, anthropicModel := models.Resolve(req.Model, anthropic.HasContext1MBeta(r.Header))
	if req.IsThinkingEnabled() {
		thinking = true
	}
	rm.setModel(anthropicModel, req.Stream)

	s.logRequest(ctx, short, ccSessionID, kiroModel, contextWindowSize, req, thinking)

	thinkingBudget := resolveThinkingBudget(ctx, req)

	// Tool search short-circuits to the orchestrator, which has its own retry loop.
	if tsCtx := toolsearch.NewContext(req.Tools); tsCtx != nil {
		refs := reqconv.ExtractToolReferences(req.Messages)
		tsCtx.PromoteTools(refs)
		slog.InfoContext(ctx, "tool search enabled",
			"trace_id", short,
			"search_type", tsCtx.SearchType,
			"deferred_tools", len(tsCtx.DeferredTools),
			"active_tools", len(tsCtx.ActiveTools),
		)
		s.runToolSearch(ctx, w, req, creds, tsCtx, kiroModel, anthropicModel, contextWindowSize, thinking, thinkingBudget, ccSessionID, short)
		return
	}

	payload, nameMap, err := reqconv.BuildPayload(req, reqconv.BuildOptions{
		ProfileARN:     creds.ProfileARN,
		ModelID:        kiroModel,
		ConversationID: ccSessionID,
		Thinking:       thinking,
		ThinkingBudget: thinkingBudget,
	})
	if err != nil {
		slog.WarnContext(ctx, "payload build error", "trace_id", short, "err", err)
		rm.errKind(metrics.RequestKindProxy)
		httpx.WriteError(w, http.StatusBadRequest, errTypeInvalidRequest, err.Error())
		return
	}

	s.executeWithRetry(ctx, w, &invocation{
		req:               req,
		payload:           payload,
		creds:             creds,
		model:             kiroModel,
		responseModel:     anthropicModel,
		contextWindowSize: contextWindowSize,
		thinking:          thinking,
		toolNameMap:       nameMap.ReverseMap(),
		metrics:           rm,
	})
}

// logRequest emits the "--> POST /v1/messages" info log summarizing the call.
func (s *Service) logRequest(ctx context.Context, short, ccSessionID, kiroModel string, contextWindowSize int, req *anthropic.Request, thinking bool) {
	var thinkingLog any = false
	if thinking {
		if effort := req.Effort(); effort != "" {
			thinkingLog = effort
		} else {
			thinkingLog = "enabled"
		}
	}
	slog.InfoContext(ctx, "--> POST /v1/messages",
		"trace_id", short,
		"session_id", logging.ShortID(ccSessionID),
		"model", kiroModel,
		"thinking", thinkingLog,
		"stream", req.Stream,
		"context_window", formatContextWindow(contextWindowSize),
	)
}

// runToolSearch wires up the orchestrator and retries once on empty-visible end_turn.
func (s *Service) runToolSearch(ctx context.Context, w http.ResponseWriter, req *anthropic.Request, creds *auth.Credentials, tsCtx *toolsearch.Context, kiroModel, responseModel string, contextWindowSize int, thinking bool, thinkingBudget int, ccSessionID, short string) {
	orch := &toolSearchOrchestrator{
		service: s,
		tsCtx:   tsCtx,
		req:     req,
		creds:   creds,
		buildOpts: reqconv.BuildOptions{
			ProfileARN:     creds.ProfileARN,
			ModelID:        kiroModel,
			ConversationID: ccSessionID,
			Thinking:       thinking,
			ThinkingBudget: thinkingBudget,
			ToolSearchCtx:  tsCtx,
		},
		contextWindowSize: contextWindowSize,
		responseModel:     responseModel,
	}
	reason := orch.run(ctx, w)
	if reason != retryReasonEmptyVisibleEndTurn {
		return
	}
	slog.WarnContext(ctx, "retrying tool search after empty visible end_turn", "trace_id", short)
	if r2 := orch.run(ctx, w); r2 == retryReasonEmptyVisibleEndTurn {
		slog.ErrorContext(ctx, "tool search retry also returned empty visible end_turn", "trace_id", short)
		httpx.WriteError(w, http.StatusBadGateway, errTypeAPI, "upstream returned empty response")
	}
}
