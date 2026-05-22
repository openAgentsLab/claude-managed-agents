package orchestration

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"forge/internal/entity"
	appstore "forge/internal/gateway/store"
)

// ── Vault handlers ────────────────────────────────────────────────────────────

func (o *HTTPOrchestrator) handleListVaults(c *gin.Context) {
	id := c.MustGet(identityKey).(Identity)
	if o.masterKey == nil {
		c.JSON(http.StatusOK, []entity.VaultListItem{})
		return
	}
	secrets, err := o.tenantStore.Secrets(o.masterKey).List(c.Request.Context(), id.TenantID, id.UserID)
	if err != nil {
		c.String(http.StatusInternalServerError, "list vaults: %s", err.Error())
		return
	}
	out := make([]entity.VaultListItem, len(secrets))
	for i, s := range secrets {
		out[i] = entity.VaultListItem{Name: s.Name, Description: s.Description, UpdatedAt: s.UpdatedAt}
	}
	c.JSON(http.StatusOK, out)
}

func (o *HTTPOrchestrator) handleSetVault(c *gin.Context) {
	id := c.MustGet(identityKey).(Identity)
	if o.masterKey == nil {
		c.String(http.StatusServiceUnavailable, "vault not configured (set FORGE_VAULT_KEY)")
		return
	}
	var req entity.SetVaultRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.String(http.StatusBadRequest, "bad request: %s", err.Error())
		return
	}
	if req.Name == "" {
		c.String(http.StatusBadRequest, "name is required")
		return
	}
	if req.Value == "" {
		c.String(http.StatusBadRequest, "value is required")
		return
	}
	if err := o.tenantStore.Secrets(o.masterKey).Set(c.Request.Context(), id.TenantID, id.UserID, req.Name, req.Description, req.Value); err != nil {
		c.String(http.StatusInternalServerError, "set vault: %s", err.Error())
		return
	}
	c.Status(http.StatusNoContent)
}

func (o *HTTPOrchestrator) handleDeleteVault(c *gin.Context) {
	id := c.MustGet(identityKey).(Identity)
	if o.masterKey == nil {
		c.Status(http.StatusNoContent)
		return
	}
	name := c.Param("name")
	if name == "" {
		c.String(http.StatusBadRequest, "missing vault name")
		return
	}
	if err := o.tenantStore.Secrets(o.masterKey).Delete(c.Request.Context(), id.TenantID, id.UserID, name); err != nil {
		c.String(http.StatusInternalServerError, "delete vault: %s", err.Error())
		return
	}
	c.Status(http.StatusNoContent)
}

// ── MCP server config handlers ────────────────────────────────────────────────

// mcpRecordToResponse converts a store MCPServerRecord to an API response.
func mcpRecordToResponse(rec *appstore.MCPServerRecord) entity.MCPServerResponse {
	return entity.MCPServerResponse{
		Name:      rec.Name,
		Type:      rec.Type,
		Command:   rec.Command,
		Args:      rec.Args,
		Env:       rec.Env,
		URL:       rec.URL,
		Headers:   rec.Headers,
		Disabled:  rec.Disabled,
		UpdatedAt: rec.UpdatedAt,
	}
}

