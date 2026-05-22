package entity

import (
	"encoding/json"
	"strings"
	"testing"
)

// ── Constants ─────────────────────────────────────────────────────────────────

func TestRoleConstants(t *testing.T) {
	roles := []string{RoleAdmin, RoleMember, RoleViewer}
	seen := make(map[string]bool)
	for _, r := range roles {
		if r == "" {
			t.Error("role constants must not be empty")
		}
		if seen[r] {
			t.Errorf("duplicate role constant: %q", r)
		}
		seen[r] = true
	}
}

func TestPermissionModeConstants(t *testing.T) {
	if PermissionModeDefault == "" || PermissionModePlan == "" {
		t.Error("permission mode constants must not be empty")
	}
	if PermissionModeDefault == PermissionModePlan {
		t.Error("permission mode constants must be distinct")
	}
}

func TestResourceTypeConstants(t *testing.T) {
	if ResourceTypeFile == ResourceTypeGit {
		t.Error("resource type constants must be distinct")
	}
}

func TestMCPTypeConstants_Distinct(t *testing.T) {
	types := []string{MCPTypeStdio, MCPTypeSSE, MCPTypeHTTP, MCPTypeWS}
	seen := make(map[string]bool)
	for _, tt := range types {
		if tt == "" {
			t.Error("MCP type constant must not be empty")
		}
		if seen[tt] {
			t.Errorf("duplicate MCP type constant: %q", tt)
		}
		seen[tt] = true
	}
}

func TestOutcomeIterationLimits(t *testing.T) {
	if DefaultOutcomeMaxIterations <= 0 {
		t.Error("DefaultOutcomeMaxIterations must be positive")
	}
	if MaxOutcomeMaxIterations < DefaultOutcomeMaxIterations {
		t.Error("MaxOutcomeMaxIterations must be >= DefaultOutcomeMaxIterations")
	}
}

// ── AgentResponse JSON ────────────────────────────────────────────────────────

func TestAgentResponse_JSONRoundTrip(t *testing.T) {
	ag := AgentResponse{
		ID:             "agent-1",
		Name:           "test agent",
		Description:    "does stuff",
		Version:        3,
		Model:          "claude-sonnet-4-6",
		SkillNames:     []string{"skill-a"},
		MCPServerNames: []string{"fs"},
		IsDefault:      true,
		CreatedAt:      "2026-01-01T00:00:00Z",
		UpdatedAt:      "2026-05-01T00:00:00Z",
	}
	b, err := json.Marshal(ag)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got AgentResponse
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.ID != ag.ID || got.Name != ag.Name || got.Version != ag.Version {
		t.Errorf("round-trip mismatch: got %+v", got)
	}
}

// ── UpdateAgentRequest omitempty ──────────────────────────────────────────────

func TestUpdateAgentRequest_OmitEmpty(t *testing.T) {
	req := UpdateAgentRequest{}
	b, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(b), "name") {
		t.Errorf("nil pointer fields should be omitted; got %s", b)
	}
}

func TestUpdateAgentRequest_PointerFields(t *testing.T) {
	name := "updated-name"
	req := UpdateAgentRequest{Name: &name}
	b, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(b), "updated-name") {
		t.Errorf("expected name in JSON; got %s", b)
	}
}

// ── CreateAgentRequest ────────────────────────────────────────────────────────

func TestCreateAgentRequest_JSONRoundTrip(t *testing.T) {
	req := CreateAgentRequest{
		Name:           "my-agent",
		Model:          "claude-haiku-4-5-20251001",
		SystemPrompt:   "You are helpful.",
		MCPServerNames: []string{"fs", "git"},
		SkillNames:     []string{"code-review"},
	}
	b, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got CreateAgentRequest
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Name != req.Name || got.Model != req.Model {
		t.Errorf("round-trip mismatch: got %+v", got)
	}
	if len(got.MCPServerNames) != 2 || got.MCPServerNames[0] != "fs" {
		t.Errorf("MCPServerNames round-trip: got %v", got.MCPServerNames)
	}
}

// ── UpsertSkillRequest ────────────────────────────────────────────────────────

func TestUpsertSkillRequest_JSON(t *testing.T) {
	req := UpsertSkillRequest{Name: "my-skill", Content: "---\nname: my-skill\n---\nBody."}
	b, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got UpsertSkillRequest
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Name != req.Name || got.Content != req.Content {
		t.Errorf("round-trip mismatch")
	}
}

// ── AuthBearerPrefix ──────────────────────────────────────────────────────────

func TestAuthBearerPrefix(t *testing.T) {
	if !strings.HasPrefix("Bearer mytoken", AuthBearerPrefix) {
		t.Errorf("AuthBearerPrefix %q should be prefix of a Bearer token header", AuthBearerPrefix)
	}
}

// ── TimeFormatISO8601 ─────────────────────────────────────────────────────────

func TestTimeFormatISO8601(t *testing.T) {
	if TimeFormatISO8601 == "" {
		t.Error("TimeFormatISO8601 must not be empty")
	}
}
