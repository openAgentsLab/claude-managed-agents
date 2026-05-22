package compact

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	model "github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

var ptlTokenGapRe = regexp.MustCompile(`\d{3,}`)

func extractTokenGapFromError(err error) int {
	if err == nil {
		return 0
	}
	matches := ptlTokenGapRe.FindAllString(err.Error(), -1)
	best := 0
	for _, m := range matches {
		n, parseErr := strconv.Atoi(m)
		if parseErr == nil && n > best {
			best = n
		}
	}
	return best
}

const (
	summaryPrompt = `Compress the conversation into a detailed summary preserving:
1. All technical decisions and their rationale
2. Current task status and next steps
3. Key code changes made
4. Important file paths and their purposes
5. Any errors encountered and solutions tried
6. All user requests (verbatim or closely paraphrased)
7. Pending tasks not yet completed
Keep code snippets that are likely to be referenced again.
Target: ~3000 tokens

Format your response as:
<summary>
[your summary here]
</summary>`

	// CompactBoundaryMarker is inserted as a user message to mark where compaction occurred.
	CompactBoundaryMarker = "[Compact boundary: conversation compressed above]"

	ptlRetryMarker = "[earlier conversation truncated for compaction retry]"
)

// GlobalResult holds the outcome of a successful global compaction.
type GlobalResult struct {
	Messages   []*schema.Message
	PreTokens  int
	PostTokens int
	Summary    string
}

// GlobalCompact calls the LLM to summarise the conversation.
// stripForCompact is applied before the LLM call (Layer 3).
// PTL retry is applied when the summarisation request itself is too long (Layer 4).
func GlobalCompact(
	ctx context.Context,
	m model.BaseChatModel,
	messages []*schema.Message,
	customInstruction string,
	maxPTLRetries int,
) (*GlobalResult, error) {
	preTokens := EstimateMessagesTokens(messages)
	stripped := stripForCompact(messages)

	result, err := globalCompactAttempt(ctx, m, stripped, customInstruction, maxPTLRetries, 0)
	if err != nil {
		return nil, err
	}
	result.PreTokens = preTokens
	result.PostTokens = EstimateMessagesTokens(result.Messages)
	return result, nil
}

func globalCompactAttempt(
	ctx context.Context,
	m model.BaseChatModel,
	messages []*schema.Message,
	customInstruction string,
	maxRetries int,
	attempt int,
) (*GlobalResult, error) {
	prompt := summaryPrompt
	if customInstruction != "" {
		prompt += "\n\nAdditional instructions: " + customInstruction
	}

	reqMessages := make([]*schema.Message, 0, len(messages)+1)
	reqMessages = append(reqMessages, messages...)
	reqMessages = append(reqMessages, schema.UserMessage(prompt))

	summaryMsg, err := m.Generate(ctx, reqMessages)
	if err != nil {
		if isPromptTooLong(err) && attempt < maxRetries {
			tokenGap := extractTokenGapFromError(err)
			var truncated []*schema.Message
			if tokenGap > 0 {
				truncated = dropOldestTurnsForTokenGap(messages, tokenGap)
			} else {
				truncated = dropOldestTurns(messages, 0.25)
			}
			if len(truncated) == len(messages) {
				return nil, fmt.Errorf("compact: prompt too long and cannot truncate further: %w", err)
			}
			truncated = ensureUserFirst(truncated)
			return globalCompactAttempt(ctx, m, truncated, customInstruction, maxRetries, attempt+1)
		}
		return nil, fmt.Errorf("compact: summarise: %w", err)
	}

	summaryText := extractSummaryContent(summaryMsg.Content)
	newMessages := []*schema.Message{
		schema.UserMessage(CompactBoundaryMarker),
		{Role: schema.Assistant, Content: "<summary>\n" + summaryText + "\n</summary>"},
	}

	return &GlobalResult{
		Messages: newMessages,
		Summary:  summaryText,
	}, nil
}

// stripForCompact removes system messages and pure tool-dispatch assistant messages
// before the summarisation LLM call, reducing the request size.
func stripForCompact(messages []*schema.Message) []*schema.Message {
	out := make([]*schema.Message, 0, len(messages))
	for _, m := range messages {
		switch m.Role {
		case schema.System:
			continue
		case schema.Assistant:
			if m.Content == "" && len(m.ToolCalls) > 0 {
				continue
			}
			if len(m.ToolCalls) > 0 {
				cp := &schema.Message{Role: schema.Assistant, Content: m.Content}
				out = append(out, cp)
				continue
			}
		}
		out = append(out, m)
	}
	return out
}

type turnPair struct {
	start int
	end   int
}

func collectTurnPairs(messages []*schema.Message) []turnPair {
	var pairs []turnPair
	i := 0
	for i < len(messages) {
		if messages[i].Role != schema.User {
			i++
			continue
		}
		userIdx := i
		j := i + 1
		for j < len(messages) && messages[j].Role != schema.Assistant {
			j++
		}
		if j < len(messages) {
			pairs = append(pairs, turnPair{userIdx, j})
			i = j + 1
		} else {
			break
		}
	}
	return pairs
}

func dropOldestTurnsForTokenGap(messages []*schema.Message, tokenGap int) []*schema.Message {
	pairs := collectTurnPairs(messages)
	if len(pairs) == 0 {
		return messages
	}
	freed := 0
	dropCount := 0
	for _, p := range pairs {
		freed += EstimateTokens(messages[p.start].Content)
		freed += EstimateTokens(messages[p.end].Content)
		dropCount++
		if freed >= tokenGap {
			break
		}
	}
	if dropCount >= len(pairs) {
		dropCount = len(pairs) - 1
		if dropCount == 0 {
			dropCount = 1
		}
	}
	if dropCount >= len(pairs) {
		return nil
	}
	return messages[pairs[dropCount].start:]
}

func dropOldestTurns(messages []*schema.Message, fraction float64) []*schema.Message {
	pairs := collectTurnPairs(messages)
	if len(pairs) == 0 {
		return messages
	}
	toDrop := int(float64(len(pairs)) * fraction)
	if toDrop == 0 {
		toDrop = 1
	}
	if toDrop >= len(pairs) {
		toDrop = len(pairs) - 1
	}
	return messages[pairs[toDrop].start:]
}

func ensureUserFirst(messages []*schema.Message) []*schema.Message {
	if len(messages) == 0 || messages[0].Role != schema.Assistant {
		return messages
	}
	out := make([]*schema.Message, 0, len(messages)+1)
	out = append(out, schema.UserMessage(ptlRetryMarker))
	out = append(out, messages...)
	return out
}

func extractSummaryContent(content string) string {
	const open, close = "<summary>", "</summary>"
	start := strings.Index(content, open)
	if start == -1 {
		return content
	}
	inner := content[start+len(open):]
	end := strings.LastIndex(inner, close)
	if end == -1 {
		return strings.TrimSpace(inner)
	}
	return strings.TrimSpace(inner[:end])
}

// ptlCodeErr is satisfied by Anthropic SDK errors and any wrapper that exposes
// a string error-type code (e.g. "prompt_too_long"). Prefer this over string
// matching when available.
type ptlCodeErr interface {
	error
	ErrorCode() string
}

func isPromptTooLong(err error) bool {
	if err == nil {
		return false
	}
	// Structured detection: works when the error (or any wrapped cause) exposes
	// an ErrorCode() method, as the Anthropic SDK's apierror.APIError does.
	var coded ptlCodeErr
	if errors.As(err, &coded) {
		return strings.EqualFold(coded.ErrorCode(), "prompt_too_long")
	}
	// Fallback: string matching for SDK errors wrapped by Eino or other layers.
	// Keep only the most specific patterns to avoid false positives.
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "prompt_too_long") ||
		strings.Contains(s, "context_length_exceeded") ||
		strings.Contains(s, "maximum context length")
}
