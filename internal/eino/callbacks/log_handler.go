// Package callbacks provides Eino callback handlers for the agent.
package callbacks

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/components/model"
	einotool "github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

// maxLogBytes is the maximum byte length for a single logged string value.
// Content exceeding this limit is truncated with a suffix showing the omitted byte count.
const maxLogBytes = 4096

// startTimeKey stores the invocation start time in the callback context so
// OnEnd / OnError can compute duration (the context chain flows within the
// same handler instance across timings).
type startTimeKey struct{}

// NewLogHandler builds an Eino callback Handler that logs every model and tool
// invocation at DEBUG level (errors at ERROR level) using the provided logger.
//
// Register it once at startup:
//
//	callbacks.AppendGlobalHandlers(eino_callbacks.NewLogHandler(logger))
//
// What is logged:
//
//	Model OnStart      DEBUG  component name, full messages (role+content+tool_calls), tool definitions
//	Model OnEnd        DEBUG  component name, token usage, full response content
//	Model streaming    DEBUG  component name, accumulated full response from stream
//	Tool  OnStart      DEBUG  tool name, full args JSON
//	Tool  OnEnd        DEBUG  tool name, full output content, duration
//	Any   OnError      ERROR  component name, type, error
func NewLogHandler(logger *slog.Logger) callbacks.Handler {
	return callbacks.NewHandlerBuilder().
		OnStartFn(func(ctx context.Context, info *callbacks.RunInfo, input callbacks.CallbackInput) context.Context {
			ctx = context.WithValue(ctx, startTimeKey{}, time.Now())
			if info == nil {
				return ctx
			}
			switch info.Component {
			case components.ComponentOfChatModel:
				//if mi := model.ConvCallbackInput(input); mi != nil {
				//	msgs := marshalMessages(mi.Messages)
				//	tools := marshalToolDefs(mi.Tools)
				//	logger.DebugContext(ctx, "model call start",
				//		"component", info.Name,
				//		"message_count", len(mi.Messages),
				//		"tool_count", len(mi.Tools),
				//		"messages", msgs,
				//		"tools", tools,
				//	)
				//}
			case components.ComponentOfTool:
				args := ""
				if ti := einotool.ConvCallbackInput(input); ti != nil {
					args = ti.ArgumentsInJSON
				}
				logger.DebugContext(ctx, "tool call start",
					"tool", info.Name,
					"args", args,
				)
			}
			return ctx
		}).
		OnEndFn(func(ctx context.Context, info *callbacks.RunInfo, output callbacks.CallbackOutput) context.Context {
			//dur := elapsed(ctx)
			if info == nil {
				return ctx
			}
			switch info.Component {
			case components.ComponentOfChatModel:
				//if mo := model.ConvCallbackOutput(output); mo != nil {
				//	attrs := []any{
				//		"component", info.Name,
				//		"duration", dur,
				//	}
				//	if mo.TokenUsage != nil {
				//		attrs = append(attrs,
				//			"prompt_tokens", mo.TokenUsage.PromptTokens,
				//			"completion_tokens", mo.TokenUsage.CompletionTokens,
				//			"total_tokens", mo.TokenUsage.TotalTokens,
				//		)
				//	}
				//	if mo.Message != nil {
				//		attrs = append(attrs,
				//			"response_role", string(mo.Message.Role),
				//			"response_content", mo.Message.Content,
				//			"response_tool_calls", marshalToolCalls(mo.Message.ToolCalls),
				//		)
				//	}
				//	logger.DebugContext(ctx, "model call end", attrs...)
				//}
			case components.ComponentOfTool:
				//toolOut := ""
				//if to := einotool.ConvCallbackOutput(output); to != nil {
				//	if to.ToolOutput != nil {
				//		toolOut = marshalJSON(to.ToolOutput)
				//	} else {
				//		toolOut = to.Response
				//	}
				//}
				//logger.DebugContext(ctx, "tool call end",
				//	"tool", info.Name,
				//	"output", toolOut,
				//	"duration", dur,
				//)
			}
			return ctx
		}).
		OnEndWithStreamOutputFn(func(ctx context.Context, info *callbacks.RunInfo, output *schema.StreamReader[callbacks.CallbackOutput]) context.Context {
			if info == nil || info.Component != components.ComponentOfChatModel {
				output.Close()
				return ctx
			}

			dur := elapsed(ctx)
			go func() {
				defer output.Close()
				var chunks []callbacks.CallbackOutput
				for {
					chunk, err := output.Recv()
					if err != nil {
						break
					}
					chunks = append(chunks, chunk)
				}
				// Concatenate all stream chunks into a single message.
				msg, tokenUsage := concatStreamChunks(chunks)
				attrs := []any{
					"component", info.Name,
					"duration_to_first_token", dur,
				}
				if tokenUsage != nil {
					attrs = append(attrs,
						"prompt_tokens", tokenUsage.PromptTokens,
						"completion_tokens", tokenUsage.CompletionTokens,
						"total_tokens", tokenUsage.TotalTokens,
					)
				}
				if msg != nil {
					attrs = append(attrs,
						"response_role", string(msg.Role),
						"response_content", msg.Content,
						"response_tool_calls", marshalToolCalls(msg.ToolCalls),
					)
				}
				logger.DebugContext(ctx, "model streaming end", attrs...)
				if tokenUsage != nil {
					if ch, ok := ctx.Value(SpanNotifKey{}).(chan<- SpanNotif); ok {
						select {
						case ch <- SpanNotif{
							InputTokens:          tokenUsage.PromptTokens,
							OutputTokens:         tokenUsage.CompletionTokens,
							CacheReadInputTokens: tokenUsage.PromptTokenDetails.CachedTokens,
						}:
						default:
						}
					}
				}
			}()
			return ctx
		}).
		OnErrorFn(func(ctx context.Context, info *callbacks.RunInfo, err error) context.Context {
			name, comp := "", ""
			if info != nil {
				name = info.Name
				comp = string(info.Component)
			}
			logger.ErrorContext(ctx, "component error",
				"name", name,
				"component", comp,
				"error", err,
				"duration", elapsed(ctx),
			)
			return ctx
		}).
		Build()
}

func elapsed(ctx context.Context) time.Duration {
	if t, ok := ctx.Value(startTimeKey{}).(time.Time); ok {
		return time.Since(t)
	}
	return 0
}

// marshalMessages serializes a slice of messages to a JSON string for logging.
func marshalMessages(msgs []*schema.Message) string {
	if len(msgs) == 0 {
		return "[]"
	}
	type msgLog struct {
		Role      string           `json:"role"`
		Content   string           `json:"content,omitempty"`
		ToolCalls []schema.ToolCall `json:"tool_calls,omitempty"`
		ToolCallID string          `json:"tool_call_id,omitempty"`
	}
	out := make([]msgLog, 0, len(msgs))
	for _, m := range msgs {
		if m == nil {
			continue
		}
		out = append(out, msgLog{
			Role:       string(m.Role),
			Content:    m.Content,
			ToolCalls:  m.ToolCalls,
			ToolCallID: m.ToolCallID,
		})
	}
	return marshalJSON(out)
}

// marshalToolDefs serializes tool definitions (schemas) to a JSON string for logging.
func marshalToolDefs(tools []*schema.ToolInfo) string {
	if len(tools) == 0 {
		return "[]"
	}
	type toolLog struct {
		Name        string `json:"name"`
		Description string `json:"desc,omitempty"`
	}
	out := make([]toolLog, 0, len(tools))
	for _, t := range tools {
		if t == nil {
			continue
		}
		out = append(out, toolLog{Name: t.Name, Description: t.Desc})
	}
	return marshalJSON(out)
}

// marshalToolCalls serializes tool call data to a JSON string for logging.
func marshalToolCalls(calls []schema.ToolCall) string {
	if len(calls) == 0 {
		return ""
	}
	return marshalJSON(calls)
}

// marshalJSON serializes v to a compact JSON string truncated to maxLogBytes; returns "" on error.
func marshalJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return truncate(string(b), maxLogBytes)
}

// truncate cuts s to max bytes, appending a note with the omitted byte count.
func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	omitted := len(s) - max
	return s[:max] + fmt.Sprintf("…[%d bytes omitted]", omitted)
}

// concatStreamChunks merges all stream callback outputs into a single Message and TokenUsage.
func concatStreamChunks(chunks []callbacks.CallbackOutput) (*schema.Message, *model.TokenUsage) {
	var msg *schema.Message
	var usage *model.TokenUsage
	for _, chunk := range chunks {
		mo := model.ConvCallbackOutput(chunk)
		if mo == nil {
			continue
		}
		if mo.TokenUsage != nil {
			usage = mo.TokenUsage
		}
		if mo.Message == nil {
			continue
		}
		if msg == nil {
			m := *mo.Message
			msg = &m
			continue
		}
		msg.Content += mo.Message.Content
		msg.ToolCalls = append(msg.ToolCalls, mo.Message.ToolCalls...)
	}
	return msg, usage
}
