// Package history provides the HistoryManager layer that sits between Harness
// and SessionStore.  It loads conversation history, applies context compression
// when needed, and returns a message slice ready to pass to Brain.
package history

import (
	"context"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"forge/internal/compact"
	"forge/internal/gateway/session"
)

// sanitizeToolPairs removes orphaned tool calls and tool results from msgs so
// that every ToolCall in an assistant message has a matching tool result, and
// every tool result has a matching ToolCall. Sending unpaired entries to the
// LLM API causes a 400 error.
func sanitizeToolPairs(msgs []*schema.Message) []*schema.Message {
	// Pass 1: collect ToolCallIDs present in tool-result messages.
	resultIDs := make(map[string]struct{}, len(msgs))
	for _, m := range msgs {
		if m.Role == schema.Tool && m.ToolCallID != "" {
			resultIDs[m.ToolCallID] = struct{}{}
		}
	}

	// Pass 2: for each assistant message, keep only tool calls that have a
	// result; collect the surviving IDs into callIDs.
	callIDs := make(map[string]struct{}, len(msgs))
	for _, m := range msgs {
		if m.Role != schema.Assistant {
			continue
		}
		for _, tc := range m.ToolCalls {
			if _, ok := resultIDs[tc.ID]; ok {
				callIDs[tc.ID] = struct{}{}
			}
		}
	}

	// Pass 3: rebuild the slice, dropping unpaired entries.
	out := make([]*schema.Message, 0, len(msgs))
	for _, m := range msgs {
		switch m.Role {
		case schema.Assistant:
			if len(m.ToolCalls) == 0 {
				out = append(out, m)
				continue
			}
			paired := make([]schema.ToolCall, 0, len(m.ToolCalls))
			for _, tc := range m.ToolCalls {
				if _, ok := callIDs[tc.ID]; ok {
					paired = append(paired, tc)
				}
			}
			// Message had tool calls but all were orphaned and there is no
			// text content — drop the message entirely.
			if len(paired) == 0 && m.Content == "" && len(m.MultiContent) == 0 {
				continue
			}
			if len(paired) == len(m.ToolCalls) {
				out = append(out, m)
			} else {
				cp := *m
				cp.ToolCalls = paired
				out = append(out, &cp)
			}
		case schema.Tool:
			if _, ok := callIDs[m.ToolCallID]; ok {
				out = append(out, m)
			}
		default:
			out = append(out, m)
		}
	}
	return out
}

// CompactionNotifyFn is called just before compaction starts so the caller can
// inform the client that context compression is in progress.
type CompactionNotifyFn func()

type compactionNotifyKey struct{}

// WithCompactionNotify injects fn into ctx. compactingManager.Prepare calls it
// when auto-compaction triggers, before the (potentially slow) LLM compact call.
func WithCompactionNotify(ctx context.Context, fn CompactionNotifyFn) context.Context {
	return context.WithValue(ctx, compactionNotifyKey{}, fn)
}

// Compactor is the compression dependency history needs from compact.Service.
type Compactor interface {
	ShouldAutoCompact(messages []*schema.Message) bool
	Compact(ctx context.Context, messages []*schema.Message) (*compact.Result, error)
}

// BuildWithDefaultCompact creates a Manager backed by compact.Service with default config.
func BuildWithDefaultCompact(store session.SessionStore, m model.BaseChatModel) Manager {
	return NewManager(store, compact.New(compact.DefaultConfig(), m))
}

// Manager loads and (optionally) compresses conversation history for a session.
// Harness calls Prepare before each Brain.Run; it never calls it directly itself.
type Manager interface {
	Prepare(ctx context.Context, sessionID string) ([]*schema.Message, error)
}

// NewManager creates a Manager.
// When svc is nil, a baseManager is returned (no LLM compaction).
// When svc is non-nil, a compactingManager is returned that uses the full pipeline.
func NewManager(store session.SessionStore, svc Compactor) Manager {
	if svc == nil {
		return &baseManager{store: store}
	}
	return &compactingManager{store: store, svc: svc}
}

// ─── base (no compaction) ─────────────────────────────────────────────────────

type baseManager struct {
	store session.SessionStore
}

func (m *baseManager) Prepare(_ context.Context, sessionID string) ([]*schema.Message, error) {
	events, err := m.store.GetEvents(sessionID)
	if err != nil {
		return nil, err
	}
	return sanitizeToolPairs(session.ToMessages(events)), nil
}

// ─── compacting ───────────────────────────────────────────────────────────────

type compactingManager struct {
	store session.SessionStore
	svc   Compactor
}

func (m *compactingManager) Prepare(ctx context.Context, sessionID string) ([]*schema.Message, error) {
	all, err := m.store.GetEvents(sessionID)
	if err != nil {
		return nil, err
	}

	// Load snapshot + tail.
	snapshot, err := m.store.GetSnapshot(sessionID)
	if err != nil {
		return nil, err
	}

	var msgs []*schema.Message
	tailStart := 0
	if snapshot != nil {
		msgs = append(msgs, snapshot.Messages...)
		tailStart = snapshot.EventCount
	}
	if tailStart < len(all) {
		msgs = append(msgs, session.ToMessages(all[tailStart:])...)
	}
	msgs = sanitizeToolPairs(msgs)

	if !m.svc.ShouldAutoCompact(msgs) {
		return msgs, nil
	}

	if fn, ok := ctx.Value(compactionNotifyKey{}).(CompactionNotifyFn); ok && fn != nil {
		fn()
	}

	// Full compaction: MicroCompact + GlobalCompact.
	result, err := m.svc.Compact(ctx, msgs)
	if err != nil {
		// fail-open: return the full (uncompacted) history so the Brain turn can
		// still proceed, albeit with a potentially oversized context window.
		return msgs, nil
	}

	// Persist the snapshot so the next turn can reuse this result.
	// EventCount is intentionally set to len(all) as of when GetEvents was called
	// at the start of Prepare — events appended during the Compact LLM call will
	// appear as tail entries on the next Prepare, which is correct.
	_ = m.store.SaveSnapshot(sessionID, &session.Snapshot{
		Messages:   result.Messages,
		EventCount: len(all),
		CreatedAt:  time.Now(),
	})

	return result.Messages, nil
}
