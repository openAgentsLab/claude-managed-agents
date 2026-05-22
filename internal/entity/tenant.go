package entity

import (
	"forge/internal/config"
	appstore "forge/internal/gateway/store"
)

// TenantInfoResponse is the body returned by GET /v1/tenant.
type TenantInfoResponse struct {
	ID       string                `json:"id"`
	Name     string                `json:"name"`
	Role     string                `json:"role"`
	Settings config.TenantSettings `json:"settings"`
}

// UpdateSettingsRequest is the payload for PATCH /admin/v1/tenant/settings.
// Nil pointer fields are treated as "no change"; send an explicit value to overwrite.
type UpdateSettingsRequest struct {
	AllowRules    []string              `json:"allow_rules,omitempty"`
	DenyRules     []string              `json:"deny_rules,omitempty"`
	ResourceQuota *config.ResourceQuota `json:"resource_quota,omitempty"`
	// ModelOverride replaces the tenant's model config. Send null to clear.
	ModelOverride *config.ModelOverride `json:"model,omitempty"`
	// BrainOverride replaces the tenant's inference config. Send null to clear.
	BrainOverride *config.BrainOverride `json:"brain,omitempty"`
}

// UserInfoResponse is one entry in the list returned by
// GET /admin/v1/tenant/users.
type UserInfoResponse struct {
	Username string `json:"username"`
	Role     string `json:"role"`
}

// UpdateRoleRequest is the payload for PATCH /admin/v1/tenant/users/:username.
type UpdateRoleRequest struct {
	Role string `json:"role"`
}

// CreateUserRequest is the payload for POST /admin/v1/tenant/users.
type CreateUserRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	// Role defaults to RoleMember when omitted.
	Role string `json:"role,omitempty"`
}

// UpdateUserSettingsRequest is the payload for PATCH /v1/user/settings.
// Nil pointer fields are treated as "no change".
type UpdateUserSettingsRequest struct {
	// ModelOverride replaces the user's model config.
	// Send {} (all fields empty) to inherit from the tenant/global config.
	ModelOverride *appstore.ModelSettings `json:"model,omitempty"`
	// BrainOverride replaces the user's inference config.
	BrainOverride *appstore.BrainSettings `json:"brain,omitempty"`
}
