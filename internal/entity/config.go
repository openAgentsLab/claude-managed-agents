package entity

import "time"

// ── Vault ────────────────────────────────────────────────────────────────────

// VaultListItem is one entry in vault list responses
// (GET /v1/vaults and GET /admin/v1/tenant/vaults).
type VaultListItem struct {
	Name        string    `json:"name"`
	Description string    `json:"description"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// SetVaultRequest is the payload for POST /v1/vaults and
// POST /admin/v1/tenant/vaults.
type SetVaultRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Value       string `json:"value"`
}

// ── MCP servers ──────────────────────────────────────────────────────────────

// MCPServerResponse is the resource body for MCP server list/detail responses.
type MCPServerResponse struct {
	Name      string            `json:"name"`
	Type      string            `json:"type"`
	Command   string            `json:"command,omitempty"`
	Args      []string          `json:"args,omitempty"`
	Env       map[string]string `json:"env,omitempty"`
	URL       string            `json:"url,omitempty"`
	Headers   map[string]string `json:"headers,omitempty"`
	Disabled  bool              `json:"disabled"`
	UpdatedAt time.Time         `json:"updated_at"`
}

// UpsertMCPServerRequest is the payload for POST/PUT MCP server endpoints.
// Type defaults to MCPTypeStdio when omitted.
type UpsertMCPServerRequest struct {
	Name     string            `json:"name"`
	Type     string            `json:"type"`
	Command  string            `json:"command,omitempty"`
	Args     []string          `json:"args,omitempty"`
	Env      map[string]string `json:"env,omitempty"`
	URL      string            `json:"url,omitempty"`
	Headers  map[string]string `json:"headers,omitempty"`
	Disabled bool              `json:"disabled"`
}

// ── Skills ───────────────────────────────────────────────────────────────────

// SkillMetaResponse is one entry in skill list responses (content omitted).
type SkillMetaResponse struct {
	Name      string    `json:"name"`
	UpdatedAt time.Time `json:"updated_at"`
}

// SkillFullResponse includes the full SKILL.md content; returned by detail endpoints.
type SkillFullResponse struct {
	Name      string    `json:"name"`
	Content   string    `json:"content"`
	UpdatedAt time.Time `json:"updated_at"`
}

// UpsertSkillRequest is the payload for POST/PUT skill endpoints.
type UpsertSkillRequest struct {
	Name    string `json:"name"`
	Content string `json:"content"`
}

