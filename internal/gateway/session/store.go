package session

import (
	"encoding/json"
	"time"

	"github.com/cloudwego/eino/schema"
)

// Event role constants.
const (
	RoleUser           = "user"
	RoleAssistant      = "assistant"
	RoleToolCall       = "tool_call"        // intermediate assistant message requesting a tool
	RoleToolResult     = "tool_result"      // tool execution result
	RoleSystem         = "system"           // session lifecycle events (status_running, status_idle)
	RoleThinking       = "thinking"         // agent.thinking progress hints
	RoleError          = "error"            // session.error
	RoleSpan           = "span"             // span.model_request_end (Content = JSON token counts)
	RoleCustomToolCall = "custom_tool_call" // agent.custom_tool_use (client-executed tool)
	RoleRequiresAction = "requires_action"  // session.requires_action (HITL confirmation gate)
	RoleOutcomeEval    = "outcome_eval"     // agent.outcome_evaluation (grader result; Content = JSON)
)

// SessionStatus represents the lifecycle state of a session.
type SessionStatus string

const (
	SessionIdle         SessionStatus = "idle"
	SessionRunning      SessionStatus = "running"
	SessionRescheduling SessionStatus = "rescheduling"
	SessionTerminated   SessionStatus = "terminated"
	SessionInitializing SessionStatus = "initializing"
	SessionInitFailed   SessionStatus = "init_failed"
)

// CustomToolDef describes a client-executed tool registered at session creation.
type CustomToolDef struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"input_schema,omitempty"`
}

// Session is session metadata.
type Session struct {
	ID              string
	ProjectID       string
	AgentID         string          // agent driving this session; empty = no agent customisation
	Title           string          // short display title; empty until auto-generated or manually set
	MemoryStoreIDs  []string        // custom memory store IDs mounted at creation time
	CustomTools     []CustomToolDef // client-executed tools registered at creation time
	Status          SessionStatus   // lifecycle state; default "idle"
	InitError       string          // non-empty when Status == SessionInitFailed
	ProjectSnapshot string          // JSON snapshot of project config at creation; empty for project-less sessions
	CreatedAt       time.Time
}

// Event is the minimal unit of conversation history, corresponding to the
// append-only event log in the managed-agents architecture.
// Pending=true means the event has not yet been consumed by the Harness.
type Event struct {
	ID         string    // idempotency key: duplicate EmitEvent calls with same ID are no-ops
	Seq        int64     // monotonic sequence number for SSE Last-Event-ID replay (SQLite=rowid, Postgres=seq col, memory=counter)
	Role       string    // see Role* constants
	Content    string
	ToolName   string // non-empty for tool_call / tool_result events
	ToolCallID string // links tool_call ↔ tool_result pairs
	Pending    bool   // true = not yet consumed by Harness
	CreatedAt  time.Time
}

// Snapshot is a cached compacted history. It records the compressed messages
// and how many raw events were included, so that subsequent Prepare calls only
// need to reconstruct the tail (events after EventCount).
type Snapshot struct {
	Messages   []*schema.Message
	EventCount int // events[:EventCount] are represented by Messages
	CreatedAt  time.Time
}

// SessionStore is the core Session abstraction.
type SessionStore interface {
	// CreateSession registers a new session. Only ID, ProjectID, MemoryStoreIDs,
	// and CustomTools are read from sess; other fields are set by the store.
	CreateSession(sess Session) error
	// GetSession returns session metadata and all events.
	GetSession(sessionID string) (*Session, []Event, error)
	// GetEvents returns all events for the session ordered by insertion time.
	GetEvents(sessionID string) ([]Event, error)
	// GetEventsSince returns events with Seq > afterSeq, ordered by Seq ASC.
	// Used for SSE Last-Event-ID replay and history pagination (afterSeq=0 returns all).
	GetEventsSince(sessionID string, afterSeq int64) ([]Event, error)
	// EmitEvent idempotently appends an event. Repeated calls with same ID are no-ops.
	// Returns the monotonic sequence number assigned to the event (0 if a duplicate was ignored).
	EmitEvent(sessionID string, event Event) (int64, error)
	// ListSessions returns all sessions whose internal ID begins with userScope+":".
	// Results are ordered by creation time descending. The Session.ID values
	// returned have the userScope prefix stripped so callers receive the
	// client-visible session ID directly.
	ListSessions(userScope string) ([]Session, error)
	// ListProjectSessions returns sessions belonging to userScope filtered by
	// projectID, applying the filter at the storage layer to avoid loading all
	// sessions into memory and filtering in Go.
	ListProjectSessions(userScope, projectID string) ([]Session, error)
	// UpdateSessionTitle sets the display title for a session.
	UpdateSessionTitle(sessionID, title string) error
	// UpdateCustomTools replaces the custom tool definitions for a session.
	UpdateCustomTools(sessionID string, tools []CustomToolDef) error
	// UpdateSessionStatus transitions the session to the given status.
	UpdateSessionStatus(sessionID string, status SessionStatus) error
	// UpdateSessionInitStatus transitions a session from initializing to idle or
	// init_failed, storing an optional error message.
	UpdateSessionInitStatus(sessionID string, status SessionStatus, initError string) error
	// HasSessionsForProject reports whether any sessions reference the given project ID.
	HasSessionsForProject(projectID string) (bool, error)
	// ClearSession removes all events, the snapshot, and the session record.
	// Used by the session-delete flow; the sandbox and brain are released by the caller.
	ClearSession(sessionID string) error
	// ResetHistory deletes all events and the snapshot for sessionID but keeps the
	// session record intact. Used by the /clear command to start a fresh conversation
	// without destroying the session or its sandbox/brain.
	ResetHistory(sessionID string) error
	// GetSnapshot returns the cached compacted history, or (nil, nil) if none exists.
	GetSnapshot(sessionID string) (*Snapshot, error)
	// SaveSnapshot persists a new compacted history snapshot.
	SaveSnapshot(sessionID string, s *Snapshot) error
}

// ToMessages converts session events into Eino schema.Message slices.
// Tool call events (RoleToolCall) are accumulated and flushed as a single
// assistant message with ToolCalls whenever a tool_result, assistant, or user
// event is encountered — preserving the Anthropic API requirement that each
// tool_use block is immediately followed by its tool_result.
func ToMessages(events []Event) []*schema.Message {
	var msgs []*schema.Message
	var pendingCalls []schema.ToolCall

	flushCalls := func() {
		if len(pendingCalls) == 0 {
			return
		}
		msgs = append(msgs, &schema.Message{
			Role:      schema.Assistant,
			ToolCalls: pendingCalls,
		})
		pendingCalls = nil
	}

	for _, e := range events {
		switch e.Role {
		case RoleSystem:
			// lifecycle events (status_running, status_idle) are not part of conversation history
			continue
		case RoleUser:
			flushCalls()
			msgs = append(msgs, schema.UserMessage(e.Content))
		case RoleToolCall:
			var tc schema.ToolCall
			if err := json.Unmarshal([]byte(e.Content), &tc); err == nil {
				pendingCalls = append(pendingCalls, tc)
			}
		case RoleToolResult:
			flushCalls()
			msgs = append(msgs, &schema.Message{
				Role:       schema.Tool,
				Content:    e.Content,
				ToolCallID: e.ToolCallID,
			})
		case RoleAssistant:
			flushCalls()
			msgs = append(msgs, schema.AssistantMessage(e.Content, nil))
		}
	}
	flushCalls()
	return msgs
}
