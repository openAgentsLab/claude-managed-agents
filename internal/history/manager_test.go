package history

import (
	"testing"

	"github.com/cloudwego/eino/schema"
)

func toolCall(id string) schema.ToolCall {
	return schema.ToolCall{ID: id, Function: schema.FunctionCall{Name: "fn"}}
}

func assistantWithCalls(ids ...string) *schema.Message {
	calls := make([]schema.ToolCall, len(ids))
	for i, id := range ids {
		calls[i] = toolCall(id)
	}
	return &schema.Message{Role: schema.Assistant, ToolCalls: calls}
}

func toolResult(id string) *schema.Message {
	return &schema.Message{Role: schema.Tool, ToolCallID: id, Content: "ok"}
}

func TestSanitizeToolPairs_AllPaired(t *testing.T) {
	msgs := []*schema.Message{
		schema.UserMessage("hi"),
		assistantWithCalls("c1", "c2"),
		toolResult("c1"),
		toolResult("c2"),
	}
	got := sanitizeToolPairs(msgs)
	if len(got) != 4 {
		t.Fatalf("expected 4 messages, got %d", len(got))
	}
}

func TestSanitizeToolPairs_OrphanedResult(t *testing.T) {
	// tool result with no matching tool call → dropped
	msgs := []*schema.Message{
		schema.UserMessage("hi"),
		toolResult("ghost"),
	}
	got := sanitizeToolPairs(msgs)
	if len(got) != 1 {
		t.Fatalf("expected 1 message after dropping orphaned result, got %d", len(got))
	}
	if got[0].Role != schema.User {
		t.Fatalf("remaining message should be user, got %s", got[0].Role)
	}
}

func TestSanitizeToolPairs_OrphanedCall(t *testing.T) {
	// tool call with no matching result → assistant message dropped entirely (no content)
	msgs := []*schema.Message{
		schema.UserMessage("hi"),
		assistantWithCalls("c1"),
	}
	got := sanitizeToolPairs(msgs)
	if len(got) != 1 {
		t.Fatalf("expected 1 message after dropping orphaned call message, got %d", len(got))
	}
}

func TestSanitizeToolPairs_PartialCalls(t *testing.T) {
	// assistant has two calls; only one has a result → unpaired call stripped
	msgs := []*schema.Message{
		schema.UserMessage("hi"),
		assistantWithCalls("c1", "c2"),
		toolResult("c1"),
	}
	got := sanitizeToolPairs(msgs)
	// expect: user, assistant(c1 only), toolResult(c1)
	if len(got) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(got))
	}
	if len(got[1].ToolCalls) != 1 || got[1].ToolCalls[0].ID != "c1" {
		t.Fatalf("assistant message should contain only c1, got %v", got[1].ToolCalls)
	}
}

func TestSanitizeToolPairs_AssistantWithContentAndOrphanedCall(t *testing.T) {
	// assistant has text content + orphaned tool call → keep message, strip call
	msg := &schema.Message{
		Role:      schema.Assistant,
		Content:   "thinking…",
		ToolCalls: []schema.ToolCall{toolCall("c1")},
	}
	got := sanitizeToolPairs([]*schema.Message{msg})
	if len(got) != 1 {
		t.Fatalf("expected message to survive (has content), got %d messages", len(got))
	}
	if len(got[0].ToolCalls) != 0 {
		t.Fatalf("orphaned tool call should be stripped, got %v", got[0].ToolCalls)
	}
	if got[0].Content != "thinking…" {
		t.Fatalf("content should be preserved, got %q", got[0].Content)
	}
}

func TestSanitizeToolPairs_EmptyInput(t *testing.T) {
	got := sanitizeToolPairs(nil)
	if len(got) != 0 {
		t.Fatalf("expected empty output for nil input")
	}
}

func TestSanitizeToolPairs_NoToolMessages(t *testing.T) {
	msgs := []*schema.Message{
		schema.UserMessage("a"),
		schema.AssistantMessage("b", nil),
	}
	got := sanitizeToolPairs(msgs)
	if len(got) != 2 {
		t.Fatalf("expected 2 messages unchanged, got %d", len(got))
	}
}

func TestSanitizeToolPairs_OriginalUnmodified(t *testing.T) {
	// sanitizeToolPairs must not mutate the original message when filtering calls
	orig := assistantWithCalls("c1", "c2")
	msgs := []*schema.Message{orig, toolResult("c1")}
	sanitizeToolPairs(msgs)
	if len(orig.ToolCalls) != 2 {
		t.Fatalf("original message ToolCalls should be unmodified, got %d", len(orig.ToolCalls))
	}
}
