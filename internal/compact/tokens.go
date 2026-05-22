// Package compact implements the context-compression pipeline.
//
// Layer 1 – MicroCompact:   shorten oversized old turns in-place (no model call).
// Layer 2 – Strip:          truncate oversized tool-result messages (always applied).
// Layer 3 – GlobalCompact:  call the LLM to produce a conversation summary.
// Layer 4 – PTLRetry:       if the summary request is too long, drop oldest turns and retry.
package compact

import "github.com/cloudwego/eino/schema"

const charsPerToken = 4 // conservative estimate: 4 chars ≈ 1 token

// EstimateTokens returns a rough token count for a string.
func EstimateTokens(s string) int {
	if s == "" {
		return 0
	}
	return (len(s) + charsPerToken - 1) / charsPerToken
}

// EstimateMessagesTokens sums the estimated token counts of every message.
// Tool-result messages carry a 1.33× safety factor to match observed behaviour.
const toolResultSafetyFactor = 4 // numerator; denominator = 3 → ×1.33

func EstimateMessagesTokens(messages []*schema.Message) int {
	total := 0
	for _, m := range messages {
		total += EstimateTokens(m.Content)
		for _, tc := range m.ToolCalls {
			total += EstimateTokens(tc.Function.Arguments)
		}
		if m.Role == schema.Tool && m.Content != "" {
			base := EstimateTokens(m.Content)
			extra := (base * toolResultSafetyFactor / 3) - base // +33%
			total += extra
		}
	}
	return total
}
