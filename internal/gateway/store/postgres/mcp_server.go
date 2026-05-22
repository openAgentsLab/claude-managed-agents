package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"forge/internal/gateway/store"
)

type mcpServerRepo struct{ db *sql.DB }

func (r *mcpServerRepo) Upsert(_ context.Context, rec *store.MCPServerRecord) error {
	argsJSON, err := json.Marshal(rec.Args)
	if err != nil {
		return wrapErr("marshal mcp args", err)
	}
	envJSON, err := json.Marshal(rec.Env)
	if err != nil {
		return wrapErr("marshal mcp env", err)
	}
	headersJSON, err := json.Marshal(rec.Headers)
	if err != nil {
		return wrapErr("marshal mcp headers", err)
	}
	now := time.Now().Unix()
	_, err = r.db.Exec(`
		INSERT INTO mcp_servers
			(tenant_id, user_id, name, type, command, args_json, env_json, url, headers_json, disabled, created_at, updated_at)
		VALUES ($1, '', $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		ON CONFLICT (tenant_id, user_id, name) DO UPDATE SET
			type         = EXCLUDED.type,
			command      = EXCLUDED.command,
			args_json    = EXCLUDED.args_json,
			env_json     = EXCLUDED.env_json,
			url          = EXCLUDED.url,
			headers_json = EXCLUDED.headers_json,
			disabled     = EXCLUDED.disabled,
			updated_at   = EXCLUDED.updated_at
	`, rec.TenantID, rec.Name, rec.Type,
		rec.Command, string(argsJSON), string(envJSON),
		rec.URL, string(headersJSON), rec.Disabled, now, now,
	)
	return wrapErr("upsert mcp server", err)
}

func (r *mcpServerRepo) Get(_ context.Context, tenantID, name string) (*store.MCPServerRecord, error) {
	var typ, command, url string
	var argsJSON, envJSON, headersJSON []byte
	var disabled bool
	var createdAt, updatedAt int64
	err := r.db.QueryRow(`
		SELECT type, command, args_json, env_json, url, headers_json, disabled, created_at, updated_at
		FROM mcp_servers WHERE tenant_id = $1 AND user_id = '' AND name = $2
	`, tenantID, name).Scan(
		&typ, &command, &argsJSON, &envJSON, &url, &headersJSON, &disabled, &createdAt, &updatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, wrapErr("get mcp server", err)
	}
	return scanMCPServer(tenantID, name, typ, command, argsJSON, envJSON, url, headersJSON, disabled, createdAt, updatedAt)
}

func (r *mcpServerRepo) List(_ context.Context, tenantID string) ([]*store.MCPServerRecord, error) {
	rows, err := r.db.Query(`
		SELECT name, type, command, args_json, env_json, url, headers_json, disabled, created_at, updated_at
		FROM mcp_servers WHERE tenant_id = $1 AND user_id = '' ORDER BY name ASC
	`, tenantID)
	if err != nil {
		return nil, wrapErr("list mcp servers", err)
	}
	defer rows.Close()

	var out []*store.MCPServerRecord
	for rows.Next() {
		var name, typ, command, url string
		var argsJSON, envJSON, headersJSON []byte
		var disabled bool
		var createdAt, updatedAt int64
		if err := rows.Scan(&name, &typ, &command, &argsJSON, &envJSON, &url, &headersJSON, &disabled, &createdAt, &updatedAt); err != nil {
			return nil, wrapErr("scan mcp server row", err)
		}
		rec, err := scanMCPServer(tenantID, name, typ, command, argsJSON, envJSON, url, headersJSON, disabled, createdAt, updatedAt)
		if err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	return out, rows.Err()
}

func (r *mcpServerRepo) Delete(_ context.Context, tenantID, name string) error {
	_, err := r.db.Exec(
		`DELETE FROM mcp_servers WHERE tenant_id = $1 AND user_id = '' AND name = $2`,
		tenantID, name,
	)
	return wrapErr("delete mcp server", err)
}

func scanMCPServer(tenantID, name, typ, command string, argsJSON, envJSON []byte, url string, headersJSON []byte, disabled bool, createdAt, updatedAt int64) (*store.MCPServerRecord, error) {
	var args []string
	if err := json.Unmarshal(argsJSON, &args); err != nil {
		return nil, wrapErr("unmarshal mcp args", err)
	}
	var env map[string]string
	if err := json.Unmarshal(envJSON, &env); err != nil {
		return nil, wrapErr("unmarshal mcp env", err)
	}
	var headers map[string]string
	if err := json.Unmarshal(headersJSON, &headers); err != nil {
		return nil, wrapErr("unmarshal mcp headers", err)
	}
	return &store.MCPServerRecord{
		TenantID:  tenantID,
		Name:      name,
		Type:      typ,
		Command:   command,
		Args:      args,
		Env:       env,
		URL:       url,
		Headers:   headers,
		Disabled:  disabled,
		CreatedAt: time.Unix(createdAt, 0),
		UpdatedAt: time.Unix(updatedAt, 0),
	}, nil
}
