package harness

// EventType categorizes structured events sent from Harness to the SSE layer.
// Names follow the Managed Agents spec for direct client consumption.
type EventType string

const (
	// ── Server → Client: agent activity ────────────────────────────────────
	EventAgentThinking      EventType = "agent.thinking"         // reasoning/progress status
	EventAgentMessage       EventType = "agent.message"          // LLM text chunk
	EventAgentToolUse       EventType = "agent.tool_use"         // agent called a built-in tool
	EventAgentToolResult    EventType = "agent.tool_result"      // built-in tool execution result
	EventAgentCustomToolUse EventType = "agent.custom_tool_use" // agent requests client-executed tool
	EventTitle              EventType = "title"                  // auto-generated session title

	// ── Server → Client: session lifecycle ─────────────────────────────────
	EventSessionRunning        EventType = "session.status_running"  // session started processing
	EventSessionIdle           EventType = "session.status_idle"     // session finished, waiting for input
	EventSessionError          EventType = "session.error"           // unrecoverable error
	EventSessionRequiresAction EventType = "session.requires_action" // HITL confirmation required

	// ── Server → Client: observability ─────────────────────────────────────
	EventSpanModelRequestEnd EventType = "span.model_request_end" // per-model-call token usage

	// ── Server → Client: outcome evaluation ────────────────────────────────
	EventAgentOutcomeEvaluation EventType = "agent.outcome_evaluation" // grader result after each agent turn

	// ── Client → Server: session control ───────────────────────────────────
	EventUserInterrupt        EventType = "user.interrupt"          // cancel the running turn
	EventUserCustomToolResult EventType = "user.custom_tool_result" // deliver result to a waiting custom tool
	EventUserToolConfirmation EventType = "user.tool_confirmation"  // HITL confirm / deny
	EventUserDefineOutcome    EventType = "user.define_outcome"     // start an outcome-driven iteration cycle
)

// CriterionFeedback holds the grader's assessment for one rubric criterion.
type CriterionFeedback struct {
	Criterion string `json:"criterion"`
	Status    string `json:"status,omitempty"` // "satisfied" | "needs_work"
	Feedback  string `json:"feedback"`
	Evidence  string `json:"evidence,omitempty"` // verbatim tool-output quote supporting the judgment; empty = unverified
}

// ModelUsage holds token consumption for a single model API call.
type ModelUsage struct {
	InputTokens          int `json:"input_tokens"`
	OutputTokens         int `json:"output_tokens"`
	CacheReadInputTokens int `json:"cache_read_input_tokens"`
}

// Event is one structured unit sent over the Harness output channel.
// The SSE layer marshals it directly to JSON for the client.
type Event struct {
	Type       EventType   `json:"type"`
	Content    string      `json:"content,omitempty"`     // token text, status text, or result summary
	Tool       string      `json:"tool,omitempty"`        // agent.tool_use / agent.custom_tool_use: tool name
	ToolUseID  string      `json:"tool_use_id,omitempty"` // agent.custom_tool_use / session.requires_action: idempotency ID
	ToolInput  string      `json:"tool_input,omitempty"`  // agent.custom_tool_use: JSON-encoded input args
	Description string     `json:"description,omitempty"` // agent.tool_use: human-readable description of what the tool call is doing
	StopReason string      `json:"stop_reason,omitempty"` // session.status_idle: why the run ended
	ModelUsage          *ModelUsage         `json:"model_usage,omitempty"`           // span.model_request_end: token counts
	OutcomeResult       string              `json:"outcome_result,omitempty"`        // agent.outcome_evaluation: satisfied | needs_revision | max_iterations_reached | failed | interrupted
	OutcomeIteration    int                 `json:"outcome_iteration,omitempty"`     // agent.outcome_evaluation: current iteration (1-based)
	OutcomeMaxIter      int                 `json:"outcome_max_iterations,omitempty"` // agent.outcome_evaluation: max allowed
	CriteriaFeedback    []CriterionFeedback `json:"criteria_feedback,omitempty"`     // agent.outcome_evaluation: per-criterion grader feedback
	Seq                 int64               `json:"seq,omitempty"`                   // monotonic seq from store Event.Seq
}
