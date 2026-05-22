package orchestration

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"forge/internal/entity"
	"forge/internal/hands"
)

func (o *HTTPOrchestrator) handleListOutputs(c *gin.Context) {
	id := c.MustGet(identityKey).(Identity)
	clientID := c.Param("id")
	if clientID == "" {
		c.String(http.StatusBadRequest, "missing session id")
		return
	}
	internalSID := scopedSessionID(id.UserID, clientID)

	op, ok := o.sandboxPool.(hands.OutputsProvider)
	if !ok {
		c.JSON(http.StatusServiceUnavailable, entity.ErrorResponse{Error: "outputs not supported by this sandbox driver"})
		return
	}

	entries, err := op.ListOutputs(c.Request.Context(), internalSID)
	if err != nil {
		if errors.Is(err, hands.ErrSharedStorageUnavailable) {
			c.JSON(http.StatusServiceUnavailable, entity.ErrorResponse{Error: err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, entity.ErrorResponse{Error: err.Error()})
		return
	}
	if entries == nil {
		entries = []hands.OutputEntry{}
	}
	c.JSON(http.StatusOK, entries)
}

func (o *HTTPOrchestrator) handleReadOutput(c *gin.Context) {
	id := c.MustGet(identityKey).(Identity)
	clientID := c.Param("id")
	if clientID == "" {
		c.String(http.StatusBadRequest, "missing session id")
		return
	}
	// Gin includes the leading slash in wildcard params; strip it.
	path := strings.TrimPrefix(c.Param("path"), "/")
	if path == "" {
		c.String(http.StatusBadRequest, "missing output path")
		return
	}
	internalSID := scopedSessionID(id.UserID, clientID)

	op, ok := o.sandboxPool.(hands.OutputsProvider)
	if !ok {
		c.JSON(http.StatusServiceUnavailable, entity.ErrorResponse{Error: "outputs not supported by this sandbox driver"})
		return
	}

	data, err := op.ReadOutput(c.Request.Context(), internalSID, path)
	if err != nil {
		if errors.Is(err, hands.ErrSharedStorageUnavailable) {
			c.JSON(http.StatusServiceUnavailable, entity.ErrorResponse{Error: err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, entity.ErrorResponse{Error: err.Error()})
		return
	}
	c.Data(http.StatusOK, "application/octet-stream", data)
}
