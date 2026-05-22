package store

import (
	"context"
	"time"
)

// AgentRecord is a tenant-scoped Agent stored in the DB.
// MCPServerNames, SkillNames, and CallableAgents are populated only when loading with associations.
type AgentRecord struct {
	ID           string
	TenantID     string
	Name         string
	Description  string
	Version      int
	Model        string          // empty = inherit tenant/global default
	SystemPrompt string
	ToolConfig   map[string]bool // {"bash":true,"read":true,...}
	IsDefault    bool
	ArchivedAt   *time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time

	// Populated by LoadAssociations; empty on plain Get/List calls.
	MCPServerNames []string
	SkillNames     []string
	CallableAgents []string // IDs of other agents this agent may dispatch tasks to
}

// AgentRepository provides CRUD operations for tenant Agents.
type AgentRepository interface {
	// Create inserts a new Agent. r.ID must be pre-set by the caller.
	Create(ctx context.Context, r *AgentRecord) error

	// Update replaces mutable fields (name, description, model, system_prompt,
	// tool_config, sub_agent_types, is_default) and bumps version.
	// Returns the updated record's version.
	Update(ctx context.Context, r *AgentRecord) (int, error)

	// Get returns the agent, or nil if not found or archived.
	Get(ctx context.Context, tenantID, agentID string) (*AgentRecord, error)

	// GetDefault returns the tenant's default agent, or nil if none is set.
	GetDefault(ctx context.Context, tenantID string) (*AgentRecord, error)

	// List returns all active (non-archived) agents for the tenant, ordered by name.
	List(ctx context.Context, tenantID string) ([]*AgentRecord, error)

	// Archive soft-deletes the agent. Returns nil if not found.
	Archive(ctx context.Context, tenantID, agentID string) error

	// SetMCPs replaces all MCP associations for the agent.
	// Pass an empty slice to clear all associations.
	SetMCPs(ctx context.Context, agentID string, mcpNames []string) error

	// SetSkills replaces all Skill associations for the agent.
	// Pass an empty slice to clear all associations.
	SetSkills(ctx context.Context, agentID string, skillNames []string) error

	// SetCallableAgents replaces the set of agent IDs this agent may dispatch tasks to.
	// Pass an empty slice to clear all associations.
	SetCallableAgents(ctx context.Context, agentID string, callableIDs []string) error

	// LoadAssociations populates r.MCPServerNames, r.SkillNames, and r.CallableAgents.
	// It is a cheap 3-query complement to Get/GetDefault so callers can
	// choose whether to pay the extra round-trips.
	LoadAssociations(ctx context.Context, r *AgentRecord) error

	// ClearDefault unsets is_default on any current default agent for the tenant.
	// Called before promoting a new agent to default.
	ClearDefault(ctx context.Context, tenantID string) error
}
