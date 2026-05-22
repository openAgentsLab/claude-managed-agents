package orchestration

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"

	"github.com/gin-gonic/gin"

	"forge/internal/entity"
	appstore "forge/internal/gateway/store"
)

// envToResponse converts a store Environment to an API response.
func envToResponse(e *appstore.Environment) *entity.EnvironmentResponse {
	return &entity.EnvironmentResponse{
		ID:          e.ID,
		Scope:       e.Scope,
		Name:        e.Name,
		Description: e.Description,
		Packages:    e.Packages,
		Networking:  e.Networking,
		Env:         e.Env,
		CreatedAt:   e.CreatedAt,
		UpdatedAt:   e.UpdatedAt,
	}
}

func newEnvID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// ── user-facing handlers (/api/v1/environments) ────────────────────────────

// handleListEnvironments returns all tenant-scoped environments.
func (o *HTTPOrchestrator) handleListEnvironments(c *gin.Context) {
	id := c.MustGet(identityKey).(Identity)

	envs, err := o.tenantStore.Environments().List(c.Request.Context(), id.TenantID)
	if err != nil {
		c.String(http.StatusInternalServerError, "list environments: %s", err.Error())
		return
	}
	resp := make([]*entity.EnvironmentResponse, 0, len(envs))
	for _, e := range envs {
		resp = append(resp, envToResponse(e))
	}
	c.JSON(http.StatusOK, resp)
}

// ── admin handlers (/admin/v1/environments) ───────────────────────────────

// handleAdminListEnvironments lists all tenant-scoped Environments.
func (o *HTTPOrchestrator) handleAdminListEnvironments(c *gin.Context) {
	id := c.MustGet(identityKey).(Identity)

	envs, err := o.tenantStore.Environments().List(c.Request.Context(), id.TenantID)
	if err != nil {
		c.String(http.StatusInternalServerError, "list environments: %s", err.Error())
		return
	}
	resp := make([]*entity.EnvironmentResponse, 0, len(envs))
	for _, e := range envs {
		resp = append(resp, envToResponse(e))
	}
	c.JSON(http.StatusOK, resp)
}

// handleAdminCreateEnvironment creates a tenant-scoped Environment.
func (o *HTTPOrchestrator) handleAdminCreateEnvironment(c *gin.Context) {
	id := c.MustGet(identityKey).(Identity)

	var req entity.EnvironmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.String(http.StatusBadRequest, "bad request: %s", err.Error())
		return
	}
	if req.Name == "" {
		c.String(http.StatusBadRequest, "name is required")
		return
	}

	e := buildEnv(newEnvID(), id.TenantID, req)
	if err := o.tenantStore.Environments().Create(c.Request.Context(), e); err != nil {
		c.String(http.StatusInternalServerError, "create environment: %s", err.Error())
		return
	}
	c.JSON(http.StatusCreated, envToResponse(e))
}

// handleAdminUpdateEnvironment updates a tenant-scoped Environment.
func (o *HTTPOrchestrator) handleAdminUpdateEnvironment(c *gin.Context) {
	id := c.MustGet(identityKey).(Identity)
	envID := c.Param("id")

	var req entity.EnvironmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.String(http.StatusBadRequest, "bad request: %s", err.Error())
		return
	}

	ctx := c.Request.Context()
	existing, err := o.tenantStore.Environments().Get(ctx, envID)
	if err != nil {
		c.String(http.StatusInternalServerError, "get environment: %s", err.Error())
		return
	}
	if existing == nil {
		c.String(http.StatusNotFound, "environment not found")
		return
	}
	if existing.TenantID != id.TenantID || existing.Scope != appstore.EnvScopeTenant {
		c.String(http.StatusForbidden, "not a tenant environment in this tenant")
		return
	}

	applyEnvRequest(existing, req)
	if err := o.tenantStore.Environments().Update(ctx, existing); err != nil {
		c.String(http.StatusInternalServerError, "update environment: %s", err.Error())
		return
	}
	c.JSON(http.StatusOK, envToResponse(existing))
}

// handleAdminDeleteEnvironment deletes a tenant-scoped Environment.
// Rejects the delete if the environment is referenced by any Project.
func (o *HTTPOrchestrator) handleAdminDeleteEnvironment(c *gin.Context) {
	id := c.MustGet(identityKey).(Identity)
	envID := c.Param("id")

	ctx := c.Request.Context()
	existing, err := o.tenantStore.Environments().Get(ctx, envID)
	if err != nil {
		c.String(http.StatusInternalServerError, "get environment: %s", err.Error())
		return
	}
	if existing == nil {
		c.String(http.StatusNotFound, "environment not found")
		return
	}
	if existing.TenantID != id.TenantID || existing.Scope != appstore.EnvScopeTenant {
		c.String(http.StatusForbidden, "not a tenant environment in this tenant")
		return
	}

	refs, err := o.tenantStore.Environments().CountReferences(ctx, envID)
	if err != nil {
		c.String(http.StatusInternalServerError, "check references: %s", err.Error())
		return
	}
	if refs > 0 {
		c.String(http.StatusConflict, "environment is referenced by %d project(s)", refs)
		return
	}

	if err := o.tenantStore.Environments().Delete(ctx, envID); err != nil {
		c.String(http.StatusInternalServerError, "delete environment: %s", err.Error())
		return
	}
	c.Status(http.StatusNoContent)
}

// ── helpers ────────────────────────────────────────────────────────────────

func buildEnv(id, tenantID string, req entity.EnvironmentRequest) *appstore.Environment {
	e := &appstore.Environment{
		ID:          id,
		TenantID:    tenantID,
		Scope:       appstore.EnvScopeTenant,
		Name:        req.Name,
		Description: req.Description,
		Packages:    req.Packages,
		Env:         req.Env,
	}
	if req.Networking != nil {
		e.Networking = *req.Networking
	}
	if e.Networking.Mode == "" {
		e.Networking.Mode = appstore.NetworkingUnrestricted
	}
	return e
}

func applyEnvRequest(e *appstore.Environment, req entity.EnvironmentRequest) {
	if req.Name != "" {
		e.Name = req.Name
	}
	e.Description = req.Description
	e.Packages = req.Packages
	e.Env = req.Env
	if req.Networking != nil {
		e.Networking = *req.Networking
	}
	if e.Networking.Mode == "" {
		e.Networking.Mode = appstore.NetworkingUnrestricted
	}
}
