package messages

import (
	"context"
	"testing"

	"github.com/nopperabbo/kiroxy/internal/anthropic"
)

func TestFormatContextWindow(t *testing.T) {
	tests := []struct {
		size int
		want string
	}{
		{200000, "200k"},
		{128000, "128k"},
		{1000000, "1M"},
		{2000000, "2M"},
		{0, "0k"},
	}
	for _, tt := range tests {
		if got := formatContextWindow(tt.size); got != tt.want {
			t.Errorf("formatContextWindow(%d) = %q, want %q", tt.size, got, tt.want)
		}
	}
}

func TestResolveThinkingBudget(t *testing.T) {
	tests := []struct {
		name string
		req  *anthropic.Request
		want int
	}{
		{"nil thinking", &anthropic.Request{}, 0},
		{"explicit budget", &anthropic.Request{Thinking: &anthropic.ThinkingConfig{BudgetTokens: 5000}}, 5000},
		{"effort max", &anthropic.Request{Thinking: &anthropic.ThinkingConfig{}, OutputConfig: &anthropic.OutputConfig{Effort: anthropic.EffortMax}}, anthropic.ThinkingBudgetMax},
		{"effort xhigh", &anthropic.Request{Thinking: &anthropic.ThinkingConfig{}, OutputConfig: &anthropic.OutputConfig{Effort: anthropic.EffortXHigh}}, anthropic.ThinkingBudgetXHigh},
		{"effort high", &anthropic.Request{Thinking: &anthropic.ThinkingConfig{}, OutputConfig: &anthropic.OutputConfig{Effort: anthropic.EffortHigh}}, anthropic.ThinkingBudgetHigh},
		{"effort medium", &anthropic.Request{Thinking: &anthropic.ThinkingConfig{}, OutputConfig: &anthropic.OutputConfig{Effort: anthropic.EffortMedium}}, anthropic.ThinkingBudgetMedium},
		{"effort low", &anthropic.Request{Thinking: &anthropic.ThinkingConfig{}, OutputConfig: &anthropic.OutputConfig{Effort: anthropic.EffortLow}}, anthropic.ThinkingBudgetLow},
		{"empty effort defaults to medium", &anthropic.Request{Thinking: &anthropic.ThinkingConfig{}}, anthropic.ThinkingBudgetMedium},
		{"unknown effort falls back to medium", &anthropic.Request{Thinking: &anthropic.ThinkingConfig{}, OutputConfig: &anthropic.OutputConfig{Effort: "ultra"}}, anthropic.ThinkingBudgetMedium},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveThinkingBudget(context.Background(), tt.req)
			if got != tt.want {
				t.Errorf("resolveThinkingBudget() = %d, want %d", got, tt.want)
			}
		})
	}
}
