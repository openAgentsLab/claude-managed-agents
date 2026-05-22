package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"forge/internal/gateway/store"
)

type agentRepo struct{ db *sql.DB }

func (r *agentRepo) Create(_ context.Context, rec *store.AgentRecord) error {
	toolJSON, err := json.Marshal(rec.ToolConfig)
	if err != nil {
		return wrapErr("marshal tool_config", err)
	}
	now := time.Now().Unix()
	_, err = r.db.Exec(`
		INSERT INTO agents
			(id, tenant_id, name, description, version, model, system_prompt,
			 tool_config_json, is_default, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 1, $5, $6, $7, $8, $9, $10)
	`, rec.ID, rec.TenantID, rec.Name, rec.Description,
		rec.Model, rec.SystemPrompt, string(toolJSON),
		rec.IsDefault, now, now,
	)
	return wrapErr("create agent", err)
}

func (r *agentRepo) Update(_ context.Context, rec *store.AgentRecord) (int, error) {
	toolJSON, err := json.Marshal(rec.ToolConfig)
	if err != nil {
		return 0, wrapErr("marshal tool_config", err)
	}
	now := time.Now().Unix()
	var newVersion int
	err = r.db.QueryRow(`
		UPDATE agents
		SET name             = $1,
		    description      = $2,
		    model            = $3,
		    system_prompt    = $4,
		    tool_config_json = $5,
		    is_default       = $6,
		    version          = version + 1,
		    updated_at       = $7
		WHERE id = $8 AND tenant_id = $9 AND archived_at IS NULL
		RETURNING version
	`, rec.Name, rec.Description, rec.Model, rec.SystemPrompt,
		string(toolJSON), rec.IsDefault, now,
		rec.ID, rec.TenantID,
	).Scan(&newVersion)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	return newVersion, wrapErr("update agent", err)
}

func (r *agentRepo) Get(_ context.Context, tenantID, agentID string) (*store.AgentRecord, error) {
	row := r.db.QueryRow(`
		SELECT id, name, description, version, model, system_prompt,
		       tool_config_json, is_default, created_at, updated_at
		FROM agents
		WHERE tenant_id = $1 AND id = $2 AND archived_at IS NULL
	`, tenantID, agentID)
	return scanAgentPg(tenantID, row)
}

func (r *agentRepo) GetDefault(_ context.Context, tenantID string) (*store.AgentRecord, error) {
	row := r.db.QueryRow(`
		SELECT id, name, description, version, model, system_prompt,
		       tool_config_json, is_default, created_at, updated_at
		FROM agents
		WHERE tenant_id = $1 AND is_default = TRUE AND archived_at IS NULL
	`, tenantID)
	return scanAgentPg(tenantID, row)
}

func (r *agentRepo) List(_ context.Context, tenantID string) ([]*store.AgentRecord, error) {
	rows, err := r.db.Query(`
		SELECT id, name, description, version, model, system_prompt,
		       tool_config_json, is_default, created_at, updated_at
		FROM agents
		WHERE tenant_id = $1 AND archived_at IS NULL
		ORDER BY name ASC
	`, tenantID)
	if err != nil {
		return nil, wrapErr("list agents", err)
	}
	defer rows.Close()

	var out []*store.AgentRecord
	for rows.Next() {
		var id, name, description, model, systemPrompt string
		var toolJSON []byte
		var version int
		var isDefault bool
		var createdAt, updatedAt int64
		if err := rows.Scan(&id, &name, &description, &version, &model, &systemPrompt,
			&toolJSON, &isDefault, &createdAt, &updatedAt); err != nil {
			return nil, wrapErr("scan agent row", err)
		}
		rec, err := buildAgentRecordPg(tenantID, id, name, description, version, model, systemPrompt,
			toolJSON, isDefault, createdAt, updatedAt)
		if err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	return out, rows.Err()
}

func (r *agentRepo) Archive(_ context.Context, tenantID, agentID string) error {
	now := time.Now().Unix()
	_, err := r.db.Exec(
		`UPDATE agents SET archived_at = $1 WHERE id = $2 AND tenant_id = $3`,
		now, agentID, tenantID,
	)
	return wrapErr("archive agent", err)
}

func (r *agentRepo) SetCallableAgents(_ context.Context, agentID string, callableIDs []string) error {
	tx, err := r.db.Begin()
	if err != nil {
		return wrapErr("begin SetCallableAgents tx", err)
	}
	defer tx.Rollback() //nolint:errcheck
	if _, err := tx.Exec(`DELETE FROM agent_callable_agents WHERE agent_id = $1`, agentID); err != nil {
		return wrapErr("delete agent_callable_agents", err)
	}
	for _, cid := range callableIDs {
		if _, err := tx.Exec(
			`INSERT INTO agent_callable_agents (agent_id, callable_id) VALUES ($1, $2)`,
			agentID, cid,
		); err != nil {
			return wrapErr("insert agent_callable_agent", err)
		}
	}
	return wrapErr("commit SetCallableAgents", tx.Commit())
}

func (r *agentRepo) SetMCPs(_ context.Context, agentID string, mcpNames []string) error {
	tx, err := r.db.Begin()
	if err != nil {
		return wrapErr("begin SetMCPs tx", err)
	}
	defer tx.Rollback() //nolint:errcheck
	if _, err := tx.Exec(`DELETE FROM agent_mcps WHERE agent_id = $1`, agentID); err != nil {
		return wrapErr("delete agent_mcps", err)
	}
	for _, name := range mcpNames {
		if _, err := tx.Exec(
			`INSERT INTO agent_mcps (agent_id, mcp_name) VALUES ($1, $2)`,
			agentID, name,
		); err != nil {
			return wrapErr("insert agent_mcp", err)
		}
	}
	return wrapErr("commit SetMCPs", tx.Commit())
}

func (r *agentRepo) SetSkills(_ context.Context, agentID string, skillNames []string) error {
	tx, err := r.db.Begin()
	if err != nil {
		return wrapErr("begin SetSkills tx", err)
	}
	defer tx.Rollback() //nolint:errcheck
	if _, err := tx.Exec(`DELETE FROM agent_skills WHERE agent_id = $1`, agentID); err != nil {
		return wrapErr("delete agent_skills", err)
	}
	for _, name := range skillNames {
		if _, err := tx.Exec(
			`INSERT INTO agent_skills (agent_id, skill_name) VALUES ($1, $2)`,
			agentID, name,
		); err != nil {
			return wrapErr("insert agent_skill", err)
		}
	}
	return wrapErr("commit SetSkills", tx.Commit())
}

func (r *agentRepo) ClearDefault(_ context.Context, tenantID string) error {
	_, err := r.db.Exec(
		`UPDATE agents SET is_default = FALSE WHERE tenant_id = $1 AND is_default = TRUE AND archived_at IS NULL`,
		tenantID,
	)
	return wrapErr("clear default agent", err)
}

func (r *agentRepo) LoadAssociations(_ context.Context, rec *store.AgentRecord) error {
	mcpRows, err := r.db.Query(
		`SELECT mcp_name FROM agent_mcps WHERE agent_id = $1 ORDER BY mcp_name ASC`,
		rec.ID,
	)
	if err != nil {
		return wrapErr("list agent_mcps", err)
	}
	defer mcpRows.Close()
	var mcpNames []string
	for mcpRows.Next() {
		var n string
		if err := mcpRows.Scan(&n); err != nil {
			return wrapErr("scan agent_mcp", err)
		}
		mcpNames = append(mcpNames, n)
	}
	if err := mcpRows.Err(); err != nil {
		return wrapErr("agent_mcps rows", err)
	}
	rec.MCPServerNames = mcpNames

	skillRows, err := r.db.Query(
		`SELECT skill_name FROM agent_skills WHERE agent_id = $1 ORDER BY skill_name ASC`,
		rec.ID,
	)
	if err != nil {
		return wrapErr("list agent_skills", err)
	}
	defer skillRows.Close()
	var skillNames []string
	for skillRows.Next() {
		var n string
		if err := skillRows.Scan(&n); err != nil {
			return wrapErr("scan agent_skill", err)
		}
		skillNames = append(skillNames, n)
	}
	if err := skillRows.Err(); err != nil {
		return wrapErr("agent_skills rows", err)
	}
	rec.SkillNames = skillNames

	callRows, err := r.db.Query(
		`SELECT callable_id FROM agent_callable_agents WHERE agent_id = $1 ORDER BY callable_id ASC`,
		rec.ID,
	)
	if err != nil {
		return wrapErr("list agent_callable_agents", err)
	}
	defer callRows.Close()
	var callableIDs []string
	for callRows.Next() {
		var cid string
		if err := callRows.Scan(&cid); err != nil {
			return wrapErr("scan agent_callable_agent", err)
		}
		callableIDs = append(callableIDs, cid)
	}
	if err := callRows.Err(); err != nil {
		return wrapErr("agent_callable_agents rows", err)
	}
	rec.CallableAgents = callableIDs
	return nil
}

// ── helpers ───────────────────────────────────────────────────────────────────

func scanAgentPg(tenantID string, row *sql.Row) (*store.AgentRecord, error) {
	var id, name, description, model, systemPrompt string
	var toolJSON []byte
	var version int
	var isDefault bool
	var createdAt, updatedAt int64
	err := row.Scan(&id, &name, &description, &version, &model, &systemPrompt,
		&toolJSON, &isDefault, &createdAt, &updatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, wrapErr("scan agent", err)
	}
	return buildAgentRecordPg(tenantID, id, name, description, version, model, systemPrompt,
		toolJSON, isDefault, createdAt, updatedAt)
}

func buildAgentRecordPg(
	tenantID, id, name, description string,
	version int,
	model, systemPrompt string,
	toolJSON []byte,
	isDefault bool,
	createdAt, updatedAt int64,
) (*store.AgentRecord, error) {
	var toolConfig map[string]bool
	if err := json.Unmarshal(toolJSON, &toolConfig); err != nil {
		toolConfig = map[string]bool{}
	}
	return &store.AgentRecord{
		ID:           id,
		TenantID:     tenantID,
		Name:         name,
		Description:  description,
		Version:      version,
		Model:        model,
		SystemPrompt: systemPrompt,
		ToolConfig:   toolConfig,
		IsDefault:    isDefault,
		CreatedAt:    time.Unix(createdAt, 0),
		UpdatedAt:    time.Unix(updatedAt, 0),
	}, nil
}

func migrateAgentsV2(tx *sql.Tx) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS agents (
			id               TEXT    PRIMARY KEY,
			tenant_id        TEXT    NOT NULL,
			name             TEXT    NOT NULL,
			description      TEXT    NOT NULL DEFAULT '',
			version          INTEGER NOT NULL DEFAULT 1,
			model            TEXT    NOT NULL DEFAULT '',
			system_prompt    TEXT    NOT NULL DEFAULT '',
			tool_config_json JSONB   NOT NULL DEFAULT '{}',
			is_default       BOOLEAN NOT NULL DEFAULT FALSE,
			created_at       BIGINT  NOT NULL,
			updated_at       BIGINT  NOT NULL,
			archived_at      BIGINT
		)`,
		`CREATE INDEX IF NOT EXISTS idx_agents_tenant
			ON agents(tenant_id) WHERE archived_at IS NULL`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_agents_tenant_name
			ON agents(tenant_id, name) WHERE archived_at IS NULL`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_agents_default
			ON agents(tenant_id) WHERE is_default = TRUE AND archived_at IS NULL`,
		`CREATE TABLE IF NOT EXISTS agent_mcps (
			agent_id  TEXT NOT NULL,
			mcp_name  TEXT NOT NULL,
			PRIMARY KEY (agent_id, mcp_name)
		)`,
		`CREATE TABLE IF NOT EXISTS agent_skills (
			agent_id   TEXT NOT NULL,
			skill_name TEXT NOT NULL,
			PRIMARY KEY (agent_id, skill_name)
		)`,
		`CREATE TABLE IF NOT EXISTS agent_callable_agents (
			agent_id    TEXT NOT NULL,
			callable_id TEXT NOT NULL,
			PRIMARY KEY (agent_id, callable_id)
		)`,
	}
	for _, s := range stmts {
		if _, err := tx.Exec(s); err != nil {
			return err
		}
	}
	return nil
}
