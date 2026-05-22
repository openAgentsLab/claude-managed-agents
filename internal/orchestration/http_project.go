package orchestration

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"path"
	"strings"

	"github.com/gin-gonic/gin"

	"forge/internal/entity"
	appstore "forge/internal/gateway/store"
	"forge/internal/gateway/vault"
	"forge/internal/hands"
	"forge/internal/resources"
)

// ── mapping helpers ───────────────────────────────────────────────────────────

func projectToResponse(p *appstore.Project) *entity.ProjectResponse {
	return &entity.ProjectResponse{
		ID:          p.ID,
		Name:        p.Name,
		Description: p.Description,
		GitConfig: entity.GitConfigResponse{
			URL:      p.GitConfig.URL,
			Branch:   p.GitConfig.Branch,
			Username: p.GitConfig.Username,
		},
		EnvironmentID: p.EnvironmentID,
		RefFiles:      p.RefFiles,
		Env:           p.Env,
		CreatedAt:     p.CreatedAt,
		UpdatedAt:     p.UpdatedAt,
	}
}

// ── handlers ──────────────────────────────────────────────────────────────────

func (o *HTTPOrchestrator) handleListProjects(c *gin.Context) {
	id := c.MustGet(identityKey).(Identity)
	username := strings.TrimPrefix(id.UserID, id.TenantID+"/")

	projects, err := o.tenantStore.Projects(o.masterKey).List(c.Request.Context(), id.TenantID, username)
	if err != nil {
		c.String(http.StatusInternalServerError, "list projects: %s", err.Error())
		return
	}
	resp := make([]*entity.ProjectResponse, 0, len(projects))
	for _, p := range projects {
		resp = append(resp, projectToResponse(p))
	}
	c.JSON(http.StatusOK, resp)
}

func (o *HTTPOrchestrator) handleGetProject(c *gin.Context) {
	id := c.MustGet(identityKey).(Identity)
	username := strings.TrimPrefix(id.UserID, id.TenantID+"/")

	p, err := o.tenantStore.Projects(o.masterKey).Get(c.Request.Context(), c.Param("id"))
	if err != nil {
		c.String(http.StatusInternalServerError, "get project: %s", err.Error())
		return
	}
	if p == nil || p.TenantID != id.TenantID || p.OwnerID != username {
		c.String(http.StatusNotFound, "project not found")
		return
	}
	c.JSON(http.StatusOK, projectToResponse(p))
}

func (o *HTTPOrchestrator) handleCreateProject(c *gin.Context) {
	id := c.MustGet(identityKey).(Identity)
	username := strings.TrimPrefix(id.UserID, id.TenantID+"/")

	var req entity.ProjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.String(http.StatusBadRequest, "bad request: %s", err.Error())
		return
	}
	if req.Name == "" {
		c.String(http.StatusBadRequest, "name is required")
		return
	}

	ctx := c.Request.Context()

	if req.EnvironmentID != "" {
		if err := o.validateEnvRef(c, id.TenantID, req.EnvironmentID); err != nil {
			return
		}
	}

	p := &appstore.Project{
		ID:          newEnvID(), // reuse same crypto/rand helper
		TenantID:    id.TenantID,
		OwnerID:     username,
		Name:        req.Name,
		Description: req.Description,
		GitConfig: appstore.GitConfig{
			URL:      req.GitConfig.URL,
			Branch:   req.GitConfig.Branch,
			Username: req.GitConfig.Username,
			Token:    req.GitConfig.Token,
		},
		EnvironmentID: req.EnvironmentID,
		RefFiles:      req.RefFiles,
		Env:           req.Env,
	}
	if err := o.tenantStore.Projects(o.masterKey).Create(ctx, p); err != nil {
		c.String(http.StatusInternalServerError, "create project: %s", err.Error())
		return
	}
	c.JSON(http.StatusCreated, projectToResponse(p))
}

func (o *HTTPOrchestrator) handleUpdateProject(c *gin.Context) {
	id := c.MustGet(identityKey).(Identity)
	username := strings.TrimPrefix(id.UserID, id.TenantID+"/")

	var req entity.ProjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.String(http.StatusBadRequest, "bad request: %s", err.Error())
		return
	}

	ctx := c.Request.Context()
	existing, err := o.tenantStore.Projects(o.masterKey).Get(ctx, c.Param("id"))
	if err != nil {
		c.String(http.StatusInternalServerError, "get project: %s", err.Error())
		return
	}
	if existing == nil || existing.TenantID != id.TenantID || existing.OwnerID != username {
		c.String(http.StatusNotFound, "project not found")
		return
	}

	if req.EnvironmentID != "" && req.EnvironmentID != existing.EnvironmentID {
		if err := o.validateEnvRef(c, id.TenantID, req.EnvironmentID); err != nil {
			return
		}
	}

	if req.Name != "" {
		existing.Name = req.Name
	}
	existing.Description = req.Description
	existing.GitConfig.URL = req.GitConfig.URL
	existing.GitConfig.Branch = req.GitConfig.Branch
	existing.GitConfig.Username = req.GitConfig.Username
	// Token: only update when explicitly provided to preserve the existing value.
	if req.GitConfig.Token != "" {
		existing.GitConfig.Token = req.GitConfig.Token
	}
	existing.EnvironmentID = req.EnvironmentID
	existing.RefFiles = req.RefFiles
	existing.Env = req.Env

	if err := o.tenantStore.Projects(o.masterKey).Update(ctx, existing); err != nil {
		c.String(http.StatusInternalServerError, "update project: %s", err.Error())
		return
	}
	c.JSON(http.StatusOK, projectToResponse(existing))
}

func (o *HTTPOrchestrator) handleDeleteProject(c *gin.Context) {
	id := c.MustGet(identityKey).(Identity)
	username := strings.TrimPrefix(id.UserID, id.TenantID+"/")

	ctx := c.Request.Context()
	existing, err := o.tenantStore.Projects(o.masterKey).Get(ctx, c.Param("id"))
	if err != nil {
		c.String(http.StatusInternalServerError, "get project: %s", err.Error())
		return
	}
	if existing == nil || existing.TenantID != id.TenantID || existing.OwnerID != username {
		c.String(http.StatusNotFound, "project not found")
		return
	}

	hasSessions, err := o.harness.SessionStore().HasSessionsForProject(existing.ID)
	if err != nil {
		c.String(http.StatusInternalServerError, "check sessions: %s", err.Error())
		return
	}
	if hasSessions {
		c.String(http.StatusConflict, "project has associated sessions and cannot be deleted")
		return
	}

	if err := o.tenantStore.Projects(o.masterKey).Delete(ctx, existing.ID); err != nil {
		c.String(http.StatusInternalServerError, "delete project: %s", err.Error())
		return
	}
	c.Status(http.StatusNoContent)
}

// validateEnvRef checks that an environment_id refers to a tenant-scoped
// environment belonging to this tenant.
// Writes the error response and returns non-nil on failure.
func (o *HTTPOrchestrator) validateEnvRef(c *gin.Context, tenantID, envID string) error {
	env, err := o.tenantStore.Environments().Get(c.Request.Context(), envID)
	if err != nil {
		c.String(http.StatusInternalServerError, "validate environment: %s", err.Error())
		return err
	}
	if env == nil || env.TenantID != tenantID {
		c.String(http.StatusBadRequest, "environment not found")
		return errEnvNotFound
	}
	return nil
}

// errEnvNotFound is a sentinel used only to signal handler early-exit.
// The actual response has already been written before this is returned.
var errEnvNotFound = &envRefError{}

type envRefError struct{}

func (*envRefError) Error() string { return "environment ref error" }

// ── project resource initialisation ──────────────────────────────────────────

// resolveSession builds the MergedEnvironment for a session.
// Environment is taken exclusively from p.EnvironmentID when set; project-less
// sessions (or projects without an explicit environment) get an empty config.
// p may be nil for project-less sessions.
func (o *HTTPOrchestrator) resolveSession(ctx context.Context, _ string, _ string, p *appstore.Project) MergedEnvironment {
	var baseEnv *appstore.Environment
	if p != nil && p.EnvironmentID != "" {
		baseEnv, _ = o.tenantStore.Environments().Get(ctx, p.EnvironmentID)
	}

	merged := MergeEnvironments(baseEnv)

	if p != nil {
		// Project inline Env overrides all env-layer vars (last-writer-wins).
		for k, v := range p.Env {
			if merged.Env == nil {
				merged.Env = make(map[string]string)
			}
			merged.Env[k] = v
		}
		merged.GitConfig = p.GitConfig
		merged.RefFiles = p.RefFiles
	}

	return merged
}

// initSessionResources clones the git repository and writes reference files
// into the session workspace based on a resolved MergedEnvironment.
// A git clone failure is returned as an error; ref file failures are logged and skipped.
func (o *HTTPOrchestrator) initSessionResources(ctx context.Context, internalSID string, id Identity, merged MergedEnvironment) error {
	rm, ok := o.sandboxPool.(hands.ResourceManager)
	if !ok {
		return nil
	}

	if merged.GitConfig.URL != "" {
		token := merged.GitConfig.Token
		if vault.IsVaultRef(token) && o.secretRes != nil {
			if resolved, err := o.secretRes.Resolve(ctx, id.TenantID, id.UserID, token); err == nil {
				token = resolved
			} else {
				slog.WarnContext(ctx, "create session: resolve git token vault ref", "error", err)
			}
		}
		r := resources.GitResource{
			ID:         newResourceID(),
			URL:        merged.GitConfig.URL,
			Branch:     merged.GitConfig.Branch,
			TargetPath: gitRepoName(merged.GitConfig.URL),
			Token:      token,
		}
		if err := rm.AddGitResource(ctx, internalSID, r); err != nil {
			return fmt.Errorf("git clone %s: %w", merged.GitConfig.URL, err)
		}
	}

	for _, rf := range merged.RefFiles {
		targetPath := rf.Path
		if targetPath == "" {
			targetPath = path.Base(rf.URL)
		}
		r := resources.FileResource{
			ID:         newResourceID(),
			TargetPath: targetPath,
			SourceURL:  rf.URL,
		}
		if err := rm.AddFileResource(ctx, internalSID, r); err != nil {
			slog.WarnContext(ctx, "create session: add ref file",
				"url", rf.URL, "error", err)
		}
	}
	return nil
}

// gitRepoName extracts a safe directory name from a git URL.
// "https://github.com/org/myrepo.git" → "myrepo"
func gitRepoName(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil || u.Path == "" {
		return "repo"
	}
	base := strings.TrimSuffix(path.Base(u.Path), ".git")
	if base == "" || base == "." {
		return "repo"
	}
	return base
}

