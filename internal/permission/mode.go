package permission

// Mode controls the global permission posture.
type Mode string

const (
	// ModeDefault is the standard mode. All tools are allowed unless an
	// explicit deny rule blocks them.
	ModeDefault Mode = "default"

	// ModePlan is a read-only mode. ReadOnly tools are allowed automatically;
	// any tool marked as writable is denied.
	ModePlan Mode = "plan"
)
