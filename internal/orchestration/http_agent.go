package orchestration

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"forge/internal/entity"
	appstore "forge/internal/gateway/store"
)

func newAgentID() string {
	b := make([]byte, 10)
	_, _ = rand.Read(b)
	return "agent_" + hex.EncodeToString(b)
}

// ── read handlers (authed users) ──────────────────────────────────────────────

func (o *HTTPOrchestrator) handleListAgents(c *gin.Context) {
	id := c.MustGet(identityKey).(Identity)

	agents, err := o.tenantStore.Agents().List(c.Request.Context(), id.TenantID)
	if err != nil {
		c.String(http.StatusInternalServerError, "list agents: %s", err.Error())
		return
	}

	resp := make([]entity.AgentResponse, 0, len(agents))
	for _, a := range agents {
		resp = append(resp, agentToResponse(a))
	}
	c.JSON(http.StatusOK, resp)
}

func (o *HTTPOrchestrator) handleGetAgent(c *gin.Context) {
	id := c.MustGet(identityKey).(Identity)
	agentID := c.Param("id")

	a, err := o.tenantStore.Agents().Get(c.Request.Context(), id.TenantID, agentID)
	if err != nil {
		c.String(http.StatusInternalServerError, "get agent: %s", err.Error())
		return
	}
	if a == nil {
		c.String(http.StatusNotFound, "agent not found")
		return
	}
	if err := o.tenantStore.Agents().LoadAssociations(c.Request.Context(), a); err != nil {
		c.String(http.StatusInternalServerError, "load agent associations: %s", err.Error())
		return
	}
	c.JSON(http.StatusOK, agentToResponse(a))
}

// ── write handlers (admin only) ───────────────────────────────────────────────

func (o *HTTPOrchestrator) handleCreateAgent(c *gin.Context) {
	id := c.MustGet(identityKey).(Identity)

	var req entity.CreateAgentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.String(http.StatusBadRequest, "bad request: %s", err.Error())
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		c.String(http.StatusBadRequest, "name is required")
		return
	}

	ctx := c.Request.Context()

	// Clear the old default before setting a new one to satisfy the unique index.
	if req.IsDefault {
		if err := o.clearDefaultAgent(ctx, id.TenantID); err != nil {
			c.String(http.StatusInternalServerError, "clear default agent: %s", err.Error())
			return
		}
	}

	rec := &appstore.AgentRecord{
		ID:           newAgentID(),
		TenantID:     id.TenantID,
		Name:         strings.TrimSpace(req.Name),
		Description:  req.Description,
		Model:        req.Model,
		SystemPrompt: req.SystemPrompt,
		ToolConfig:   req.ToolConfig,
		IsDefault:    req.IsDefault,
	}
	if rec.ToolConfig == nil {
		rec.ToolConfig = map[string]bool{}
	}

	if err := o.tenantStore.Agents().Create(ctx, rec); err != nil {
		c.String(http.StatusInternalServerError, "create agent: %s", err.Error())
		return
	}
	if err := o.tenantStore.Agents().SetMCPs(ctx, rec.ID, req.MCPServerNames); err != nil {
		c.String(http.StatusInternalServerError, "set agent MCPs: %s", err.Error())
		return
	}
	if err := o.tenantStore.Agents().SetSkills(ctx, rec.ID, req.SkillNames); err != nil {
		c.String(http.StatusInternalServerError, "set agent skills: %s", err.Error())
		return
	}
	if err := o.tenantStore.Agents().SetCallableAgents(ctx, rec.ID, req.CallableAgents); err != nil {
		c.String(http.StatusInternalServerError, "set agent callable agents: %s", err.Error())
		return
	}

	rec.MCPServerNames = req.MCPServerNames
	rec.SkillNames = req.SkillNames
	rec.CallableAgents = req.CallableAgents
	c.JSON(http.StatusCreated, agentToResponse(rec))
}

func (o *HTTPOrchestrator) handleUpdateAgent(c *gin.Context) {
	id := c.MustGet(identityKey).(Identity)
	agentID := c.Param("id")

	var req entity.UpdateAgentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.String(http.StatusBadRequest, "bad request: %s", err.Error())
		return
	}

	ctx := c.Request.Context()

	existing, err := o.tenantStore.Agents().Get(ctx, id.TenantID, agentID)
	if err != nil {
		c.String(http.StatusInternalServerError, "get agent: %s", err.Error())
		return
	}
	if existing == nil {
		c.String(http.StatusNotFound, "agent not found")
		return
	}

	// Apply patch fields.
	if req.Name != nil {
		existing.Name = strings.TrimSpace(*req.Name)
	}
	if req.Description != nil {
		existing.Description = *req.Description
	}
	if req.Model != nil {
		existing.Model = *req.Model
	}
	if req.SystemPrompt != nil {
		existing.SystemPrompt = *req.SystemPrompt
	}
	if req.ToolConfig != nil {
		existing.ToolConfig = req.ToolConfig
	}
	if req.IsDefault != nil && *req.IsDefault && !existing.IsDefault {
		if err := o.clearDefaultAgent(ctx, id.TenantID); err != nil {
			c.String(http.StatusInternalServerError, "clear default agent: %s", err.Error())
			return
		}
		existing.IsDefault = true
	}

	newVersion, err := o.tenantStore.Agents().Update(ctx, existing)
	if err != nil {
		c.String(http.StatusInternalServerError, "update agent: %s", err.Error())
		return
	}
	if newVersion == 0 {
		c.String(http.StatusNotFound, "agent not found")
		return
	}
	existing.Version = newVersion

	// Update associations if provided.
	if req.MCPServerNames != nil {
		if err := o.tenantStore.Agents().SetMCPs(ctx, agentID, req.MCPServerNames); err != nil {
			c.String(http.StatusInternalServerError, "set agent MCPs: %s", err.Error())
			return
		}
		existing.MCPServerNames = req.MCPServerNames
	}
	if req.SkillNames != nil {
		if err := o.tenantStore.Agents().SetSkills(ctx, agentID, req.SkillNames); err != nil {
			c.String(http.StatusInternalServerError, "set agent skills: %s", err.Error())
			return
		}
		existing.SkillNames = req.SkillNames
	}
	if req.CallableAgents != nil {
		if err := o.tenantStore.Agents().SetCallableAgents(ctx, agentID, req.CallableAgents); err != nil {
			c.String(http.StatusInternalServerError, "set agent callable agents: %s", err.Error())
			return
		}
		existing.CallableAgents = req.CallableAgents
	}

	if existing.MCPServerNames == nil || existing.SkillNames == nil {
		_ = o.tenantStore.Agents().LoadAssociations(ctx, existing)
	}
	c.JSON(http.StatusOK, agentToResponse(existing))
}

func (o *HTTPOrchestrator) handleArchiveAgent(c *gin.Context) {
	id := c.MustGet(identityKey).(Identity)
	agentID := c.Param("id")

	if err := o.tenantStore.Agents().Archive(c.Request.Context(), id.TenantID, agentID); err != nil {
		c.String(http.StatusInternalServerError, "archive agent: %s", err.Error())
		return
	}
	c.Status(http.StatusNoContent)
}

// ── helpers ───────────────────────────────────────────────────────────────────

// clearDefaultAgent unsets is_default on any current default agent for the tenant.
func (o *HTTPOrchestrator) clearDefaultAgent(ctx context.Context, tenantID string) error {
	return o.tenantStore.Agents().ClearDefault(ctx, tenantID)
}

func agentToResponse(a *appstore.AgentRecord) entity.AgentResponse {
	return entity.AgentResponse{
		ID:             a.ID,
		Name:           a.Name,
		Description:    a.Description,
		Version:        a.Version,
		Model:          a.Model,
		SystemPrompt:   a.SystemPrompt,
		ToolConfig:     a.ToolConfig,
		CallableAgents: a.CallableAgents,
		MCPServerNames: a.MCPServerNames,
		SkillNames:     a.SkillNames,
		IsDefault:      a.IsDefault,
		CreatedAt:      a.CreatedAt.UTC().Format(entity.TimeFormatISO8601),
		UpdatedAt:      a.UpdatedAt.UTC().Format(entity.TimeFormatISO8601),
	}
}
