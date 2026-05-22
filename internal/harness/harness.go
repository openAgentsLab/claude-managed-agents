// Package harness implements the Harness layer from the managed-agents
// architecture: the effect loop that drives Brain ↔ Session interaction.
//
// Harness reads history from Session, calls Brain.Run(), streams tokens to the
// caller via a channel, and emits the completed assistant reply back to Session.
// Orchestration (REPL / HTTP) only calls harness.Run() — it never touches
// Brain or Session directly.
package harness

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"

	"forge/internal/brain"
	einocallbacks "forge/internal/eino/callbacks"
	"forge/internal/gateway/session"
	"forge/internal/history"
	"forge/internal/memory"
	"forge/internal/reqctx"
)

// Harness drives one reasoning turn: History.Prepare → Brain.Run → stream
// tokens → Session.EmitEvent.
//
// Corresponds to the article's "yield Effect<T> → EffectResult<T>" loop.
type Harness struct {
	brain     *brain.Brain
	store     session.SessionStore
	history   history.Manager
	memStores *memory.SessionStores // nil when memory is disabled
}

// New creates a Harness from the given Brain, SessionStore, and HistoryManager.
func New(b *brain.Brain, s session.SessionStore, mgr history.Manager) *Harness {
	return &Harness{brain: b, store: s, history: mgr}
}

// NewStateless creates a minimal Harness for worker use via RunStateless.
// Brain and history manager are not required; only the session store is used.
func NewStateless(s session.SessionStore) *Harness {
	return &Harness{store: s}
}

// WithMemoryStores returns a shallow copy of the Harness with a session's
// mounted memory stores. Used by serve mode to attach stores at request time.
func (h *Harness) WithMemoryStores(ss *memory.SessionStores) *Harness {
	clone := *h
	clone.memStores = ss
	return &clone
}

// WithBrain returns a shallow copy of the Harness with a session-specific Brain.
// Used by serve mode when a user has custom MCP servers or skills: a Brain with
// the merged tool set is built at session creation and injected here per request.
func (h *Harness) WithBrain(b *brain.Brain) *Harness {
	clone := *h
	clone.brain = b
	return &clone
}

// WithHistory returns a shallow copy of the Harness with a per-session history
// manager. Called alongside WithBrain so compaction uses the tenant's model.
func (h *Harness) WithHistory(mgr history.Manager) *Harness {
	clone := *h
	clone.history = mgr
	return &clone
}

// WithReadOnlyBrain returns a shallow copy of the Harness whose Brain contains
// only read-only tools (registered via tools.RegisterReadOnly). Write tools are
// removed from the LLM's tool definitions so the model never attempts to call them.
// Returns the receiver unchanged when brain is nil or the build fails.
func (h *Harness) WithReadOnlyBrain(ctx context.Context) (*Harness, error) {
	if h.brain == nil {
		return h, nil
	}
	rb, err := h.brain.WithReadOnlyTools(ctx)
	if err != nil {
		return nil, err
	}
	clone := *h
	clone.brain = rb
	return &clone, nil
}

// SessionStore returns the underlying session store. Used by embedded workers
// to share the same connection instead of opening a second one.
func (h *Harness) SessionStore() session.SessionStore {
	return h.store
}

// CreateSession registers a new session.
func (h *Harness) CreateSession(sess session.Session) error {
	return h.store.CreateSession(sess)
}

// GetSessionMeta returns metadata for the session.
func (h *Harness) GetSessionMeta(sessionID string) (*session.Session, error) {
	sess, _, err := h.store.GetSession(sessionID)
	if err != nil {
		return nil, err
	}
	return sess, nil
}

// ClearSession removes all events, the snapshot, and the session record.
// Called by the session-delete flow; the sandbox and brain are released by Orchestration.
func (h *Harness) ClearSession(ctx context.Context, sessionID string) error {
	return h.store.ClearSession(sessionID)
}

// ResetHistory clears all events and the snapshot for sessionID while keeping
// the session record alive. Called by the /clear command flow.
func (h *Harness) ResetHistory(sessionID string) error {
	return h.store.ResetHistory(sessionID)
}

// UpdateSessionTitle sets the display title for a session.
func (h *Harness) UpdateSessionTitle(sessionID, title string) error {
	return h.store.UpdateSessionTitle(sessionID, title)
}

// ListSessions returns all sessions belonging to userScope, with client-visible IDs.
func (h *Harness) ListSessions(userScope string) ([]session.Session, error) {
	return h.store.ListSessions(userScope)
}

// ListProjectSessions returns sessions for userScope filtered by projectID at
// the storage layer, avoiding a full table scan followed by in-memory filtering.
func (h *Harness) ListProjectSessions(userScope, projectID string) ([]session.Session, error) {
	return h.store.ListProjectSessions(userScope, projectID)
}

// GetEvents returns the user and assistant events for the session, ordered by
// insertion time. Tool call / tool result events are excluded — they are
// internal and not meaningful to the UI.
func (h *Harness) GetEvents(sessionID string) ([]session.Event, error) {
	events, err := h.store.GetEvents(sessionID)
	if err != nil {
		return nil, err
	}
	var out []session.Event
	for _, e := range events {
		if e.Role == session.RoleUser || e.Role == session.RoleAssistant {
			out = append(out, e)
		}
	}
	return out, nil
}

// GetEventsSince returns all events for the session with Seq > afterSeq.
// Returns all events when afterSeq=0. Used for SSE replay and history pagination.
func (h *Harness) GetEventsSince(sessionID string, afterSeq int64) ([]session.Event, error) {
	return h.store.GetEventsSince(sessionID, afterSeq)
}

// GetSessionStatus returns the current lifecycle status of the session.
func (h *Harness) GetSessionStatus(sessionID string) (session.SessionStatus, error) {
	sess, _, err := h.store.GetSession(sessionID)
	if err != nil {
		return "", err
	}
	if sess == nil {
		return "", fmt.Errorf("harness: session %q not found", sessionID)
	}
	return sess.Status, nil
}

// UpdateSessionStatus transitions the session to the given status.
// Used by the orchestration layer for interrupt handling and error recovery.
func (h *Harness) UpdateSessionStatus(sessionID string, status session.SessionStatus) error {
	return h.store.UpdateSessionStatus(sessionID, status)
}

// Run executes one reasoning turn for the given session and user text.
// Returns an event channel (structured SSE events to the caller) and an error
// channel (closed when the turn completes, or carrying a fatal error).
//
// Internal flow (corresponds to article Harness effect loop):
//  1. Emit immediate "thinking……" status so the client has an instant ack.
//  2. History.Prepare() — load prior turns; emits "compacting……" event if slow.
//  3. Emit user event to Session
//  4. Inject memory context
//  5. Brain.Run() → AsyncIterator — drive inference; tool events forwarded live.
//  6. Drain iterator: stream text tokens via outCh; detect pause_turn
//  7. Session.EmitEvent() — persist assistant reply
func (h *Harness) Run(ctx context.Context, sessionID, userText string) (<-chan Event, <-chan error) {
	outCh := make(chan Event, 64)
	errCh := make(chan error, 1)

	go func() {
		defer close(outCh)
		defer close(errCh)

		// Emit running-status event. Session status is already set to Running by
		// the orchestration layer (handleRun) before this goroutine starts.
		runSeq, _ := h.store.EmitEvent(sessionID, session.Event{Role: session.RoleSystem, Content: "running"})
		outCh <- Event{Type: EventSessionRunning, Seq: runSeq}

		// Inject sessionID into ctx so SessionWriterCallback can write tool events.
		ctx = context.WithValue(ctx, einocallbacks.SessionIDKey{}, sessionID)

		// Tool-notification forwarding: SessionWriterCallback sends ToolNotif on
		// each tool call/result; a goroutine here converts them to outCh Events.
		// toolNotifCh is closed after the brain loop to drain the forwarding goroutine
		// before outCh itself closes (ensured by wg.Wait in the deferred cleanup).
		toolNotifCh := make(chan einocallbacks.ToolNotif, 32)
		ctx = context.WithValue(ctx, einocallbacks.ToolNotifKey{}, (chan<- einocallbacks.ToolNotif)(toolNotifCh))
		spanNotifCh := make(chan einocallbacks.SpanNotif, 8)
		ctx = context.WithValue(ctx, einocallbacks.SpanNotifKey{}, (chan<- einocallbacks.SpanNotif)(spanNotifCh))
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			for notif := range toolNotifCh {
				switch notif.Kind {
				case einocallbacks.ToolCallKind:
					slog.DebugContext(ctx, "harness: emit tool_use", "tool", notif.Name)
					seq, _ := h.store.EmitEvent(sessionID, session.Event{
						ID:       uuid.NewString(),
						Role:     session.RoleToolCall,
						Content:  notif.Content,
						ToolName: notif.Name,
					})
					label := "### `" + notif.Name + "`"
					if notif.Summary != "" {
						label = "### `" + notif.Name + "(" + notif.Summary + ")`"
					}
					outCh <- Event{Type: EventAgentToolUse, Tool: notif.Name, Description: notif.Summary, Content: label, Seq: seq}
				case einocallbacks.ToolResultKind:
					slog.DebugContext(ctx, "harness: emit tool_result", "tool", notif.Name)
					seq, _ := h.store.EmitEvent(sessionID, session.Event{
						ID:         uuid.NewString(),
						Role:       session.RoleToolResult,
						Content:    notif.Content,
						ToolName:   notif.Name,
						ToolCallID: notif.CallID,
					})
					outCh <- Event{Type: EventAgentToolResult, Tool: notif.Name, ToolUseID: notif.CallID, Seq: seq}
				}
			}
		}()
		go func() {
			defer wg.Done()
			for notif := range spanNotifCh {
				seq, _ := h.store.EmitEvent(sessionID, session.Event{
					Role:    session.RoleSpan,
					Content: marshalSpanContent(notif),
				})
				outCh <- Event{
					Type: EventSpanModelRequestEnd,
					ModelUsage: &ModelUsage{
						InputTokens:          notif.InputTokens,
						OutputTokens:         notif.OutputTokens,
						CacheReadInputTokens: notif.CacheReadInputTokens,
					},
					Seq: seq,
				}
			}
		}()
		// Runs before close(outCh) (LIFO defer order) so all forwarded events
		// are written to outCh before the channel closes.
		// Also emits session.status_idle to mark the turn as complete for SSE subscribers.
		defer func() {
			close(toolNotifCh)
			close(spanNotifCh)
			wg.Wait()
			stopReason := "end_turn"
			if ctx.Err() != nil {
				stopReason = "interrupted"
			}
			_ = h.store.UpdateSessionStatus(sessionID, session.SessionIdle)
			idleSeq, _ := h.store.EmitEvent(sessionID, session.Event{Role: session.RoleSystem, Content: "idle:" + stopReason})
			outCh <- Event{Type: EventSessionIdle, StopReason: stopReason, Seq: idleSeq}
		}()

		// Build a per-run brain variant and inject HITL gate when PendingStore is present.
		runBrain, ctx := h.prepareRun(ctx, sessionID, outCh)

		// Step 2: load prior history BEFORE emitting the current user message so
		// that the message slice passed to Brain contains only previous turns.
		// Emitting first and then loading would include the current user message in
		// history AND in Brain.Run's input.Messages, sending it to the LLM twice.
		histCtx := history.WithCompactionNotify(ctx, func() {
			compactSeq, _ := h.store.EmitEvent(sessionID, session.Event{Role: session.RoleThinking, Content: "compacting……"})
			outCh <- Event{Type: EventAgentThinking, Content: "compacting……", Seq: compactSeq}
		})
		msgs, err := h.history.Prepare(histCtx, sessionID)
		if err != nil {
			errCh <- fmt.Errorf("harness: prepare history: %w", err)
			return
		}

		// Step 3: persist user message after history is prepared (idempotent).
		_, _ = h.store.EmitEvent(sessionID, session.Event{Role: session.RoleUser, Content: userText})

		// Step 4: inject memory stores and system-prompt section for Brain.
		if h.memStores != nil {
			ctx = memory.WithSessionStores(ctx, h.memStores)
			if sysCtx := h.memStores.BuildSystemContext(); sysCtx != "" {
				ctx = memory.WithSystemContext(ctx, sysCtx)
			}
		}

		// Step 5: drive Brain inference
		// Eino ADK's Runner handles the multi-turn tool_use → model loop internally.
		// pause_turn detection is handled in the loop below.
		iter := runBrain.Run(ctx, userText, msgs)
		var reply string
		var eventCount int
		for {
			ev, ok := iter.Next()
			if !ok {
				break
			}
			eventCount++
			if ev == nil || ev.Err != nil {
				slog.DebugContext(ctx, "harness: skip event", "count", eventCount, "nil", ev == nil, "err", func() error {
					if ev != nil {
						return ev.Err
					}
					return nil
				}())
				continue
			}
			if ev.Output == nil || ev.Output.MessageOutput == nil {
				slog.DebugContext(ctx, "harness: agent event", "count", eventCount, "content_len", 0)
				continue
			}
			// Tool-result events carry the raw tool output — skip them so only
			// the assistant's text response is forwarded to the client.
			if ev.Output.MessageOutput.Role == schema.Tool {
				continue
			}
			c, msg := drainMessageVariant(ev.Output.MessageOutput, outCh)
			slog.DebugContext(ctx, "harness: agent event", "count", eventCount, "content_len", len(c))
			// Detect pause_turn: server-side agentic loop iteration limit reached.
			// pause_turn means "continue inference", not "done".
			// Docs: Handling stop reasons (build-with-claude/handling-stop-reasons)
			if isPauseTurn(msg) {
				// Eino ADK v0.8.5 handles pause_turn internally; if it reaches
				// Harness, log and continue draining.
				continue
			}
			reply += c
		}
		slog.DebugContext(ctx, "harness: brain run complete", "events", eventCount, "reply_len", len(reply))

		// Step 7: persist assistant reply to Session
		if reply != "" {
			_, _ = h.store.EmitEvent(sessionID, session.Event{Role: session.RoleAssistant, Content: reply})
		}

		// Auto-generate a title on the first turn (msgs was empty before this run).
		if len(msgs) == 0 && runBrain != nil {
			if title := runBrain.GenerateTitle(ctx, userText); title != "" {
				_ = h.store.UpdateSessionTitle(sessionID, title)
				outCh <- Event{Type: EventTitle, Content: title}
			}
		}

	}()

	return outCh, errCh
}

// prepareRun builds the per-turn brain variant and injects the HITL gate into ctx.
// When PendingStore is absent from ctx both are returned unchanged.
func (h *Harness) prepareRun(ctx context.Context, sessionID string, outCh chan<- Event) (*brain.Brain, context.Context) {
	runBrain := h.brain
	ps, ok := ctx.Value(PendingActionsKey{}).(PendingStore)
	if !ok {
		return runBrain, ctx
	}

	if sess, _, err := h.store.GetSession(sessionID); err == nil && len(sess.CustomTools) > 0 {
		remoteTools := buildRemoteTools(sess.CustomTools, ps, h.store, outCh, sessionID)
		if variant, err := h.brain.WithExtraTools(ctx, remoteTools); err == nil {
			runBrain = variant
		} else {
			slog.WarnContext(ctx, "harness: failed to build brain with custom tools", "error", err)
		}
	}

	ctx = reqctx.WithHITLGate(ctx, func(gateCtx context.Context, toolName, argsJSON string) bool {
		id := uuid.NewString()
		seq, _ := h.store.EmitEvent(sessionID, session.Event{
			Role:       session.RoleRequiresAction,
			ToolName:   toolName,
			ToolCallID: id,
			Content:    argsJSON,
		})
		outCh <- Event{
			Type:      EventSessionRequiresAction,
			Tool:      toolName,
			ToolUseID: id,
			Content:   argsJSON,
			Seq:       seq,
		}
		resultCh := ps.Register(gateCtx, id)
		select {
		case r, ok := <-resultCh:
			if !ok {
				return false
			}
			return r.Confirmed
		case <-gateCtx.Done():
			return false
		}
	})

	return runBrain, ctx
}

// RunStateless executes one reasoning turn using the provided sub-agent brain
// instead of h.brain. It skips history preparation and memory — sub-agents are
// stateless; their full task context is in the prompt itself.
//
// Notification channels: RunStateless creates *fresh* ToolNotifKey and SpanNotifKey
// channels in ctx, deliberately replacing any channels inherited from the parent
// harness.Run call. This prevents sub-agent tool events from leaking into the
// parent session's SSE stream. Sub-agent tool events are persisted directly to
// the sub-agent's session store instead.
func (h *Harness) RunStateless(ctx context.Context, sessionID string, b *brain.Brain, prompt string) (<-chan string, <-chan error) {
	outCh := make(chan string, 64)
	errCh := make(chan error, 1)

	go func() {
		defer close(outCh)
		defer close(errCh)

		// Override SessionID and inject fresh notification channels so sub-agent
		// callbacks are fully isolated from the parent session's channels.
		ctx = context.WithValue(ctx, einocallbacks.SessionIDKey{}, sessionID)
		toolNotifCh := make(chan einocallbacks.ToolNotif, 32)
		ctx = context.WithValue(ctx, einocallbacks.ToolNotifKey{}, (chan<- einocallbacks.ToolNotif)(toolNotifCh))
		spanNotifCh := make(chan einocallbacks.SpanNotif, 8)
		ctx = context.WithValue(ctx, einocallbacks.SpanNotifKey{}, (chan<- einocallbacks.SpanNotif)(spanNotifCh))

		var wg sync.WaitGroup
		wg.Add(2)
		// Persist sub-agent tool events to its own session.
		go func() {
			defer wg.Done()
			for notif := range toolNotifCh {
				switch notif.Kind {
				case einocallbacks.ToolCallKind:
					_, _ = h.store.EmitEvent(sessionID, session.Event{
						ID:       uuid.NewString(),
						Role:     session.RoleToolCall,
						Content:  notif.Content,
						ToolName: notif.Name,
					})
				case einocallbacks.ToolResultKind:
					_, _ = h.store.EmitEvent(sessionID, session.Event{
						ID:         uuid.NewString(),
						Role:       session.RoleToolResult,
						Content:    notif.Content,
						ToolName:   notif.Name,
						ToolCallID: notif.CallID,
					})
				}
			}
		}()
		// Drain span notifications without forwarding — sub-agent token usage is
		// not attributed to the parent turn's span.
		go func() {
			defer wg.Done()
			for range spanNotifCh {
			}
		}()
		defer func() {
			close(toolNotifCh)
			close(spanNotifCh)
			wg.Wait()
		}()

		_, _ = h.store.EmitEvent(sessionID, session.Event{Role: session.RoleUser, Content: prompt})

		// Stateless: no history — sub-agent context is fully in the prompt.
		// brain.Brain.Run is goroutine-safe: each call creates an independent
		// Eino ADK execution context; the Brain struct itself holds no mutable
		// per-request state.
		iter := b.Run(ctx, prompt, nil)
		var reply string
		for {
			ev, ok := iter.Next()
			if !ok {
				break
			}
			if ev == nil || ev.Err != nil {
				continue
			}
			if ev.Output == nil || ev.Output.MessageOutput == nil {
				continue
			}
			if ev.Output.MessageOutput.Role == schema.Tool {
				continue
			}
			// RunStateless is worker-only; collect full content without per-chunk delivery.
			c, msg := drainMessageVariant(ev.Output.MessageOutput, nil)
			if isPauseTurn(msg) {
				continue
			}
			if c != "" {
				outCh <- c
			}
			reply += c
		}

		if reply != "" {
			_, _ = h.store.EmitEvent(sessionID, session.Event{Role: session.RoleAssistant, Content: reply})
		}
	}()

	return outCh, errCh
}

// drainMessageVariant drains a MessageVariant and returns its full text content
// plus a representative message for metadata checks (e.g. pause_turn).
// When outCh is non-nil each text chunk is forwarded immediately, giving the
// SSE layer true token-by-token streaming. Pass nil to collect without streaming
// (used by RunStateless).
func drainMessageVariant(mo *adk.MessageVariant, outCh chan<- Event) (content string, msg *schema.Message) {
	if mo.IsStreaming {
		var sb strings.Builder
		for {
			chunk, err := mo.MessageStream.Recv()
			if chunk != nil {
				if chunk.Content != "" {
					if outCh != nil {
						outCh <- Event{Type: EventAgentMessage, Content: chunk.Content}
					}
					sb.WriteString(chunk.Content)
				}
				msg = chunk
			}
			if err != nil {
				if err != io.EOF {
					slog.Debug("harness: stream recv error", "err", err)
				}
				break
			}
		}
		content = sb.String()
		return
	}
	if mo.Message != nil {
		content = mo.Message.Content
		msg = mo.Message
		if content != "" && outCh != nil {
			outCh <- Event{Type: EventAgentMessage, Content: content}
		}
	}
	return
}

func isPauseTurn(msg *schema.Message) bool {
	return msg != nil && msg.ResponseMeta != nil && msg.ResponseMeta.FinishReason == "pause_turn"
}

// marshalSpanContent serialises a SpanNotif to JSON for storage in the events table.
func marshalSpanContent(n einocallbacks.SpanNotif) string {
	b, _ := json.Marshal(struct {
		InputTokens          int `json:"input_tokens"`
		OutputTokens         int `json:"output_tokens"`
		CacheReadInputTokens int `json:"cache_read_input_tokens"`
	}{n.InputTokens, n.OutputTokens, n.CacheReadInputTokens})
	return string(b)
}
