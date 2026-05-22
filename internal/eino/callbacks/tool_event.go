package callbacks

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/components/model"
	einotool "github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"
)

// SessionIDKey is the context key used by Harness to inject the current session ID
// so that SessionWriterCallback can write tool events to the correct session.
type SessionIDKey struct{}

// ToolNotifKey is the context key Harness uses to inject a tool-notification
// channel. Harness reads notifications and forwards them to the SSE layer.
// Keeping this in the callbacks package avoids a circular import with harness.
type ToolNotifKey struct{}

// SpanNotifKey is the context key Harness uses to inject a span-notification
// channel. The log handler sends one SpanNotif per completed model call so that
// Harness can emit span.model_request_end events and persist token counts.
type SpanNotifKey struct{}

// SpanNotif carries token usage from a single model API call.
type SpanNotif struct {
	InputTokens          int
	OutputTokens         int
	CacheReadInputTokens int
}

// ToolNotifKind distinguishes a tool call from a tool result.
type ToolNotifKind int

const (
	ToolCallKind   ToolNotifKind = iota // LLM requested a tool
	ToolResultKind                      // tool execution completed
)

// ToolNotif carries a single tool lifecycle event from the callback to Harness.
// Harness is responsible for persisting to the session store and emitting to outCh.
type ToolNotif struct {
	Kind    ToolNotifKind
	Name    string
	Summary string // human-readable summary of key args, e.g. "/path/to/file" or "npm install"
	CallID  string // tool_use_id; set for ToolResultKind, links result back to call
	Content string // raw content: JSON-marshaled ToolCall for calls, response string for results
}

// NewToolEventCallback returns an Eino callback handler that forwards tool
// lifecycle events to Harness via the ToolNotifKey channel.
//
// Harness owns store persistence and event-bus publishing; this callback only
// captures the raw data and forwards it so those two writes stay co-located.
//
// It captures two kinds of events:
//   - ChatModel OnEnd with ToolCalls → one ToolCallKind notif per call
//   - Tool OnEnd → one ToolResultKind notif (call ID from compose.GetToolCallID)
//
// The session ID is read from the context value keyed by SessionIDKey.
// The handler is a no-op when the session ID is absent.
func NewToolEventCallback() callbacks.Handler {
	return callbacks.NewHandlerBuilder().
		OnEndFn(func(ctx context.Context, info *callbacks.RunInfo, output callbacks.CallbackOutput) context.Context {
			if info == nil {
				return ctx
			}
			if _, ok := ctx.Value(SessionIDKey{}).(string); !ok {
				return ctx
			}

			switch info.Component {
			case components.ComponentOfChatModel:
				mo := model.ConvCallbackOutput(output)
				if mo == nil || mo.Message == nil || len(mo.Message.ToolCalls) == 0 {
					return ctx
				}
				ch, ok := ctx.Value(ToolNotifKey{}).(chan<- ToolNotif)
				if !ok {
					slog.WarnContext(ctx, "callback: ToolNotifKey not in ctx, tool_use dropped")
					return ctx
				}
				for _, tc := range mo.Message.ToolCalls {
					b, err := json.Marshal(tc)
					if err != nil {
						continue
					}
					slog.InfoContext(ctx, "callback: tool_call notif", "tool", tc.Function.Name)
					ch <- ToolNotif{
						Kind:    ToolCallKind,
						Name:    tc.Function.Name,
						Summary: BuildSummary(tc.Function.Name, tc.Function.Arguments),
						Content: string(b),
					}
				}

			case components.ComponentOfTool:
				to := einotool.ConvCallbackOutput(output)
				if to == nil {
					return ctx
				}
				callID := compose.GetToolCallID(ctx)
				if callID == "" {
					callID = uuid.NewString()
				}
				if ch, ok := ctx.Value(ToolNotifKey{}).(chan<- ToolNotif); ok {
					ch <- ToolNotif{
						Kind:    ToolResultKind,
						Name:    info.Name,
						CallID:  callID,
						Content: to.Response,
					}
				}
			}
			return ctx
		}).
		// ChatModel with streaming fires OnEndWithStreamOutput instead of OnEnd.
		// Drain the stream to assemble the full message and forward tool_call notifs.
		OnEndWithStreamOutputFn(func(ctx context.Context, info *callbacks.RunInfo, output *schema.StreamReader[callbacks.CallbackOutput]) context.Context {
			if info == nil || info.Component != components.ComponentOfChatModel {
				output.Close()
				return ctx
			}
			if _, ok := ctx.Value(SessionIDKey{}).(string); !ok {
				output.Close()
				return ctx
			}
			var chunks []*schema.Message
			for {
				item, err := output.Recv()
				if item != nil {
					if mo := model.ConvCallbackOutput(item); mo != nil && mo.Message != nil {
						chunks = append(chunks, mo.Message)
					}
				}
				if err != nil {
					if err != io.EOF {
						_ = err // stream error — best-effort, don't block the turn
					}
					break
				}
			}
			if len(chunks) == 0 {
				return ctx
			}
			final, err := schema.ConcatMessages(chunks)
			if err != nil || final == nil || len(final.ToolCalls) == 0 {
				return ctx
			}
			ch, ok := ctx.Value(ToolNotifKey{}).(chan<- ToolNotif)
			if !ok {
				return ctx
			}
			for _, tc := range final.ToolCalls {
				b, err := json.Marshal(tc)
				if err != nil {
					continue
				}
				ch <- ToolNotif{
					Kind:    ToolCallKind,
					Name:    tc.Function.Name,
					Summary: BuildSummary(tc.Function.Name, tc.Function.Arguments),
					Content: string(b),
				}
			}
			return ctx
		}).
		Build()
}

// BuildSummary returns a short human-readable summary for a tool call.
// It first looks for a "description" field in the args (written by Claude Code
// style tools), then falls back to formatArgs which surfaces key fields like
// file paths and command strings for common tools.
func BuildSummary(toolName, argsJSON string) string {
	if argsJSON == "" {
		return ""
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal([]byte(argsJSON), &m); err == nil {
		if raw, ok := m["description"]; ok {
			var s string
			if json.Unmarshal(raw, &s) == nil && s != "" {
				return s
			}
		}
	}
	return formatArgs(toolName, argsJSON)
}
