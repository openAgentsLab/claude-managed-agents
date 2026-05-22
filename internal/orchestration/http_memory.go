package orchestration

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"

	"forge/internal/entity"
	"forge/internal/memory"
)

const builtinStoreCount = 3 // user + project + tenant

// validateMemoryStores checks that every custom store ID is accessible to the
// caller and that the total mount count stays within the 8-store limit.
func (o *HTTPOrchestrator) validateMemoryStores(_ context.Context, storeIDs []string, callerUserID, tenantID string) error {
	if o.memManager == nil {
		return fmt.Errorf("memory system is disabled")
	}
	if builtinStoreCount+len(storeIDs) > memory.MaxMountedStores {
		return fmt.Errorf("cannot mount more than %d stores per session (3 built-in + %d custom)",
			memory.MaxMountedStores, memory.MaxMountedStores-builtinStoreCount)
	}
	for _, id := range storeIDs {
		info, err := o.memManager.Get(id)
		if err != nil {
			if errors.Is(err, memory.ErrNotFound) {
				return fmt.Errorf("memory store %q not found", id)
			}
			return fmt.Errorf("memory store %q: %w", id, err)
		}
		if !storeVisibleTo(info, callerUserID, tenantID) {
			return fmt.Errorf("memory store %q: access denied", id)
		}
	}
	return nil
}

// mountSessionMemoryStores builds a SessionStores from the 3 built-in stores
// plus the custom store IDs persisted when the session was created.
func (o *HTTPOrchestrator) mountSessionMemoryStores(userID, projectID, tenantID string, tenantWritable bool, customStoreIDs []string) *memory.SessionStores {
	ss := memory.NewSession(
		o.memPool.UserStore(userID),
		o.memPool.ProjectStore(projectID),
		o.memPool.TenantStore(tenantID),
		tenantWritable,
	)
	if o.memManager != nil {
		for _, id := range customStoreIDs {
			st, writable, err := o.memManager.ResolveForSession(id, userID)
			if err != nil {
				continue // store may have been deleted after session creation
			}
			_ = ss.Mount(st, writable)
		}
	}
	return ss
}

// storeVisibleTo returns true when callerUserID may read the store.
func storeVisibleTo(info memory.StoreInfo, callerUserID, tenantID string) bool {
	if info.CreatedBy == callerUserID {
		return true
	}
	if info.Visibility == entity.MemoryVisibilitySharedTenant {
		creatorTenant := tenantPrefix(info.CreatedBy)
		return creatorTenant == tenantID
	}
	return false
}

// coalesceStr returns a if non-empty, otherwise b.
func coalesceStr(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func tenantPrefix(userID string) string {
	for i, c := range userID {
		if c == '/' {
			return userID[:i]
		}
	}
	return userID
}

func (o *HTTPOrchestrator) handleListMemoryStores(c *gin.Context) {
	if o.memManager == nil {
		c.JSON(http.StatusOK, []memory.StoreInfo{})
		return
	}
	id := c.MustGet(identityKey).(Identity)
	infos, err := o.memManager.List(id.UserID, id.TenantID)
	if err != nil {
		c.String(http.StatusInternalServerError, "list memory stores: %s", err.Error())
		return
	}
	if infos == nil {
		infos = []memory.StoreInfo{}
	}
	c.JSON(http.StatusOK, infos)
}

func (o *HTTPOrchestrator) handleCreateMemoryStore(c *gin.Context) {
	if o.memManager == nil {
		c.String(http.StatusServiceUnavailable, "memory system disabled")
		return
	}
	id := c.MustGet(identityKey).(Identity)

	var req entity.CreateMemoryStoreRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.String(http.StatusBadRequest, "bad request: %s", err.Error())
		return
	}

	storeID, err := o.memManager.Create(memory.CreateStoreRequest{
		Name:        req.Name,
		Description: req.Description,
		Visibility:  req.Visibility,
		WritePolicy: req.WritePolicy,
		CreatedBy:   id.UserID,
	})
	if err != nil {
		c.String(http.StatusBadRequest, "%s", err.Error())
		return
	}

	info, err := o.memManager.Get(storeID)
	if err != nil {
		c.String(http.StatusInternalServerError, "get store: %s", err.Error())
		return
	}
	c.JSON(http.StatusCreated, info)
}

func (o *HTTPOrchestrator) handleGetMemoryStore(c *gin.Context) {
	if o.memManager == nil {
		c.String(http.StatusServiceUnavailable, "memory system disabled")
		return
	}
	id := c.MustGet(identityKey).(Identity)
	storeID := c.Param("id")
	info, err := o.memManager.Get(storeID)
	if err != nil {
		if errors.Is(err, memory.ErrNotFound) {
			c.String(http.StatusNotFound, "store not found")
			return
		}
		c.String(http.StatusInternalServerError, "get store: %s", err.Error())
		return
	}
	// Return 404 (not 403) so callers cannot probe for private store IDs.
	if !storeVisibleTo(info, id.UserID, id.TenantID) {
		c.String(http.StatusNotFound, "store not found")
		return
	}
	c.JSON(http.StatusOK, info)
}

func (o *HTTPOrchestrator) handleUpdateMemoryStore(c *gin.Context) {
	if o.memManager == nil {
		c.String(http.StatusServiceUnavailable, "memory system disabled")
		return
	}
	id := c.MustGet(identityKey).(Identity)
	storeID := c.Param("id")

	meta, err := o.memManager.Get(storeID)
	if err != nil {
		if errors.Is(err, memory.ErrNotFound) {
			c.String(http.StatusNotFound, "store not found")
			return
		}
		c.String(http.StatusInternalServerError, "get store: %s", err.Error())
		return
	}
	if meta.CreatedBy != id.UserID {
		c.String(http.StatusForbidden, "only the store creator may update it")
		return
	}

	var req entity.UpdateMemoryStoreRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.String(http.StatusBadRequest, "bad request: %s", err.Error())
		return
	}

	// PATCH semantics: unset fields fall back to the current stored values so
	// the caller does not need to re-send fields it is not changing.
	if err := o.memManager.Update(storeID, memory.UpdateStoreRequest{
		Name:        coalesceStr(req.Name, meta.Name),
		Description: coalesceStr(req.Description, meta.Description),
		Visibility:  coalesceStr(req.Visibility, meta.Visibility),
		WritePolicy: coalesceStr(req.WritePolicy, meta.WritePolicy),
	}); err != nil {
		c.String(http.StatusBadRequest, "%s", err.Error())
		return
	}
	c.Status(http.StatusNoContent)
}

func (o *HTTPOrchestrator) handleDeleteMemoryStore(c *gin.Context) {
	if o.memManager == nil {
		c.String(http.StatusServiceUnavailable, "memory system disabled")
		return
	}
	id := c.MustGet(identityKey).(Identity)
	storeID := c.Param("id")

	meta, err := o.memManager.Get(storeID)
	if err != nil {
		if errors.Is(err, memory.ErrNotFound) {
			c.String(http.StatusNotFound, "store not found")
			return
		}
		c.String(http.StatusInternalServerError, "get store: %s", err.Error())
		return
	}
	if meta.CreatedBy != id.UserID {
		c.String(http.StatusForbidden, "only the store creator may delete it")
		return
	}

	if err := o.memManager.Delete(storeID); err != nil {
		c.String(http.StatusInternalServerError, "delete store: %s", err.Error())
		return
	}
	c.Status(http.StatusNoContent)
}
