// Package anthropic provides an eino model.ToolCallingChatModel adapter
// backed by the Anthropic Claude Messages API.
package anthropic

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	anthropic "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

const (
	defaultMaxTokens = 4096

	// extraKeyThinkingBlocks is the key in schema.Message.Extra that stores the
	// full thinking-block payloads (thinking text + signature) produced by the
	// model.  Each entry must be round-tripped back verbatim when the message is
	// included in a subsequent request, because the Anthropic API requires the
	// original signature to validate the thinking block.
	extraKeyThinkingBlocks = "thinking_blocks"
)

// thinkingEntry mirrors one Anthropic ThinkingBlock returned by the model.
// It is stored as []thinkingEntry under extraKeyThinkingBlocks in
// schema.Message.Extra so the signature survives across turns.
type thinkingEntry struct {
	Thinking  string `json:"thinking"`
	Signature string `json:"signature"`
}

// Config holds credentials and model selection for Claude.
type Config struct {
	APIKey  string
	BaseURL string // optional: override the default Anthropic API endpoint
	Model   string

	// MaxTokens overrides the default output token cap (4096). When 0 the
	// default is used. Lower values reduce latency for short-output tasks such
	// as safety classifiers or title generation.
	MaxTokens int

	// ThinkingBudgetTokens enables Extended Thinking when > 0.
	// The value is the maximum number of tokens the model may spend reasoning
	// before producing a visible reply.  Recommended minimum: 1024.
	// Note: Extended Thinking requires claude-3-7-sonnet or later and forces
	// MaxTokens to be at least ThinkingBudgetTokens+1.
	ThinkingBudgetTokens int
}

type chatModel struct {
	client               anthropic.Client
	model                string
	maxTokens            int // 0 means use defaultMaxTokens
	thinkingBudgetTokens int
	tools                []anthropic.ToolUnionParam
	toolInfos            []*schema.ToolInfo // kept for callback reporting
}

// IsCallbacksEnabled signals to the Eino framework that this component fires
// its own callbacks, so the framework should not inject a simplified wrapper
// that omits tool info.
func (m *chatModel) IsCallbacksEnabled() bool { return true }

// NewChatModel creates a Claude adapter that satisfies model.ToolCallingChatModel.
func NewChatModel(_ context.Context, cfg Config) (model.ToolCallingChatModel, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("ANTHROPIC_API_KEY is required")
	}
	opts := []option.RequestOption{option.WithAPIKey(cfg.APIKey)}
	if cfg.BaseURL != "" {
		opts = append(opts, option.WithBaseURL(cfg.BaseURL))
	}
	return &chatModel{
		client:               anthropic.NewClient(opts...),
		model:                cfg.Model,
		maxTokens:            cfg.MaxTokens,
		thinkingBudgetTokens: cfg.ThinkingBudgetTokens,
	}, nil
}

// WithMaxTokens returns a shallow copy of the model with the output token cap
// replaced. Implements the optional maxTokensModel interface used by brain.New.
func (m *chatModel) WithMaxTokens(n int) model.ToolCallingChatModel {
	clone := *m
	clone.maxTokens = n
	return &clone
}

// resolveTools merges tools from call options (highest priority) with the model's
// bound tools, matching the pattern used by the Eino react graph which passes
// tools via model.WithTools as a call option rather than via the WithTools method.
func (m *chatModel) resolveTools(opts []model.Option) ([]*schema.ToolInfo, []anthropic.ToolUnionParam, error) {
	commonOpts := model.GetCommonOptions(&model.Options{Tools: m.toolInfos}, opts...)
	toolInfos := commonOpts.Tools
	if len(toolInfos) == 0 {
		return nil, nil, nil
	}
	// If tools came from opts (not from WithTools), convert them now.
	if len(toolInfos) != len(m.toolInfos) {
		converted, err := convertToolInfos(toolInfos)
		if err != nil {
			return nil, nil, err
		}
		return toolInfos, converted, nil
	}
	return m.toolInfos, m.tools, nil
}

// Generate calls the Claude API and returns the complete response.
func (m *chatModel) Generate(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	toolInfos, tools, err := m.resolveTools(opts)
	if err != nil {
		return nil, err
	}
	ctx = callbacks.OnStart(ctx, &model.CallbackInput{Messages: input, Tools: toolInfos})
	params, err := m.buildParamsWithTools(input, tools)
	if err != nil {
		callbacks.OnError(ctx, err)
		return nil, err
	}
	resp, err := m.client.Messages.New(ctx, params)
	if err != nil {
		callbacks.OnError(ctx, fmt.Errorf("anthropic generate: %w", err))
		return nil, fmt.Errorf("anthropic generate: %w", err)
	}
	if raw, e := json.Marshal(resp); e == nil {
		slog.Debug("anthropic generate raw response", "body", string(raw))
	}
	msg := convertResponse(resp)
	callbacks.OnEnd(ctx, &model.CallbackOutput{Message: msg})
	return msg, nil
}

// Stream calls the Claude API with streaming and returns an eino StreamReader.
// Text deltas are yielded immediately; thinking and tool calls are sent as a
// final chunk after the stream ends.
func (m *chatModel) Stream(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	toolInfos, tools, err := m.resolveTools(opts)
	if err != nil {
		return nil, err
	}
	ctx = callbacks.OnStart(ctx, &model.CallbackInput{Messages: input, Tools: toolInfos})
	params, err := m.buildParamsWithTools(input, tools)
	if err != nil {
		callbacks.OnError(ctx, err)
		return nil, err
	}
	stream := m.client.Messages.NewStreaming(ctx, params)

	sr, sw := schema.Pipe[*schema.Message](32)
	go func() {
		defer sw.Close()
		defer stream.Close()

		// Accumulate tool-call inputs keyed by block index.
		toolCalls := map[int64]*schema.ToolCall{}
		// Accumulate thinking blocks keyed by block index.
		thinkingBlocks := map[int64]*thinkingEntry{}

		for stream.Next() {
			ev := stream.Current()
			switch ev.Type {
			case "content_block_start":
				start := ev.AsContentBlockStart()
				switch start.ContentBlock.Type {
				case "tool_use":
					tb := start.ContentBlock.AsToolUse()
					slog.Debug("anthropic stream tool_use start", "id", tb.ID, "name", tb.Name)
					toolCalls[start.Index] = &schema.ToolCall{
						ID:       tb.ID,
						Type:     "function",
						Function: schema.FunctionCall{Name: tb.Name},
					}
				case "thinking":
					// Start a new thinking entry; text and signature arrive via deltas.
					thinkingBlocks[start.Index] = &thinkingEntry{}
				}

			case "content_block_delta":
				delta := ev.AsContentBlockDelta()
				switch delta.Delta.Type {
				case "text_delta":
					if sw.Send(&schema.Message{Role: schema.Assistant, Content: delta.Delta.Text}, nil) {
						return
					}
				case "input_json_delta":
					if tc, ok := toolCalls[delta.Index]; ok {
						tc.Function.Arguments += delta.Delta.PartialJSON
					}
				case "thinking_delta":
					td := delta.Delta.AsThinkingDelta()
					if te, ok := thinkingBlocks[delta.Index]; ok {
						te.Thinking += td.Thinking
					}
				case "signature_delta":
					sd := delta.Delta.AsSignatureDelta()
					if te, ok := thinkingBlocks[delta.Index]; ok {
						te.Signature = sd.Signature
					}
				}

			default:
			}
		}

		if err := stream.Err(); err != nil {
			sw.Send(nil, fmt.Errorf("anthropic stream: %w", err))
			return
		}

		// Flush accumulated tool calls as a final chunk.
		hasToolCalls := len(toolCalls) > 0
		hasThinking := len(thinkingBlocks) > 0
		if !hasToolCalls && !hasThinking {
			return
		}

		msg := &schema.Message{Role: schema.Assistant}

		// Attach tool calls ordered by block index.
		if hasToolCalls {
			var maxIdx int64
			for i := range toolCalls {
				if i > maxIdx {
					maxIdx = i
				}
			}
			for i := int64(0); i <= maxIdx; i++ {
				if tc, ok := toolCalls[i]; ok {
					msg.ToolCalls = append(msg.ToolCalls, *tc)
				}
			}
		}

		// Attach thinking blocks ordered by block index.
		if hasThinking {
			entries := collectThinkingEntries(thinkingBlocks)
			if len(entries) > 0 {
				msg.ReasoningContent = entries[0].Thinking // primary thinking text
				msg.Extra = setThinkingBlocks(msg.Extra, entries)
			}
		}

		sw.Send(msg, nil)
	}()
	// Tee the stream so registered callback handlers each get a copy.
	_, wrappedSR := callbacks.OnEndWithStreamOutput(ctx, sr)
	return wrappedSR, nil
}

// WithTools returns a new chatModel instance with the given tools bound.
func (m *chatModel) WithTools(tools []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	converted, err := convertToolInfos(tools)
	if err != nil {
		return nil, err
	}
	return &chatModel{
		client:               m.client, // value copy — Client is safe to copy
		model:                m.model,
		maxTokens:            m.maxTokens,
		thinkingBudgetTokens: m.thinkingBudgetTokens,
		tools:                converted,
		toolInfos:            tools, // preserved for callback reporting
	}, nil
}

// buildParamsWithTools converts eino messages into Anthropic MessageNewParams.
func (m *chatModel) buildParamsWithTools(input []*schema.Message, tools []anthropic.ToolUnionParam) (anthropic.MessageNewParams, error) {
	var system []anthropic.TextBlockParam
	var messages []anthropic.MessageParam

	for i := 0; i < len(input); i++ {
		msg := input[i]
		switch msg.Role {
		case schema.System:
			system = append(system, anthropic.TextBlockParam{Text: msg.Content})
		case schema.User:
			messages = append(messages, anthropic.NewUserMessage(anthropic.NewTextBlock(msg.Content)))
		case schema.Assistant:
			blocks, err := assistantBlocks(msg)
			if err != nil {
				return anthropic.MessageNewParams{}, err
			}
			messages = append(messages, anthropic.NewAssistantMessage(blocks...))
		case schema.Tool:
			// Consecutive tool-result messages must be merged into one user message.
			// Anthropic requires all tool_result blocks for a turn to appear together
			// so each has a matching tool_use in the immediately preceding assistant message.
			var blocks []anthropic.ContentBlockParamUnion
			for i < len(input) && input[i].Role == schema.Tool {
				blocks = append(blocks, anthropic.NewToolResultBlock(input[i].ToolCallID, input[i].Content, false))
				i++
			}
			i-- // outer loop will increment past the last tool message
			messages = append(messages, anthropic.NewUserMessage(blocks...))
		}
	}

	maxTokens := int64(defaultMaxTokens)
	if m.maxTokens > 0 {
		maxTokens = int64(m.maxTokens)
	}

	params := anthropic.MessageNewParams{
		Model:     m.model,
		MaxTokens: maxTokens,
		Messages:  messages,
	}
	if len(system) > 0 {
		params.System = system
	}
	if len(tools) > 0 {
		params.Tools = tools
	}

	// Enable Extended Thinking when configured.
	// MaxTokens must exceed the thinking budget so the model has room to reply.
	if m.thinkingBudgetTokens > 0 {
		budget := int64(m.thinkingBudgetTokens)
		if maxTokens <= budget {
			maxTokens = budget + 1024
			params.MaxTokens = maxTokens
		}
		params.Thinking = anthropic.ThinkingConfigParamOfEnabled(budget)
	}

	return params, nil
}

// assistantBlocks converts an eino assistant message to Anthropic content blocks.
// Thinking blocks stored in Extra are round-tripped verbatim (with their
// original signatures) so the API can verify them.
func assistantBlocks(msg *schema.Message) ([]anthropic.ContentBlockParamUnion, error) {
	var blocks []anthropic.ContentBlockParamUnion

	// Re-attach thinking blocks first (must precede text in Anthropic's ordering).
	for _, te := range getThinkingBlocks(msg) {
		blocks = append(blocks, anthropic.NewThinkingBlock(te.Signature, te.Thinking))
	}

	if msg.Content != "" {
		blocks = append(blocks, anthropic.NewTextBlock(msg.Content))
	}
	for _, tc := range msg.ToolCalls {
		var input any = map[string]any{}
		if tc.Function.Arguments != "" {
			var raw json.RawMessage
			if err := json.Unmarshal([]byte(tc.Function.Arguments), &raw); err == nil {
				input = raw
			} else {
				input = tc.Function.Arguments
			}
		}
		blocks = append(blocks, anthropic.NewToolUseBlock(tc.ID, input, tc.Function.Name))
	}
	return blocks, nil
}

// convertResponse translates an Anthropic Message into an eino schema.Message.
// Thinking blocks are stored in ReasoningContent (text) and Extra (full payload
// with signature for round-tripping).
func convertResponse(resp *anthropic.Message) *schema.Message {
	msg := &schema.Message{Role: schema.Assistant}
	var entries []thinkingEntry

	for _, block := range resp.Content {
		switch block.Type {
		case "text":
			msg.Content += block.AsText().Text
		case "tool_use":
			tb := block.AsToolUse()
			msg.ToolCalls = append(msg.ToolCalls, schema.ToolCall{
				ID:   tb.ID,
				Type: "function",
				Function: schema.FunctionCall{
					Name:      tb.Name,
					Arguments: string(tb.Input),
				},
			})
		case "thinking":
			tb := block.AsThinking()
			entries = append(entries, thinkingEntry{
				Thinking:  tb.Thinking,
				Signature: tb.Signature,
			})
		}
	}

	if len(entries) > 0 {
		// Primary ReasoningContent = concatenation of all thinking texts.
		combined := ""
		for _, e := range entries {
			if combined != "" {
				combined += "\n"
			}
			combined += e.Thinking
		}
		msg.ReasoningContent = combined
		msg.Extra = setThinkingBlocks(msg.Extra, entries)
	}

	return msg
}

// convertToolInfos converts eino ToolInfo slice to Anthropic ToolUnionParam slice.
func convertToolInfos(tools []*schema.ToolInfo) ([]anthropic.ToolUnionParam, error) {
	result := make([]anthropic.ToolUnionParam, 0, len(tools))
	for _, t := range tools {
		toolParam := anthropic.ToolParam{
			Name:        t.Name,
			Description: anthropic.String(t.Desc),
		}
		if t.ParamsOneOf != nil {
			js, err := t.ParamsOneOf.ToJSONSchema()
			if err != nil {
				return nil, fmt.Errorf("tool %s: build schema: %w", t.Name, err)
			}
			data, err := json.Marshal(js)
			if err != nil {
				return nil, fmt.Errorf("tool %s: marshal schema: %w", t.Name, err)
			}
			var raw map[string]json.RawMessage
			if err := json.Unmarshal(data, &raw); err == nil {
				if props, ok := raw["properties"]; ok {
					var propsMap any
					_ = json.Unmarshal(props, &propsMap)
					toolParam.InputSchema.Properties = propsMap
				}
				if req, ok := raw["required"]; ok {
					var required []string
					_ = json.Unmarshal(req, &required)
					toolParam.InputSchema.Required = required
				}
			}
		}
		result = append(result, anthropic.ToolUnionParam{OfTool: &toolParam})
	}
	return result, nil
}

// ── helpers for thinking block storage in schema.Message.Extra ───────────────

// setThinkingBlocks stores entries under extraKeyThinkingBlocks in extra,
// allocating the map if necessary, and returns the updated map.
func setThinkingBlocks(extra map[string]any, entries []thinkingEntry) map[string]any {
	if extra == nil {
		extra = make(map[string]any)
	}
	extra[extraKeyThinkingBlocks] = entries
	return extra
}

// getThinkingBlocks retrieves thinking entries stored by setThinkingBlocks.
// Returns nil if the message has no thinking blocks.
func getThinkingBlocks(msg *schema.Message) []thinkingEntry {
	if msg.Extra == nil {
		return nil
	}
	raw, ok := msg.Extra[extraKeyThinkingBlocks]
	if !ok {
		return nil
	}
	switch v := raw.(type) {
	case []thinkingEntry:
		return v
	case []any:
		// Re-hydrate after JSON round-trip: each element is map[string]any.
		entries := make([]thinkingEntry, 0, len(v))
		for _, item := range v {
			if m, ok := item.(map[string]any); ok {
				te := thinkingEntry{}
				if s, ok := m["thinking"].(string); ok {
					te.Thinking = s
				}
				if s, ok := m["signature"].(string); ok {
					te.Signature = s
				}
				entries = append(entries, te)
			}
		}
		return entries
	}
	return nil
}

// collectThinkingEntries returns thinking entries ordered by ascending block index.
func collectThinkingEntries(blocks map[int64]*thinkingEntry) []thinkingEntry {
	if len(blocks) == 0 {
		return nil
	}
	var maxIdx int64
	for i := range blocks {
		if i > maxIdx {
			maxIdx = i
		}
	}
	entries := make([]thinkingEntry, 0, len(blocks))
	for i := int64(0); i <= maxIdx; i++ {
		if te, ok := blocks[i]; ok {
			entries = append(entries, *te)
		}
	}
	return entries
}
