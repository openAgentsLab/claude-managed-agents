package orchestration

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"forge/internal/entity"
	"forge/internal/hands"
	"forge/internal/resources"
)

func (o *HTTPOrchestrator) handleListResources(c *gin.Context) {
	id := c.MustGet(identityKey).(Identity)
	clientID := c.Param("id")
	if clientID == "" {
		c.String(http.StatusBadRequest, "missing session id")
		return
	}
	internalSID := scopedSessionID(id.UserID, clientID)

	recs, err := o.tenantStore.SessionResources().List(c.Request.Context(), internalSID)
	if err != nil {
		c.String(http.StatusInternalServerError, "list resources: %s", err.Error())
		return
	}

	items := make([]entity.ResourceListItem, 0, len(recs))
	for _, r := range recs {
		item := entity.ResourceListItem{
			ID:         r.ID,
			Type:       r.Type,
			TargetPath: r.TargetPath,
			CreatedAt:  r.CreatedAt,
		}
		if r.Type == entity.ResourceTypeGit {
			var spec struct {
				URL    string `json:"url"`
				Branch string `json:"branch"`
			}
			_ = json.Unmarshal([]byte(r.Spec), &spec)
			item.URL = spec.URL
			item.Branch = spec.Branch
		}
		items = append(items, item)
	}
	c.JSON(http.StatusOK, items)
}

func (o *HTTPOrchestrator) handleAddResource(c *gin.Context) {
	id := c.MustGet(identityKey).(Identity)

	clientID := c.Param("id")
	if clientID == "" {
		c.String(http.StatusBadRequest, "missing session id")
		return
	}
	internalSID := scopedSessionID(id.UserID, clientID)

	rm, ok := o.sandboxPool.(hands.ResourceManager)
	if !ok {
		c.JSON(http.StatusServiceUnavailable, entity.ErrorResponse{Error: "resource management not supported by this sandbox driver"})
		return
	}

	var req entity.AddResourceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.String(http.StatusBadRequest, "bad request: %s", err.Error())
		return
	}

	resourceID := newResourceID()

	switch req.Type {
	case entity.ResourceTypeFile:
		if req.TargetPath == "" {
			c.String(http.StatusBadRequest, "target_path is required")
			return
		}
		if req.SourceURL != "" && req.ContentBase64 != "" {
			c.String(http.StatusBadRequest, "provide source_url or content_base64, not both")
			return
		}
		if req.SourceURL == "" && req.ContentBase64 == "" {
			c.String(http.StatusBadRequest, "source_url or content_base64 is required")
			return
		}
		r := resources.FileResource{
			ID:         resourceID,
			TargetPath: req.TargetPath,
			SourceURL:  req.SourceURL,
		}
		if req.ContentBase64 != "" {
			content, err := base64.StdEncoding.DecodeString(req.ContentBase64)
			if err != nil {
				c.String(http.StatusBadRequest, "invalid content_base64: %s", err.Error())
				return
			}
			r.Content = content
		}
		if err := rm.AddFileResource(c.Request.Context(), internalSID, r); err != nil {
			handleResourceError(c, err)
			return
		}

	case entity.ResourceTypeGit:
		if req.URL == "" || req.TargetPath == "" {
			c.String(http.StatusBadRequest, "url and target_path are required")
			return
		}
		r := resources.GitResource{
			ID:         resourceID,
			URL:        req.URL,
			Branch:     req.Branch,
			TargetPath: req.TargetPath,
			Token:      req.Token,
		}
		if err := rm.AddGitResource(c.Request.Context(), internalSID, r); err != nil {
			handleResourceError(c, err)
			return
		}

	default:
		c.String(http.StatusBadRequest, "type must be '%s' or '%s'", entity.ResourceTypeFile, entity.ResourceTypeGit)
		return
	}

	c.JSON(http.StatusCreated, entity.AddResourceResponse{ResourceID: resourceID})
}

func (o *HTTPOrchestrator) handleRemoveResource(c *gin.Context) {
	id := c.MustGet(identityKey).(Identity)

	clientID := c.Param("id")
	resourceID := c.Param("rid")
	if clientID == "" || resourceID == "" {
		c.String(http.StatusBadRequest, "missing session id or resource id")
		return
	}
	internalSID := scopedSessionID(id.UserID, clientID)

	rm, ok := o.sandboxPool.(hands.ResourceManager)
	if !ok {
		c.JSON(http.StatusServiceUnavailable, entity.ErrorResponse{Error: "resource management not supported by this sandbox driver"})
		return
	}

	if err := rm.RemoveResource(c.Request.Context(), internalSID, resourceID); err != nil {
		handleResourceError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func handleResourceError(c *gin.Context, err error) {
	if errors.Is(err, hands.ErrSharedStorageUnavailable) {
		c.JSON(http.StatusServiceUnavailable, entity.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusInternalServerError, entity.ErrorResponse{Error: err.Error()})
}

func newResourceID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
