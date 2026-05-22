package harness

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"github.com/eino-contrib/jsonschema"
	"github.com/google/uuid"

	"forge/internal/gateway/session"
)

// PendingActionsKey is the context key used by HTTPOrchestrator to inject the
// PendingStore so that RemoteToolExecutors can unblock when the client
// returns a result via POST /sessions/:id/events.
type PendingActionsKey struct{}

// SuspendedResult carries the client-supplied result for a custom tool call
// or HITL confirmation.
type SuspendedResult struct {
	Content   string `json:"content,omitempty"`
	Confirmed bool   `json:"confirmed,omitempty"` // for session.requires_action
	Error     string `json:"error,omitempty"`     // non-empty signals a client-side error
}

// PendingStore manages suspended-result delivery for custom tools and HITL
// gates. Implementations must be safe for concurrent use across goroutines.
// The Redis implementation is also safe across multiple nodes.
type PendingStore interface {
	// Register creates a pending slot for actionID and returns a channel that
	// receives exactly one result when Deliver is called. The channel is closed
	// without a value if ctx is cancelled before Deliver fires.
	Register(ctx context.Context, actionID string) <-chan SuspendedResult

	// Deliver sends result to the slot registered under actionID.
	// Returns true if the slot existed and the result was delivered.
	// Returns false if actionID is unknown (expired, never registered, or
	// already delivered).
	Deliver(actionID string, result SuspendedResult) bool
}

// RemoteToolExecutor implements tool.InvokableTool for client-executed custom
// tools. When called by the eino inference loop it:
//  1. Persists an agent.custom_tool_use event to the store.
//  2. Emits EventAgentCustomToolUse to the SSE layer.
//  3. Blocks until the client delivers the result via PendingStore.
//  4. Persists the tool result and returns it to the model.
type RemoteToolExecutor struct {
	def     session.CustomToolDef
	pending PendingStore
	store   session.SessionStore
	outCh   chan<- Event
	sid     string
}

func (t *RemoteToolExecutor) Info(_ context.Context) (*schema.ToolInfo, error) {
	info := &schema.ToolInfo{
		Name: t.def.Name,
		Desc: t.def.Description,
	}
	if len(t.def.InputSchema) > 0 {
		var s jsonschema.Schema
		if err := json.Unmarshal(t.def.InputSchema, &s); err == nil {
			info.ParamsOneOf = schema.NewParamsOneOfByJSONSchema(&s)
		}
	}
	return info, nil
}

func (t *RemoteToolExecutor) InvokableRun(ctx context.Context, argumentsInJSON string, _ ...tool.Option) (string, error) {
	id := uuid.NewString()

	// Persist custom tool call event.
	seq, _ := t.store.EmitEvent(t.sid, session.Event{
		Role:       session.RoleCustomToolCall,
		ToolName:   t.def.Name,
		ToolCallID: id,
		Content:    argumentsInJSON,
	})

	// Notify SSE clients.
	t.outCh <- Event{
		Type:      EventAgentCustomToolUse,
		Tool:      t.def.Name,
		ToolUseID: id,
		ToolInput: argumentsInJSON,
		Seq:       seq,
	}

	// Register a pending slot and wait for the client to deliver a result.
	resultCh := t.pending.Register(ctx, id)

	select {
	case r, ok := <-resultCh:
		if !ok {
			return "", ctx.Err()
		}
		if r.Error != "" {
			return "", errors.New(r.Error)
		}
		// Persist tool result for history replay.
		t.store.EmitEvent(t.sid, session.Event{ //nolint:errcheck
			Role:       session.RoleToolResult,
			ToolName:   t.def.Name,
			ToolCallID: id,
			Content:    r.Content,
		})
		return r.Content, nil
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

// buildRemoteTools constructs RemoteToolExecutor instances for each custom tool
// registered on the session.
func buildRemoteTools(defs []session.CustomToolDef, pending PendingStore, store session.SessionStore, outCh chan<- Event, sid string) []tool.BaseTool {
	ts := make([]tool.BaseTool, 0, len(defs))
	for _, def := range defs {
		d := def // capture
		ts = append(ts, &RemoteToolExecutor{
			def:     d,
			pending: pending,
			store:   store,
			outCh:   outCh,
			sid:     sid,
		})
	}
	return ts
}
