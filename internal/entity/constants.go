// Package entity defines all HTTP API request/response types and shared
// constants used across the orchestration layer.
package entity

// User roles.
const (
	RoleAdmin  = "admin"
	RoleMember = "member"
	RoleViewer = "viewer"
)

// Permission mode values for the run endpoint.
const (
	PermissionModeDefault = "default"
	PermissionModePlan    = "plan"
)

// Session resource types.
const (
	ResourceTypeFile = "file"
	ResourceTypeGit  = "git"
)

// MCP server transport types.
const (
	MCPTypeStdio = "stdio"
	MCPTypeSSE   = "sse"
	MCPTypeHTTP  = "http"
	MCPTypeWS    = "ws"
)

// TimeFormatISO8601 is the timestamp format used in all API responses.
const TimeFormatISO8601 = "2006-01-02T15:04:05Z"

// AuthBearerPrefix is the expected prefix for the Authorization header value.
const AuthBearerPrefix = "Bearer "

// ContentTypeSSE is the MIME type for Server-Sent Events streams.
const ContentTypeSSE = "text/event-stream"

// Memory store visibility scopes.
const MemoryVisibilitySharedTenant = "shared_tenant"

// Outcome iteration limits for the define_outcome cycle.
const (
	DefaultOutcomeMaxIterations = 3
	MaxOutcomeMaxIterations     = 20
)
