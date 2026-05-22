package postgres

import (
	"context"
	"database/sql"
	"time"

	"forge/internal/gateway/store"
	"forge/internal/gateway/store/encoding"
)

type projectRepo struct {
	db        *sql.DB
	masterKey []byte
}

func (r *projectRepo) Create(_ context.Context, p *store.Project) error {
	rfJSON, err := encoding.MarshalRefFiles(p.RefFiles)
	if err != nil {
		return err
	}
	envJSON, err := encoding.MarshalStringMap(p.Env)
	if err != nil {
		return err
	}
	tokenVal, nonceVal, err := encoding.EncryptToken(r.masterKey, p.TenantID, p.OwnerID, p.GitConfig.Token)
	if err != nil {
		return err
	}
	now := time.Now().Unix()
	_, err = r.db.Exec(
		`INSERT INTO projects
			(id, tenant_id, owner_id, name, description, git_url, git_branch, git_username,
			 git_token, git_token_nonce, environment_id, ref_files_json, env_json, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)`,
		p.ID, p.TenantID, p.OwnerID, p.Name, p.Description,
		p.GitConfig.URL, p.GitConfig.Branch, p.GitConfig.Username,
		tokenVal, nonceVal,
		p.EnvironmentID, rfJSON, envJSON, now, now,
	)
	return wrapErr("create project", err)
}

func (r *projectRepo) Get(_ context.Context, id string) (*store.Project, error) {
	row := r.db.QueryRow(
		`SELECT id, tenant_id, owner_id, name, description, git_url, git_branch, git_username,
		        git_token, git_token_nonce, environment_id, ref_files_json, env_json, created_at, updated_at
		 FROM projects WHERE id = $1`, id,
	)
	return r.scanProject(row)
}

func (r *projectRepo) List(_ context.Context, tenantID, ownerID string) ([]*store.Project, error) {
	rows, err := r.db.Query(
		`SELECT id, tenant_id, owner_id, name, description, git_url, git_branch, git_username,
		        git_token, git_token_nonce, environment_id, ref_files_json, env_json, created_at, updated_at
		 FROM projects
		 WHERE tenant_id = $1 AND owner_id = $2
		 ORDER BY created_at ASC`,
		tenantID, ownerID,
	)
	if err != nil {
		return nil, wrapErr("list projects", err)
	}
	defer rows.Close()

	var out []*store.Project
	for rows.Next() {
		p, err := r.scanProject(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (r *projectRepo) Update(_ context.Context, p *store.Project) error {
	rfJSON, err := encoding.MarshalRefFiles(p.RefFiles)
	if err != nil {
		return err
	}
	envJSON, err := encoding.MarshalStringMap(p.Env)
	if err != nil {
		return err
	}
	tokenVal, nonceVal, err := encoding.EncryptToken(r.masterKey, p.TenantID, p.OwnerID, p.GitConfig.Token)
	if err != nil {
		return err
	}
	_, err = r.db.Exec(
		`UPDATE projects
		 SET name = $1, description = $2, git_url = $3, git_branch = $4, git_username = $5,
		     git_token = $6, git_token_nonce = $7,
		     environment_id = $8, ref_files_json = $9, env_json = $10, updated_at = $11
		 WHERE id = $12`,
		p.Name, p.Description, p.GitConfig.URL, p.GitConfig.Branch, p.GitConfig.Username,
		tokenVal, nonceVal,
		p.EnvironmentID, rfJSON, envJSON, time.Now().Unix(), p.ID,
	)
	return wrapErr("update project", err)
}

func (r *projectRepo) Delete(_ context.Context, id string) error {
	_, err := r.db.Exec(`DELETE FROM projects WHERE id = $1`, id)
	return wrapErr("delete project", err)
}

func (r *projectRepo) scanProject(s rowScanner) (*store.Project, error) {
	var (
		id, tenantID, ownerID, name, description string
		gitURL, gitBranch, gitUsername            string
		gitToken, gitNonce                        string
		envID                                     string
		rfJSON, envJSON                           []byte
		createdAt, updatedAt                      int64
	)
	err := s.Scan(&id, &tenantID, &ownerID, &name, &description,
		&gitURL, &gitBranch, &gitUsername, &gitToken, &gitNonce, &envID, &rfJSON, &envJSON, &createdAt, &updatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, wrapErr("scan project", err)
	}
	plainToken, err := encoding.DecryptToken(r.masterKey, tenantID, ownerID, gitToken, gitNonce)
	if err != nil {
		return nil, err
	}
	refFiles, err := encoding.UnmarshalRefFiles(string(rfJSON))
	if err != nil {
		return nil, err
	}
	env, err := encoding.UnmarshalStringMap(string(envJSON))
	if err != nil {
		return nil, err
	}
	return &store.Project{
		ID:          id,
		TenantID:    tenantID,
		OwnerID:     ownerID,
		Name:        name,
		Description: description,
		GitConfig: store.GitConfig{
			URL:      gitURL,
			Branch:   gitBranch,
			Username: gitUsername,
			Token:    plainToken,
		},
		EnvironmentID: envID,
		RefFiles:      refFiles,
		Env:           env,
		CreatedAt:     time.Unix(createdAt, 0),
		UpdatedAt:     time.Unix(updatedAt, 0),
	}, nil
}
