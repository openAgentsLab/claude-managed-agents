package entity

import (
	"time"

	appstore "forge/internal/gateway/store"
)

// ── Project ──────────────────────────────────────────────────────────────────

// GitConfigRequest carries Git repository settings in project create/update requests.
// Token is write-only: provide to set or rotate; omit to keep the existing value.
type GitConfigRequest struct {
	URL      string `json:"url,omitempty"`
	Branch   string `json:"branch,omitempty"`
	Username string `json:"username,omitempty"`
	Token    string `json:"token,omitempty"`
}

// GitConfigResponse is the read-only git config in project responses (Token omitted).
type GitConfigResponse struct {
	URL      string `json:"url,omitempty"`
	Branch   string `json:"branch,omitempty"`
	Username string `json:"username,omitempty"`
}

// ProjectRequest is the payload for POST /v1/projects and PUT /v1/projects/:id.
type ProjectRequest struct {
	Name          string             `json:"name"`
	Description   string             `json:"description,omitempty"`
	GitConfig     GitConfigRequest   `json:"git,omitempty"`
	EnvironmentID string             `json:"environment_id,omitempty"`
	RefFiles      []appstore.RefFile `json:"ref_files,omitempty"`
	Env           map[string]string  `json:"env,omitempty"`
}

// ProjectResponse is the resource body returned for project read/create/update.
type ProjectResponse struct {
	ID            string             `json:"id"`
	Name          string             `json:"name"`
	Description   string             `json:"description,omitempty"`
	GitConfig     GitConfigResponse  `json:"git,omitempty"`
	EnvironmentID string             `json:"environment_id,omitempty"`
	RefFiles      []appstore.RefFile `json:"ref_files,omitempty"`
	Env           map[string]string  `json:"env,omitempty"`
	CreatedAt     time.Time          `json:"created_at"`
	UpdatedAt     time.Time          `json:"updated_at"`
}

// ── Environment ──────────────────────────────────────────────────────────────

// EnvironmentRequest is the payload for create/update environment endpoints.
type EnvironmentRequest struct {
	Name        string                     `json:"name"`
	Description string                     `json:"description,omitempty"`
	Packages    appstore.PackageList       `json:"packages,omitempty"`
	Networking  *appstore.NetworkingConfig `json:"networking,omitempty"`
	Env         map[string]string          `json:"env,omitempty"`
}

// EnvironmentResponse is the resource body returned for environment read/create/update.
type EnvironmentResponse struct {
	ID          string                    `json:"id"`
	Scope       string                    `json:"scope"`
	Name        string                    `json:"name"`
	Description string                    `json:"description,omitempty"`
	Packages    appstore.PackageList      `json:"packages,omitempty"`
	Networking  appstore.NetworkingConfig `json:"networking"`
	Env         map[string]string         `json:"env,omitempty"`
	CreatedAt   time.Time                 `json:"created_at"`
	UpdatedAt   time.Time                 `json:"updated_at"`
}

// ── Resources ────────────────────────────────────────────────────────────────

// AddResourceRequest is the payload for POST /v1/sessions/:id/resources.
// Type must be ResourceTypeFile or ResourceTypeGit.
type AddResourceRequest struct {
	Type       string `json:"type"`
	TargetPath string `json:"target_path"`
	// File resource: provide exactly one of ContentBase64 or SourceURL.
	ContentBase64 string `json:"content_base64,omitempty"`
	SourceURL     string `json:"source_url,omitempty"`
	// Git resource: URL and TargetPath are required; Branch and Token are optional.
	URL    string `json:"url,omitempty"`
	Branch string `json:"branch,omitempty"`
	Token  string `json:"token,omitempty"`
}

// AddResourceResponse is the body returned on a successful
// POST /v1/sessions/:id/resources.
type AddResourceResponse struct {
	ResourceID string `json:"resource_id"`
}

// ResourceListItem is one entry in GET /v1/sessions/:id/resources.
type ResourceListItem struct {
	ID         string `json:"id"`
	Type       string `json:"type"`
	TargetPath string `json:"target_path"`
	URL        string `json:"url,omitempty"`    // git only
	Branch     string `json:"branch,omitempty"` // git only
	CreatedAt  int64  `json:"created_at"`
}

// ── Memory stores ─────────────────────────────────────────────────────────────

// CreateMemoryStoreRequest is the payload for POST /v1/memory-stores.
type CreateMemoryStoreRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Visibility  string `json:"visibility,omitempty"`
	WritePolicy string `json:"write_policy,omitempty"`
}

// UpdateMemoryStoreRequest is the payload for PATCH /v1/memory-stores/:id.
// All fields use PATCH semantics: unset fields retain the current stored value.
type UpdateMemoryStoreRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Visibility  string `json:"visibility"`
	WritePolicy string `json:"write_policy"`
}
