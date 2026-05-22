package hands

import (
	"context"
	"errors"

	"forge/internal/resources"
)

// ErrSharedStorageUnavailable is returned by ResourceManager methods when the
// pool requires volumes_root for the operation but it is not configured.
var ErrSharedStorageUnavailable = errors.New("shared storage unavailable: sandbox.volumes_root is not set")

// ResourceManager is an optional interface implemented by Pool drivers that
// support dynamic resource mounting (FileResource, GitResource add/remove).
// Use a type assertion to check whether a Pool supports it:
//
//	if rm, ok := pool.(ResourceManager); ok { ... }
type ResourceManager interface {
	// AddFileResource writes the file content to the session workspace and
	// persists the resource declaration (without content) to DB so it can be
	// re-materialised after a container rebuild.
	AddFileResource(ctx context.Context, sessionID string, r resources.FileResource) error

	// AddGitResource clones the git repository into the session workspace on
	// the orchestration side (token never enters the container). The declaration
	// is persisted to DB without the token.
	AddGitResource(ctx context.Context, sessionID string, r resources.GitResource) error

	// RemoveResource deletes the resource with the given ID from the workspace
	// and removes its declaration from DB.
	RemoveResource(ctx context.Context, sessionID, resourceID string) error
}
