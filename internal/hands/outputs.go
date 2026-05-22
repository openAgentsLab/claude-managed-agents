package hands

import "context"

// OutputEntry describes a file in the session outputs directory.
type OutputEntry struct {
	Path string // relative path within the outputs directory
	Size int64
}

// OutputsProvider is an optional interface implemented by Pool drivers that
// support reading the session outputs directory. Use a type assertion:
//
//	if op, ok := pool.(hands.OutputsProvider); ok { ... }
type OutputsProvider interface {
	// ListOutputs returns all files in the session outputs directory.
	// Returns ErrSharedStorageUnavailable when the driver cannot access the
	// outputs directory (Docker degraded mode).
	ListOutputs(ctx context.Context, sessionID string) ([]OutputEntry, error)

	// ReadOutput returns the contents of the file at path (relative to the
	// outputs directory). Returns ErrSharedStorageUnavailable in degraded mode.
	ReadOutput(ctx context.Context, sessionID, path string) ([]byte, error)
}
