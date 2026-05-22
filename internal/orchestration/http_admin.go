package orchestration

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"

	"forge/internal/entity"
	appstore "forge/internal/gateway/store"
)

// ── create user (admin) ───────────────────────────────────────────────────────

func (o *HTTPOrchestrator) handleCreateUser(c *gin.Context) {
	id := c.MustGet(identityKey).(Identity)

	var req entity.CreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.String(http.StatusBadRequest, "bad request: %s", err.Error())
		return
	}
	if req.Username == "" {
		c.String(http.StatusBadRequest, "username is required")
		return
	}
	if req.Password == "" {
		c.String(http.StatusBadRequest, "password is required")
		return
	}
	role := req.Role
	if role == "" {
		role = entity.RoleMember
	}
	switch role {
	case entity.RoleAdmin, entity.RoleMember, entity.RoleViewer:
	default:
		c.String(http.StatusBadRequest, "role must be admin, member, or viewer")
		return
	}

	// Username must be globally unique for login to work.
	existing, _, err := o.tenantStore.Users().FindByUsername(c.Request.Context(), req.Username)
	if err != nil {
		c.String(http.StatusInternalServerError, "lookup user: %s", err.Error())
		return
	}
	if existing != nil {
		c.String(http.StatusConflict, "username already exists")
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.String(http.StatusInternalServerError, "hash password: %s", err.Error())
		return
	}

	if err := o.tenantStore.Users().Seed(c.Request.Context(), &appstore.User{
		TenantID:     id.TenantID,
		Username:     req.Username,
		PasswordHash: string(hash),
		Role:         role,
	}); err != nil {
		c.String(http.StatusInternalServerError, "create user: %s", err.Error())
		return
	}
	c.Status(http.StatusNoContent)
}

// ── tenant-level vault secrets (admin) ───────────────────────────────────────

func (o *HTTPOrchestrator) handleListTenantVaults(c *gin.Context) {
	id := c.MustGet(identityKey).(Identity)
	if o.masterKey == nil {
		c.JSON(http.StatusOK, []entity.VaultListItem{})
		return
	}
	secrets, err := o.tenantStore.Secrets(o.masterKey).List(c.Request.Context(), id.TenantID, "")
	if err != nil {
		c.String(http.StatusInternalServerError, "list tenant vaults: %s", err.Error())
		return
	}
	out := make([]entity.VaultListItem, len(secrets))
	for i, s := range secrets {
		out[i] = entity.VaultListItem{Name: s.Name, Description: s.Description, UpdatedAt: s.UpdatedAt}
	}
	c.JSON(http.StatusOK, out)
}

func (o *HTTPOrchestrator) handleSetTenantVault(c *gin.Context) {
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
	if err := o.tenantStore.Secrets(o.masterKey).Set(c.Request.Context(), id.TenantID, "", req.Name, req.Description, req.Value); err != nil {
		c.String(http.StatusInternalServerError, "set tenant vault: %s", err.Error())
		return
	}
	c.Status(http.StatusNoContent)
}

func (o *HTTPOrchestrator) handleDeleteTenantVault(c *gin.Context) {
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
	if err := o.tenantStore.Secrets(o.masterKey).Delete(c.Request.Context(), id.TenantID, "", name); err != nil {
		c.String(http.StatusInternalServerError, "delete tenant vault: %s", err.Error())
		return
	}
	c.Status(http.StatusNoContent)
}

// ── tenant-level MCP servers (admin) ─────────────────────────────────────────

func (o *HTTPOrchestrator) handleListTenantMCPServers(c *gin.Context) {
	id := c.MustGet(identityKey).(Identity)
	recs, err := o.tenantStore.MCPServers().List(c.Request.Context(), id.TenantID)
	if err != nil {
		c.String(http.StatusInternalServerError, "list tenant mcp servers: %s", err.Error())
		return
	}
	out := make([]entity.MCPServerResponse, len(recs))
	for i, rec := range recs {
		out[i] = mcpRecordToResponse(rec)
	}
	c.JSON(http.StatusOK, out)
}

func (o *HTTPOrchestrator) handleUpsertTenantMCPServer(c *gin.Context) {
	id := c.MustGet(identityKey).(Identity)
	var req entity.UpsertMCPServerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.String(http.StatusBadRequest, "bad request: %s", err.Error())
		return
	}
	if name := c.Param("name"); name != "" {
		req.Name = name
	}
	if req.Name == "" {
		c.String(http.StatusBadRequest, "name is required")
		return
	}
	if req.Type == "" {
		req.Type = entity.MCPTypeStdio
	}
	rec := &appstore.MCPServerRecord{
		TenantID: id.TenantID,
		Name:     req.Name,
		Type:     req.Type,
		Command:  req.Command,
		Args:     req.Args,
		Env:      req.Env,
		URL:      req.URL,
		Headers:  req.Headers,
		Disabled: req.Disabled,
	}
	if err := o.tenantStore.MCPServers().Upsert(c.Request.Context(), rec); err != nil {
		c.String(http.StatusInternalServerError, "upsert tenant mcp server: %s", err.Error())
		return
	}
	c.Status(http.StatusNoContent)
}

func (o *HTTPOrchestrator) handleDeleteTenantMCPServer(c *gin.Context) {
	id := c.MustGet(identityKey).(Identity)
	name := c.Param("name")
	if name == "" {
		c.String(http.StatusBadRequest, "missing server name")
		return
	}
	if err := o.tenantStore.MCPServers().Delete(c.Request.Context(), id.TenantID, name); err != nil {
		c.String(http.StatusInternalServerError, "delete tenant mcp server: %s", err.Error())
		return
	}
	c.Status(http.StatusNoContent)
}

// ── tenant-level skills (admin) ───────────────────────────────────────────────

func (o *HTTPOrchestrator) handleListTenantSkills(c *gin.Context) {
	id := c.MustGet(identityKey).(Identity)
	recs, err := o.tenantStore.UserSkills().List(c.Request.Context(), id.TenantID, "")
	if err != nil {
		c.String(http.StatusInternalServerError, "list tenant skills: %s", err.Error())
		return
	}
	out := make([]entity.SkillMetaResponse, len(recs))
	for i, rec := range recs {
		out[i] = entity.SkillMetaResponse{Name: rec.Name, UpdatedAt: rec.UpdatedAt}
	}
	c.JSON(http.StatusOK, out)
}

func (o *HTTPOrchestrator) handleGetTenantSkill(c *gin.Context) {
	id := c.MustGet(identityKey).(Identity)
	name := c.Param("name")
	if name == "" {
		c.String(http.StatusBadRequest, "missing skill name")
		return
	}
	rec, err := o.tenantStore.UserSkills().Get(c.Request.Context(), id.TenantID, "", name)
	if err != nil {
		c.String(http.StatusInternalServerError, "get tenant skill: %s", err.Error())
		return
	}
	if rec == nil {
		c.String(http.StatusNotFound, "skill not found")
		return
	}
	c.JSON(http.StatusOK, entity.SkillFullResponse{Name: rec.Name, Content: rec.Content, UpdatedAt: rec.UpdatedAt})
}

func (o *HTTPOrchestrator) handleUpsertTenantSkill(c *gin.Context) {
	id := c.MustGet(identityKey).(Identity)
	var req entity.UpsertSkillRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.String(http.StatusBadRequest, "bad request: %s", err.Error())
		return
	}
	if name := c.Param("name"); name != "" {
		req.Name = name
	}
	if req.Name == "" {
		c.String(http.StatusBadRequest, "name is required")
		return
	}
	if req.Content == "" {
		c.String(http.StatusBadRequest, "content is required")
		return
	}
	rec := &appstore.UserSkillRecord{
		TenantID: id.TenantID,
		UserID:   "", // tenant-level
		Name:     req.Name,
		Content:  req.Content,
	}
	if err := o.tenantStore.UserSkills().Upsert(c.Request.Context(), rec); err != nil {
		c.String(http.StatusInternalServerError, "upsert tenant skill: %s", err.Error())
		return
	}
	c.Status(http.StatusNoContent)
}

func (o *HTTPOrchestrator) handleDeleteTenantSkill(c *gin.Context) {
	id := c.MustGet(identityKey).(Identity)
	name := c.Param("name")
	if name == "" {
		c.String(http.StatusBadRequest, "missing skill name")
		return
	}
	if err := o.tenantStore.UserSkills().Delete(c.Request.Context(), id.TenantID, "", name); err != nil {
		c.String(http.StatusInternalServerError, "delete tenant skill: %s", err.Error())
		return
	}
	c.Status(http.StatusNoContent)
}
