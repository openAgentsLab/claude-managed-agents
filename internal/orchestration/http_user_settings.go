package orchestration

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"forge/internal/entity"
)

// handleGetUserSettings returns the calling user's current model/brain overrides.
func (o *HTTPOrchestrator) handleGetUserSettings(c *gin.Context) {
	id := c.MustGet(identityKey).(Identity)
	username := strings.TrimPrefix(id.UserID, id.TenantID+"/")
	s, err := o.tenantStore.Users().GetSettings(c.Request.Context(), id.TenantID, username)
	if err != nil {
		c.String(http.StatusInternalServerError, "get user settings: %s", err.Error())
		return
	}
	c.JSON(http.StatusOK, s)
}

// handleUpdateUserSettings lets the calling user patch their own model/brain overrides.
func (o *HTTPOrchestrator) handleUpdateUserSettings(c *gin.Context) {
	id := c.MustGet(identityKey).(Identity)

	var req entity.UpdateUserSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.String(http.StatusBadRequest, "bad request: %s", err.Error())
		return
	}

	username := strings.TrimPrefix(id.UserID, id.TenantID+"/")

	current, err := o.tenantStore.Users().GetSettings(c.Request.Context(), id.TenantID, username)
	if err != nil {
		c.String(http.StatusInternalServerError, "get user settings: %s", err.Error())
		return
	}
	if req.ModelOverride != nil {
		current.ModelOverride = req.ModelOverride
	}
	if req.BrainOverride != nil {
		current.BrainOverride = req.BrainOverride
	}

	if err := o.tenantStore.Users().UpdateSettings(c.Request.Context(), id.TenantID, username, current); err != nil {
		c.String(http.StatusInternalServerError, "update user settings: %s", err.Error())
		return
	}
	c.Status(http.StatusNoContent)
}
