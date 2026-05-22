package compact

import (
	"fmt"

	"github.com/cloudwego/eino/schema"
)

// MicroCompact is Layer 1 of the compression pipeline.
// It shortens any individual message whose content exceeds maxCharsPerMsg,
// protecting the most recent keepTurns user+assistant pairs from trimming.
// No LLM call is made.
func MicroCompact(messages []*schema.Message, keepTurns int, maxCharsPerMsg int) []*schema.Message {
	if len(messages) == 0 {
		return messages
	}
	protected := protectedBoundary(messages, keepTurns)
	out := make([]*schema.Message, len(messages))
	for i, m := range messages {
		if i >= protected {
			out[i] = m
			continue
		}
		out[i] = trimMessage(m, maxCharsPerMsg)
	}
	return out
}

// StripToolResults truncates tool-result messages (schema.Tool role) whose
// content exceeds maxChars. This is always applied before sending to the LLM
// and before deciding whether GlobalCompact is needed.
func StripToolResults(messages []*schema.Message, maxChars int) []*schema.Message {
	if maxChars <= 0 {
		return messages
	}
	out := make([]*schema.Message, len(messages))
	for i, m := range messages {
		if m.Role == schema.Tool && len(m.Content) > maxChars {
			cp := *m
			omitted := len(m.Content) - maxChars
			cp.Content = m.Content[:maxChars] + fmt.Sprintf("\n[... %d chars omitted ...]", omitted)
			out[i] = &cp
		} else {
			out[i] = m
		}
	}
	return out
}

func protectedBoundary(messages []*schema.Message, keepTurns int) int {
	if keepTurns <= 0 {
		return len(messages)
	}
	pairs := 0
	i := len(messages) - 1
	for i >= 0 {
		if messages[i].Role == schema.Assistant {
			if i > 0 && messages[i-1].Role == schema.User {
				pairs++
				i -= 2
			} else {
				i--
			}
			if pairs >= keepTurns {
				return i + 1
			}
		} else {
			i--
		}
	}
	return 0
}

func trimMessage(m *schema.Message, maxChars int) *schema.Message {
	if maxChars <= 0 {
		return m
	}
	contentTrimmed := trimStr(m.Content, maxChars)

	var toolCalls []schema.ToolCall
	if len(m.ToolCalls) > 0 {
		toolCalls = make([]schema.ToolCall, len(m.ToolCalls))
		for j, tc := range m.ToolCalls {
			if len(tc.Function.Arguments) > maxChars {
				omitted := len(tc.Function.Arguments) - maxChars
				tc.Function.Arguments = fmt.Sprintf(
					`{"_truncated_by_micro_compact": "%d chars removed"}`, omitted)
			}
			toolCalls[j] = tc
		}
	}

	if contentTrimmed == m.Content && len(toolCalls) == 0 {
		return m
	}
	cp := *m
	cp.Content = contentTrimmed
	if toolCalls != nil {
		cp.ToolCalls = toolCalls
	}
	return &cp
}

func trimStr(s string, maxChars int) string {
	if maxChars <= 0 || len(s) <= maxChars {
		return s
	}
	omitted := len(s) - maxChars
	return s[:maxChars] + fmt.Sprintf("\n[... %d chars omitted by micro-compact ...]", omitted)
}
