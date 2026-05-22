package anthropic

import (
	"testing"

	"github.com/cloudwego/eino/schema"
)

// TestBuildParamsWithTools_parallelToolResults verifies that consecutive schema.Tool
// messages (produced by parallel tool calls) are merged into a single user message.
// The Anthropic API rejects a tool_result block when its preceding message is not
// an assistant message that contains the matching tool_use block.
func TestBuildParamsWithTools_parallelToolResults(t *testing.T) {
	m := &chatModel{model: "claude-sonnet-4-6"}

	input := []*schema.Message{
		schema.UserMessage("do both"),
		{
			Role: schema.Assistant,
			ToolCalls: []schema.ToolCall{
				{ID: "id-a", Type: "function", Function: schema.FunctionCall{Name: "toolA", Arguments: `{}`}},
				{ID: "id-b", Type: "function", Function: schema.FunctionCall{Name: "toolB", Arguments: `{}`}},
			},
		},
		{Role: schema.Tool, Content: "result-a", ToolCallID: "id-a"},
		{Role: schema.Tool, Content: "result-b", ToolCallID: "id-b"},
	}

	params, err := m.buildParamsWithTools(input, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Expected: user(text), assistant(tool_use x2), user(tool_result x2)
	if got, want := len(params.Messages), 3; got != want {
		t.Fatalf("message count: got %d, want %d", got, want)
	}

	lastMsg := params.Messages[2]
	if got, want := len(lastMsg.Content), 2; got != want {
		t.Fatalf("tool result blocks in last message: got %d, want %d (parallel results must be batched)", got, want)
	}
}

// TestBuildParamsWithTools_singleToolResult verifies the common single-tool path is unaffected.
func TestBuildParamsWithTools_singleToolResult(t *testing.T) {
	m := &chatModel{model: "claude-sonnet-4-6"}

	input := []*schema.Message{
		schema.UserMessage("do one"),
		{
			Role: schema.Assistant,
			ToolCalls: []schema.ToolCall{
				{ID: "id-x", Type: "function", Function: schema.FunctionCall{Name: "toolX", Arguments: `{}`}},
			},
		},
		{Role: schema.Tool, Content: "result-x", ToolCallID: "id-x"},
	}

	params, err := m.buildParamsWithTools(input, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got, want := len(params.Messages), 3; got != want {
		t.Fatalf("message count: got %d, want %d", got, want)
	}

	lastMsg := params.Messages[2]
	if got, want := len(lastMsg.Content), 1; got != want {
		t.Fatalf("tool result blocks: got %d, want %d", got, want)
	}
}
