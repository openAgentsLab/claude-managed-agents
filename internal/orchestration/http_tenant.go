package orchestration

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"forge/internal/entity"
)

func (o *HTTPOrchestrator) handleGetTenant(c *gin.Context) {
	id := c.MustGet(identityKey).(Identity)

	t, err := o.tenantStore.Tenants().Get(c.Request.Context(), id.TenantID)
	if err != nil {
		c.String(http.StatusInternalServerError, "get tenant: %s", err.Error())
		return
	}
	name := id.TenantID
	if t != nil {
		name = t.Name
	}

	c.JSON(http.StatusOK, entity.TenantInfoResponse{
		ID:       id.TenantID,
		Name:     name,
		Role:     id.Role,
		Settings: o.res.Settings(c.Request.Context(), id.TenantID),
	})
}

func (o *HTTPOrchestrator) handleUpdateTenantSettings(c *gin.Context) {
	id := c.MustGet(identityKey).(Identity)

	var req entity.UpdateSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.String(http.StatusBadRequest, "bad request: %s", err.Error())
		return
	}

	// Start from the current effective settings and patch.
	current := o.res.Settings(c.Request.Context(), id.TenantID)
	if req.AllowRules != nil {
		current.AllowRules = req.AllowRules
	}
	if req.DenyRules != nil {
		current.DenyRules = req.DenyRules
	}
	if req.ResourceQuota != nil {
		current.ResourceQuota = *req.ResourceQuota
	}
	if req.ModelOverride != nil {
		current.ModelOverride = req.ModelOverride
	}
	if req.BrainOverride != nil {
		current.BrainOverride = req.BrainOverride
	}

	// Persist to DB and update the in-memory cache.
	if err := o.tenantStore.Tenants().UpdateSettings(c.Request.Context(), id.TenantID, configSettingsToStore(current)); err != nil {
		c.String(http.StatusInternalServerError, "update settings: %s", err.Error())
		return
	}
	// Re-fetch to get the DB-assigned updated_at so the refresh loop won't
	// redundantly reload settings this node just wrote.
	updatedAt := time.Now()
	if t, err := o.tenantStore.Tenants().Get(c.Request.Context(), id.TenantID); err == nil && t != nil {
		updatedAt = t.UpdatedAt
	}
	o.res.PutSettings(id.TenantID, current, updatedAt)

	// Rebuild live permission engines so new rules take effect immediately.
	o.engines.set(id.TenantID, buildTenantEngine(current, o.globalCfg))
	o.engines.set(id.TenantID+":viewer", buildViewerEngine(o.globalCfg))

	c.Status(http.StatusNoContent)
}

func (o *HTTPOrchestrator) handleListUsers(c *gin.Context) {
	id := c.MustGet(identityKey).(Identity)

	users, err := o.tenantStore.Users().List(c.Request.Context(), id.TenantID)
	if err != nil {
		c.String(http.StatusInternalServerError, "list users: %s", err.Error())
		return
	}

	resp := make([]entity.UserInfoResponse, 0, len(users))
	for _, u := range users {
		resp = append(resp, entity.UserInfoResponse{Username: u.Username, Role: u.Role})
	}

	c.JSON(http.StatusOK, resp)
}

func (o *HTTPOrchestrator) handleUpdateUserRole(c *gin.Context) {
	id := c.MustGet(identityKey).(Identity)

	username := c.Param("username")
	if username == "" {
		c.String(http.StatusBadRequest, "missing username")
		return
	}

	var req entity.UpdateRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.String(http.StatusBadRequest, "bad request: %s", err.Error())
		return
	}
	switch req.Role {
	case entity.RoleAdmin, entity.RoleMember, entity.RoleViewer:
	default:
		c.String(http.StatusBadRequest, "role must be admin, member, or viewer")
		return
	}

	// Verify the target user exists in this tenant before updating.
	u, err := o.tenantStore.Users().Get(c.Request.Context(), id.TenantID, username)
	if err != nil {
		c.String(http.StatusInternalServerError, "lookup user: %s", err.Error())
		return
	}
	if u == nil {
		c.String(http.StatusNotFound, "user not found")
		return
	}

	if err := o.tenantStore.Users().UpdateRole(c.Request.Context(), id.TenantID, username, req.Role); err != nil {
		c.String(http.StatusInternalServerError, "update role: %s", err.Error())
		return
	}

	c.Status(http.StatusNoContent)
}
