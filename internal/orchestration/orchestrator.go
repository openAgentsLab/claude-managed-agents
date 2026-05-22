// Package orchestration implements the Orchestration layer from the
// managed-agents architecture: wake(session_id) → void.
package orchestration

import "context"

// Orchestrator corresponds to the article's wake(session_id) interface.
// Run blocks until ctx is cancelled or the orchestration source closes.
type Orchestrator interface {
	Run(ctx context.Context) error
}
