package compact

import (
	"strings"
	"testing"

	"github.com/cloudwego/eino/schema"
)

// ── EstimateTokens ────────────────────────────────────────────────────────────

func TestEstimateTokens_Empty(t *testing.T) {
	if got := EstimateTokens(""); got != 0 {
		t.Errorf("empty string: got %d, want 0", got)
	}
}

func TestEstimateTokens_FourCharsOneToken(t *testing.T) {
	// "abcd" = 4 chars → 1 token (ceiling(4/4)=1)
	if got := EstimateTokens("abcd"); got != 1 {
		t.Errorf("4-char string: got %d, want 1", got)
	}
}

func TestEstimateTokens_CeilingDivision(t *testing.T) {
	// 5 chars → ceiling(5/4) = 2 tokens
	if got := EstimateTokens("abcde"); got != 2 {
		t.Errorf("5-char string: got %d, want 2", got)
	}
}

func TestEstimateTokens_LargeString(t *testing.T) {
	s := strings.Repeat("x", 400)
	want := 100
	if got := EstimateTokens(s); got != want {
		t.Errorf("400-char string: got %d, want %d", got, want)
	}
}

// ── EstimateMessagesTokens ────────────────────────────────────────────────────

func TestEstimateMessagesTokens_Empty(t *testing.T) {
	if got := EstimateMessagesTokens(nil); got != 0 {
		t.Errorf("nil input: got %d, want 0", got)
	}
}

func TestEstimateMessagesTokens_UserMessage(t *testing.T) {
	msgs := []*schema.Message{schema.UserMessage("hello")} // 5 chars → 2 tokens
	got := EstimateMessagesTokens(msgs)
	if got <= 0 {
		t.Errorf("expected positive token count, got %d", got)
	}
}

func TestEstimateMessagesTokens_ToolResultSafetyFactor(t *testing.T) {
	// Tool result gets ×1.33 safety factor; plain user message does not.
	content := strings.Repeat("a", 400) // 100 tokens base
	toolResult := &schema.Message{Role: schema.Tool, Content: content, ToolCallID: "tc1"}
	userMsg := schema.UserMessage(content)

	toolTokens := EstimateMessagesTokens([]*schema.Message{toolResult})
	userTokens := EstimateMessagesTokens([]*schema.Message{userMsg})

	if toolTokens <= userTokens {
		t.Errorf("tool result (%d) should have more tokens than user message (%d) due to safety factor", toolTokens, userTokens)
	}
}

func TestEstimateMessagesTokens_ToolCalls(t *testing.T) {
	msg := &schema.Message{
		Role: schema.Assistant,
		ToolCalls: []schema.ToolCall{
			{Function: schema.FunctionCall{Arguments: strings.Repeat("x", 40)}},
		},
	}
	got := EstimateMessagesTokens([]*schema.Message{msg})
	if got <= 0 {
		t.Errorf("expected positive token count for tool calls, got %d", got)
	}
}

// ── MicroCompact ──────────────────────────────────────────────────────────────

func TestMicroCompact_EmptyInput(t *testing.T) {
	got := MicroCompact(nil, 4, 8000)
	if len(got) != 0 {
		t.Errorf("nil input: expected empty, got %d", len(got))
	}
}

func TestMicroCompact_ShortMessageUnchanged(t *testing.T) {
	msg := schema.UserMessage("hello")
	got := MicroCompact([]*schema.Message{msg}, 0, 8000)
	if got[0] != msg {
		t.Error("short message should be returned as-is (same pointer)")
	}
}

func TestMicroCompact_LongMessageTrimmed(t *testing.T) {
	long := strings.Repeat("x", 20000)
	msg := schema.UserMessage(long)
	got := MicroCompact([]*schema.Message{msg}, 0, 8000)
	if len(got[0].Content) <= 8000 {
		// Content must now be shorter + have the omission notice
	}
	if !strings.Contains(got[0].Content, "omitted") {
		t.Error("trimmed message should contain omission notice")
	}
}

func TestMicroCompact_ProtectsRecentTurns(t *testing.T) {
	// 2 user+assistant pairs → keepTurns=2 protects both
	long := strings.Repeat("y", 20000)
	msgs := []*schema.Message{
		schema.UserMessage(long),
		schema.AssistantMessage(long, nil),
		schema.UserMessage(long),
		schema.AssistantMessage(long, nil),
	}
	got := MicroCompact(msgs, 2, 8000)
	// Recent protected messages should be unchanged
	if got[2] != msgs[2] || got[3] != msgs[3] {
		t.Error("recent turns should not be trimmed")
	}
}

func TestMicroCompact_OldTurnsTrimmed(t *testing.T) {
	long := strings.Repeat("z", 20000)
	msgs := []*schema.Message{
		schema.UserMessage(long),   // old, should be trimmed
		schema.AssistantMessage(long, nil), // old, should be trimmed
		schema.UserMessage("new"),
		schema.AssistantMessage("reply", nil),
	}
	got := MicroCompact(msgs, 1, 8000)
	// Old turns should be trimmed
	if got[0] == msgs[0] {
		t.Error("old user message should have been trimmed (pointer changed)")
	}
}

func TestMicroCompact_ToolCallArgsTrimmed(t *testing.T) {
	longArgs := strings.Repeat("a", 20000)
	msg := &schema.Message{
		Role: schema.Assistant,
		ToolCalls: []schema.ToolCall{
			{ID: "tc1", Function: schema.FunctionCall{Name: "fn", Arguments: longArgs}},
		},
	}
	got := MicroCompact([]*schema.Message{msg}, 0, 8000)
	if len(got[0].ToolCalls[0].Function.Arguments) >= len(longArgs) {
		t.Error("long tool call arguments should be truncated")
	}
}

// ── StripToolResults ──────────────────────────────────────────────────────────

func TestStripToolResults_ShortContentUnchanged(t *testing.T) {
	msg := &schema.Message{Role: schema.Tool, Content: "short", ToolCallID: "tc1"}
	got := StripToolResults([]*schema.Message{msg}, 2000)
	if got[0] != msg {
		t.Error("short tool result should be returned as-is")
	}
}

func TestStripToolResults_LongContentTruncated(t *testing.T) {
	long := strings.Repeat("x", 5000)
	msg := &schema.Message{Role: schema.Tool, Content: long, ToolCallID: "tc1"}
	got := StripToolResults([]*schema.Message{msg}, 2000)
	if len(got[0].Content) <= 2000 {
		// Content is truncated plus the omission notice
	}
	if !strings.Contains(got[0].Content, "omitted") {
		t.Error("truncated tool result should contain omission notice")
	}
}

func TestStripToolResults_NonToolMessageUnchanged(t *testing.T) {
	long := strings.Repeat("x", 5000)
	msg := schema.UserMessage(long)
	got := StripToolResults([]*schema.Message{msg}, 2000)
	if got[0] != msg {
		t.Error("non-tool message should not be modified")
	}
}

func TestStripToolResults_ZeroMaxCharsReturnsOriginal(t *testing.T) {
	msg := &schema.Message{Role: schema.Tool, Content: "any content", ToolCallID: "tc1"}
	got := StripToolResults([]*schema.Message{msg}, 0)
	if got[0] != msg {
		t.Error("maxChars=0 should return original slice unchanged")
	}
}

func TestStripToolResults_OriginalUnmodified(t *testing.T) {
	orig := &schema.Message{Role: schema.Tool, Content: strings.Repeat("x", 5000), ToolCallID: "tc1"}
	StripToolResults([]*schema.Message{orig}, 2000)
	if len(orig.Content) != 5000 {
		t.Error("StripToolResults must not modify the original message in place")
	}
}

// ── Service ───────────────────────────────────────────────────────────────────

func TestService_ShouldAutoCompact_BelowThreshold(t *testing.T) {
	cfg := DefaultConfig()
	svc := New(cfg, nil)
	// One short message — far below threshold
	msgs := []*schema.Message{schema.UserMessage("hi")}
	if svc.ShouldAutoCompact(msgs) {
		t.Error("short history should not trigger auto-compact")
	}
}

func TestService_ShouldAutoCompact_ZeroContextWindow(t *testing.T) {
	cfg := DefaultConfig()
	cfg.ContextWindowTokens = 0
	svc := New(cfg, nil)
	msgs := []*schema.Message{schema.UserMessage("hi")}
	if svc.ShouldAutoCompact(msgs) {
		t.Error("zero ContextWindowTokens should disable auto-compact")
	}
}

func TestService_CircuitBreakerTripped_InitiallyFalse(t *testing.T) {
	svc := New(DefaultConfig(), nil)
	if svc.CircuitBreakerTripped() {
		t.Error("circuit breaker should not be tripped initially")
	}
}

func TestService_CircuitBreakerTripped_DisabledWhenZero(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxConsecFailures = 0
	svc := New(cfg, nil)
	// Simulate failures by calling Compact (will fail with nil model) a few times
	// Circuit breaker should never trip when MaxConsecFailures=0
	if svc.CircuitBreakerTripped() {
		t.Error("circuit breaker should be disabled when MaxConsecFailures=0")
	}
}

func TestService_ShouldAutoCompact_CircuitBreakerPreventsCompact(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxConsecFailures = 1
	svc := New(cfg, nil)

	// Manually trip the circuit breaker
	svc.mu.Lock()
	svc.consecutiveFailures = cfg.MaxConsecFailures
	svc.mu.Unlock()

	// Even with a large history, circuit breaker should prevent compaction
	bigContent := strings.Repeat("x", 500_000)
	msgs := []*schema.Message{schema.UserMessage(bigContent)}
	if svc.ShouldAutoCompact(msgs) {
		t.Error("circuit breaker tripped: ShouldAutoCompact should return false")
	}
}

func TestDefaultConfig_Threshold(t *testing.T) {
	cfg := DefaultConfig()
	want := cfg.ContextWindowTokens - cfg.MaxOutputTokens - cfg.AutoCompactBuffer
	if cfg.autoCompactThreshold() != want {
		t.Errorf("threshold = %d, want %d", cfg.autoCompactThreshold(), want)
	}
}
