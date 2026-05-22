package entity

import "encoding/json"

// CustomToolDef describes a client-executed tool passed at session creation.
type CustomToolDef struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"input_schema,omitempty"`
}

// SessionListItem is one entry in the list returned by GET /v1/sessions.
type SessionListItem struct {
	SessionID string `json:"session_id"`
	ProjectID string `json:"project_id,omitempty"`
	Title     string `json:"title,omitempty"`
	Status    string `json:"status"`
	InitError string `json:"init_error,omitempty"`
	CreatedAt string `json:"created_at"`
}

// SessionResponse is the detail body returned by GET /v1/sessions/:id.
type SessionResponse struct {
	SessionID string `json:"session_id"`
	ProjectID string `json:"project_id,omitempty"`
	Title     string `json:"title,omitempty"`
	Status    string `json:"status"`
	InitError string `json:"init_error,omitempty"`
	CreatedAt string `json:"created_at"`
}

// CreateSessionRequest is the payload for POST /v1/sessions.
type CreateSessionRequest struct {
	SessionID    string          `json:"session_id,omitempty"`
	ProjectID    string          `json:"project_id,omitempty"`
	AgentID      string          `json:"agent_id,omitempty"`
	MemoryStores []string        `json:"memory_stores,omitempty"`
	CustomTools  []CustomToolDef `json:"custom_tools,omitempty"`
}

// CreateSessionResponse is the response body for a successful POST /v1/sessions.
type CreateSessionResponse struct {
	SessionID     string `json:"session_id"`
	ProjectID     string `json:"project_id"`
	Status        string `json:"status"`
	ProjectName   string `json:"project_name,omitempty"`
	EnvironmentID string `json:"environment_id,omitempty"`
}

// RunRequest is the payload for POST /v1/sessions/:id/run.
type RunRequest struct {
	Message string `json:"message"`
	// Mode overrides the permission posture for this run: PermissionModeDefault or
	// PermissionModePlan. Omit to inherit the tenant default. Viewer role always
	// uses plan regardless of this field.
	Mode string `json:"mode,omitempty"`
}

// RunResponse is the body returned with 202 Accepted from POST /v1/sessions/:id/run.
type RunResponse struct {
	Status string `json:"status"`
}

// UpdateSessionTitleRequest is the payload for PATCH /v1/sessions/:id.
type UpdateSessionTitleRequest struct {
	Title string `json:"title"`
}

// SendEventRequest is the payload for POST /v1/sessions/:id/events.
// The Type field selects the event variant; other fields are type-specific.
type SendEventRequest struct {
	Type string `json:"type"`
	// user.custom_tool_result / user.tool_confirmation
	ToolUseID string `json:"tool_use_id,omitempty"`
	// user.custom_tool_result
	Content string `json:"content,omitempty"`
	Error   string `json:"error,omitempty"`
	// user.tool_confirmation
	Confirmed bool `json:"confirmed,omitempty"`
	// user.define_outcome
	Description   string `json:"description,omitempty"`
	Rubric        string `json:"rubric,omitempty"`
	MaxIterations int    `json:"max_iterations,omitempty"`
}

// EventContentResponse is the body returned by
// GET /v1/sessions/:id/events/:seq/content.
type EventContentResponse struct {
	Seq     int64  `json:"seq"`
	Content string `json:"content"`
}

// ErrorResponse is the standard JSON error body used for 4xx/5xx responses
// that carry a machine-readable message.
type ErrorResponse struct {
	Error string `json:"error"`
}
