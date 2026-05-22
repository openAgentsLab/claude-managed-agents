package harness

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"

	einocallbacks "forge/internal/eino/callbacks"
	"forge/internal/gateway/session"
	"forge/internal/reqctx"
)

// GraderResult is the structured output from a single grader evaluation.
type GraderResult struct {
	Overall  string              `json:"overall"`  // "satisfied" | "needs_revision"
	Criteria []CriterionFeedback `json:"criteria"`
	Summary  string              `json:"summary"`
}

// extractJSON finds the outermost { ... } block in s.
// Handles model responses that wrap JSON in prose or markdown.
func extractJSON(s string) string {
	start := strings.Index(s, "{")
	if start < 0 {
		return s
	}
	end := strings.LastIndex(s, "}")
	if end <= start {
		return s
	}
	return s[start : end+1]
}

// RunGraderWithBrain runs an active grader that uses tools to independently verify
// results against rubric. It executes inside a throwaway session so its tool events
// do not appear in the main session history.
// Falls back to treating as satisfied when h.brain is nil.
func (h *Harness) RunGraderWithBrain(ctx context.Context, rubric, conversation string) (*GraderResult, error) {
	if h.brain == nil {
		return nil, fmt.Errorf("grader: no brain configured")
	}

	// Throwaway session keeps tool callbacks working without polluting main history.
	graderSID := "grader-" + uuid.NewString()
	_ = h.store.CreateSession(session.Session{ID: graderSID, Status: session.SessionRunning})
	defer func() { _ = h.store.ClearSession(graderSID) }()

	graderCtx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()
	graderCtx = reqctx.WithPermissionMode(graderCtx, "plan")

	// Wire notification channels; drain and discard — grader tool events are internal.
	toolNotifCh := make(chan einocallbacks.ToolNotif, 32)
	spanNotifCh := make(chan einocallbacks.SpanNotif, 8)
	graderCtx = context.WithValue(graderCtx, einocallbacks.SessionIDKey{}, graderSID)
	graderCtx = context.WithValue(graderCtx, einocallbacks.ToolNotifKey{}, (chan<- einocallbacks.ToolNotif)(toolNotifCh))
	graderCtx = context.WithValue(graderCtx, einocallbacks.SpanNotifKey{}, (chan<- einocallbacks.SpanNotif)(spanNotifCh))
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); for range toolNotifCh {} }()
	go func() { defer wg.Done(); for range spanNotifCh {} }()
	defer func() {
		close(toolNotifCh)
		close(spanNotifCh)
		wg.Wait()
	}()

	prompt := buildActiveGraderPrompt(rubric, conversation)
	iter := h.brain.Run(graderCtx, prompt, nil)
	var sb strings.Builder
	for {
		ev, ok := iter.Next()
		if !ok {
			break
		}
		if ev == nil || ev.Err != nil {
			continue
		}
		if ev.Output == nil || ev.Output.MessageOutput == nil {
			continue
		}
		if ev.Output.MessageOutput.Role == schema.Tool {
			continue
		}
		c, msg := drainMessageVariant(ev.Output.MessageOutput, nil)
		if isPauseTurn(msg) {
			continue
		}
		sb.WriteString(c)
	}

	content := extractJSON(sb.String())
	var result GraderResult
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		return nil, fmt.Errorf("grader: parse response: %w (raw: %.200s)", err, sb.String())
	}
	if result.Overall == "" {
		result.Overall = "needs_revision"
	}
	return &result, nil
}

// buildActiveGraderPrompt constructs the verification prompt for a tool-enabled grader.
// Uses XML tags for structural clarity, CoT self-verification, and requires evidence
// quotes so the grader's own claims are traceable to actual tool output.
func buildActiveGraderPrompt(rubric, conversation string) string {
	return `You are an independent evaluator with tool access. Your job is to verify whether the agent's work satisfies each criterion in the rubric.

<instructions>
CRITICAL: Do NOT trust the agent's claims. Use your tools to independently verify every criterion:
- Run test suites, linters, or build commands to check actual state
- Read files directly to verify that changes were made correctly
- Check conditions yourself rather than relying on what the agent reported
- Think step by step for each criterion: (1) state what you expect, (2) run the tool to check, (3) compare result to expectation
- For each judgment, quote the specific tool output that supports it in the "evidence" field
- If you cannot find tool output that supports a "satisfied" judgment, mark it "needs_work" instead
</instructions>

<rubric>
` + rubric + `
</rubric>

<agent_work>
` + conversation + `
</agent_work>

After completing all verifications with tools, output ONLY valid JSON with no text outside it:
{
  "overall": "satisfied" or "needs_revision",
  "criteria": [
    {
      "criterion": "<criterion name from rubric>",
      "status": "satisfied" or "needs_work",
      "feedback": "<what you verified and what you found>",
      "evidence": "<verbatim excerpt from tool output that supports this judgment — empty string if not verifiable>"
    }
  ],
  "summary": "<brief assessment based on your verification>"
}`
}

// BuildRevisionMessage produces the next agent prompt when the grader reports needs_revision.
// Injects the original rubric and per-criterion feedback so the agent knows exactly what to fix.
func BuildRevisionMessage(rubric string, result *GraderResult) string {
	var sb strings.Builder
	sb.WriteString("The previous attempt did not fully satisfy the acceptance criteria. Please revise your work.\n\n")
	if result.Summary != "" {
		sb.WriteString("**Overall feedback:** ")
		sb.WriteString(result.Summary)
		sb.WriteString("\n\n")
	}
	sb.WriteString("**Per-criterion feedback:**\n")
	for _, c := range result.Criteria {
		if c.Status == "needs_work" || c.Status == "" {
			sb.WriteString("- **")
			sb.WriteString(c.Criterion)
			sb.WriteString("**: ")
			sb.WriteString(c.Feedback)
			sb.WriteString("\n")
		}
	}
	sb.WriteString("\n**Acceptance criteria (rubric):**\n")
	sb.WriteString(rubric)
	return sb.String()
}
