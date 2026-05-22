package entity

// AgentResponse is the detail body for a single Agent.
type AgentResponse struct {
	ID             string          `json:"id"`
	Name           string          `json:"name"`
	Description    string          `json:"description"`
	Version        int             `json:"version"`
	Model          string          `json:"model,omitempty"`
	SystemPrompt   string          `json:"system_prompt,omitempty"`
	ToolConfig     map[string]bool `json:"tool_config,omitempty"`
	MCPServerNames []string        `json:"mcp_server_names,omitempty"`
	SkillNames     []string        `json:"skill_names,omitempty"`
	CallableAgents []string        `json:"callable_agents,omitempty"`
	IsDefault      bool            `json:"is_default"`
	CreatedAt      string          `json:"created_at"`
	UpdatedAt      string          `json:"updated_at"`
}

// CreateAgentRequest is the payload for POST /agents.
type CreateAgentRequest struct {
	Name           string          `json:"name"`
	Description    string          `json:"description,omitempty"`
	Model          string          `json:"model,omitempty"`
	SystemPrompt   string          `json:"system_prompt,omitempty"`
	ToolConfig     map[string]bool `json:"tool_config,omitempty"`
	MCPServerNames []string        `json:"mcp_server_names,omitempty"`
	SkillNames     []string        `json:"skill_names,omitempty"`
	CallableAgents []string        `json:"callable_agents,omitempty"`
	IsDefault      bool            `json:"is_default,omitempty"`
}

// UpdateAgentRequest is the payload for PATCH /agents/:id.
// Only non-zero fields are applied.
type UpdateAgentRequest struct {
	Name           *string         `json:"name,omitempty"`
	Description    *string         `json:"description,omitempty"`
	Model          *string         `json:"model,omitempty"`
	SystemPrompt   *string         `json:"system_prompt,omitempty"`
	ToolConfig     map[string]bool `json:"tool_config,omitempty"`
	MCPServerNames []string        `json:"mcp_server_names,omitempty"`
	SkillNames     []string        `json:"skill_names,omitempty"`
	CallableAgents []string        `json:"callable_agents,omitempty"`
	IsDefault      *bool           `json:"is_default,omitempty"`
}
