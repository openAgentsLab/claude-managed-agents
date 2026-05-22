package session

import (
	"testing"
)

func TestToMessages_UserAndAssistant(t *testing.T) {
	events := []Event{
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: "world"},
		{Role: "user", Content: "again"},
	}
	msgs := ToMessages(events)
	if len(msgs) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(msgs))
	}
	if msgs[0].Content != "hello" {
		t.Errorf("msgs[0].Content = %q, want %q", msgs[0].Content, "hello")
	}
	if msgs[1].Content != "world" {
		t.Errorf("msgs[1].Content = %q, want %q", msgs[1].Content, "world")
	}
}

func TestToMessages_IncludesToolResult(t *testing.T) {
	events := []Event{
		{Role: "user", Content: "run it"},
		{Role: "tool_result", Content: "output", ToolName: "bash", ToolCallID: "call-1"},
		{Role: "assistant", Content: "done"},
	}
	msgs := ToMessages(events)
	if len(msgs) != 3 {
		t.Fatalf("expected 3 messages (user + tool_result + assistant), got %d", len(msgs))
	}
	if string(msgs[1].Role) != "tool" {
		t.Errorf("msgs[1].Role = %q, want %q", msgs[1].Role, "tool")
	}
	if msgs[1].ToolCallID != "call-1" {
		t.Errorf("msgs[1].ToolCallID = %q, want %q", msgs[1].ToolCallID, "call-1")
	}
}

func TestToMessages_ToolCallFlushed(t *testing.T) {
	callJSON := `{"id":"cid","type":"function","function":{"name":"bash","arguments":"{\"command\":\"ls\"}"}}`
	events := []Event{
		{Role: "user", Content: "do it"},
		{Role: "tool_call", Content: callJSON, ToolName: "bash"},
		{Role: "tool_result", Content: "file.go", ToolCallID: "cid"},
		{Role: "assistant", Content: "done"},
	}
	msgs := ToMessages(events)
	// user + assistant-with-toolcalls + tool-result + assistant
	if len(msgs) != 4 {
		t.Fatalf("expected 4 messages, got %d", len(msgs))
	}
	if len(msgs[1].ToolCalls) != 1 {
		t.Errorf("expected 1 tool call on msgs[1], got %d", len(msgs[1].ToolCalls))
	}
	if string(msgs[2].Role) != "tool" {
		t.Errorf("msgs[2].Role = %q, want %q", msgs[2].Role, "tool")
	}
}

func TestToMessages_Empty(t *testing.T) {
	msgs := ToMessages(nil)
	if len(msgs) != 0 {
		t.Errorf("expected empty slice for nil input, got %d", len(msgs))
	}
	msgs = ToMessages([]Event{})
	if len(msgs) != 0 {
		t.Errorf("expected empty slice for empty input, got %d", len(msgs))
	}
}

func TestToMessages_PreservesContent(t *testing.T) {
	want := "multi\nline\ncontent"
	events := []Event{{Role: "user", Content: want}}
	msgs := ToMessages(events)
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	if msgs[0].Content != want {
		t.Errorf("content mismatch: got %q, want %q", msgs[0].Content, want)
	}
}
