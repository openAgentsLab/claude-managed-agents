package harness

import (
	"encoding/json"
	"strings"
	"testing"
)

// ── extractJSON ───────────────────────────────────────────────────────────────

func TestExtractJSON_PlainJSON(t *testing.T) {
	input := `{"overall":"satisfied","criteria":[],"summary":"ok"}`
	got := extractJSON(input)
	if got != input {
		t.Errorf("expected unchanged, got %q", got)
	}
}

func TestExtractJSON_WrappedInProse(t *testing.T) {
	input := `Here is my evaluation: {"overall":"needs_revision","criteria":[],"summary":"not done"} Hope that helps.`
	got := extractJSON(input)
	if !strings.HasPrefix(got, "{") || !strings.HasSuffix(got, "}") {
		t.Errorf("extracted JSON should start with { and end with }, got: %q", got)
	}
	var result GraderResult
	if err := json.Unmarshal([]byte(got), &result); err != nil {
		t.Errorf("extracted JSON is not valid: %v", err)
	}
}

func TestExtractJSON_MarkdownCodeFence(t *testing.T) {
	input := "```json\n{\"overall\":\"satisfied\",\"criteria\":[],\"summary\":\"all good\"}\n```"
	got := extractJSON(input)
	var result GraderResult
	if err := json.Unmarshal([]byte(got), &result); err != nil {
		t.Errorf("failed to parse extracted JSON: %v (extracted: %q)", err, got)
	}
	if result.Overall != "satisfied" {
		t.Errorf("expected overall=satisfied, got %q", result.Overall)
	}
}

func TestExtractJSON_NoJSON(t *testing.T) {
	input := "I cannot determine the result."
	got := extractJSON(input)
	// Should return original when no JSON found
	if got != input {
		t.Errorf("no-JSON input should return original, got %q", got)
	}
}

// ── GraderResult JSON round-trip ─────────────────────────────────────────────

func TestCriterionFeedback_EvidenceField(t *testing.T) {
	cf := CriterionFeedback{
		Criterion: "file_created",
		Status:    "satisfied",
		Feedback:  "File /tmp/x.txt exists with correct content",
		Evidence:  "$ cat /tmp/x.txt\nhello world",
	}
	data, err := json.Marshal(cf)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var back CriterionFeedback
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if back.Evidence != cf.Evidence {
		t.Errorf("evidence round-trip failed: got %q, want %q", back.Evidence, cf.Evidence)
	}
}

func TestCriterionFeedback_EmptyEvidenceOmitted(t *testing.T) {
	cf := CriterionFeedback{
		Criterion: "task_complete",
		Status:    "satisfied",
		Feedback:  "done",
	}
	data, _ := json.Marshal(cf)
	if strings.Contains(string(data), `"evidence"`) {
		t.Errorf("empty evidence should be omitted from JSON, got: %s", data)
	}
}

func TestGraderResult_DefaultOverall(t *testing.T) {
	// When the model returns no overall field, it should default to needs_revision
	// in the RunGraderWithBrain fallback path — test that behaviour via the
	// struct zero-value (the actual default is applied by RunGraderWithBrain after
	// unmarshal, not by the struct itself).
	var r GraderResult
	if r.Overall != "" {
		t.Errorf("zero-value Overall should be empty string, got %q", r.Overall)
	}
	// Simulate the fallback applied in RunGraderWithBrain
	if r.Overall == "" {
		r.Overall = "needs_revision"
	}
	if r.Overall != "needs_revision" {
		t.Errorf("fallback should be needs_revision, got %q", r.Overall)
	}
}

// ── buildActiveGraderPrompt ───────────────────────────────────────────────────

func TestBuildActiveGraderPrompt_ContainsXMLTags(t *testing.T) {
	prompt := buildActiveGraderPrompt("check file exists", "agent said it created the file")

	required := []string{
		"<instructions>",
		"</instructions>",
		"<rubric>",
		"</rubric>",
		"<agent_work>",
		"</agent_work>",
		"check file exists",
		"agent said it created the file",
	}
	for _, r := range required {
		if !strings.Contains(prompt, r) {
			t.Errorf("prompt missing required string: %q", r)
		}
	}
}

func TestBuildActiveGraderPrompt_ContainsCoT(t *testing.T) {
	prompt := buildActiveGraderPrompt("x", "y")
	if !strings.Contains(prompt, "step by step") {
		t.Error("prompt should contain CoT instruction 'step by step'")
	}
}

func TestBuildActiveGraderPrompt_ContainsEvidenceField(t *testing.T) {
	prompt := buildActiveGraderPrompt("x", "y")
	if !strings.Contains(prompt, `"evidence"`) {
		t.Error("prompt JSON schema should include evidence field")
	}
}

func TestBuildActiveGraderPrompt_DoNotTrustClaims(t *testing.T) {
	prompt := buildActiveGraderPrompt("x", "y")
	if !strings.Contains(prompt, "Do NOT trust") {
		t.Error("prompt should instruct grader to not trust agent claims")
	}
}

// ── BuildRevisionMessage ──────────────────────────────────────────────────────

func TestBuildRevisionMessage_ContainsRubric(t *testing.T) {
	rubric := "file /tmp/out.txt must contain 'hello'"
	result := &GraderResult{
		Overall: "needs_revision",
		Summary: "file was not created",
		Criteria: []CriterionFeedback{
			{Criterion: "file_exists", Status: "needs_work", Feedback: "file not found"},
		},
	}
	msg := BuildRevisionMessage(rubric, result)

	if !strings.Contains(msg, rubric) {
		t.Error("revision message should contain the original rubric")
	}
	if !strings.Contains(msg, "file was not created") {
		t.Error("revision message should contain grader summary")
	}
	if !strings.Contains(msg, "file_exists") {
		t.Error("revision message should contain the failing criterion")
	}
	if !strings.Contains(msg, "file not found") {
		t.Error("revision message should contain criterion feedback")
	}
}

func TestBuildRevisionMessage_SkipsSatisfiedCriteria(t *testing.T) {
	result := &GraderResult{
		Overall: "needs_revision",
		Criteria: []CriterionFeedback{
			{Criterion: "done", Status: "satisfied", Feedback: "looks good"},
			{Criterion: "missing", Status: "needs_work", Feedback: "not done"},
		},
	}
	msg := BuildRevisionMessage("rubric text", result)

	// The satisfied criterion "done" should not appear in the failure list
	if strings.Contains(msg, "looks good") {
		t.Error("revision message should not include feedback for satisfied criteria")
	}
	if !strings.Contains(msg, "not done") {
		t.Error("revision message should include feedback for needs_work criteria")
	}
}
