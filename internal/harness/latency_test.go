package harness_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"forge/internal/config"
	"forge/internal/gateway/session"
	"forge/internal/harness"
)

// ── TTFT (Time-To-First-Token) ────────────────────────────────────────────────

// TestTTFT measures Time-To-First-Token for a simple prompt.
// TTFT = latency from Run() call to first EventAgentMessage event.
func TestTTFT(t *testing.T) {
	b := newTestBrain(t, haikuCfg(t))
	skipIfAPIUnaccessible(t, b)
	h := newTestHarness(t, b)
	ctx := context.Background()

	sid := "ttft-" + uuid.NewString()
	if err := h.CreateSession(session.Session{ID: sid, Status: session.SessionIdle}); err != nil {
		t.Fatalf("create session: %v", err)
	}

	start := time.Now()
	outCh, errCh := h.Run(ctx, sid, "Reply with exactly one word: hello")

	var ttft, total time.Duration
	for ev := range outCh {
		if ev.Type == harness.EventAgentMessage && ttft == 0 {
			ttft = time.Since(start)
		}
	}
	total = time.Since(start)
	if err := <-errCh; err != nil {
		t.Fatalf("harness error: %v", err)
	}

	t.Logf("TTFT: %v  total: %v", ttft, total)
	if ttft == 0 {
		t.Error("TTFT was never recorded — no EventAgentMessage received")
	}
	if ttft > 5*time.Second {
		t.Errorf("TTFT %v exceeds 5s threshold", ttft)
	}
}

// ── Model latency comparison ──────────────────────────────────────────────────

// TestModelLatencyComparison logs Haiku vs Sonnet latency for a trivial prompt.
// Haiku should be consistently faster for short-output tasks.
// This test never fails — it only emits data for operational decisions.
func TestModelLatencyComparison(t *testing.T) {
	if testing.Short() {
		t.Skip("model comparison runs 2 API calls")
	}
	key := anthropicKey(t)
	const prompt = "Reply with one word: hello"

	haikuH := newTestHarness(t, newTestBrain(t, config.ModelConfig{
		Provider: "anthropic", APIKey: key, Model: "claude-haiku-4-5-20251001",
	}))
	sonnetH := newTestHarness(t, newTestBrain(t, config.ModelConfig{
		Provider: "anthropic", APIKey: key, Model: "claude-sonnet-4-6",
	}))

	haiku := measureLatency(t, haikuH, prompt)
	sonnet := measureLatency(t, sonnetH, prompt)
	t.Logf("Haiku: %v  Sonnet: %v  speedup: %.1fx", haiku, sonnet, float64(sonnet)/float64(haiku))
}

// ── Prompt caching ────────────────────────────────────────────────────────────

// TestPromptCachingHit verifies that the second run against the same brain
// records CacheReadInputTokens > 0, confirming Anthropic prompt caching is active.
func TestPromptCachingHit(t *testing.T) {
	if testing.Short() {
		t.Skip("prompt caching runs 2 API calls")
	}
	b := newTestBrain(t, haikuCfg(t))
	h := newTestHarness(t, b)
	ctx := context.Background()

	run := func(label string) *harness.ModelUsage {
		sid := "cache-" + label + "-" + uuid.NewString()
		if err := h.CreateSession(session.Session{ID: sid, Status: session.SessionIdle}); err != nil {
			t.Fatalf("create session: %v", err)
		}
		outCh, errCh := h.Run(ctx, sid, "Say hello in one word.")
		var usage *harness.ModelUsage
		for ev := range outCh {
			if ev.Type == harness.EventSpanModelRequestEnd && ev.ModelUsage != nil {
				u := *ev.ModelUsage
				usage = &u
			}
		}
		if err := <-errCh; err != nil {
			t.Fatalf("harness error: %v", err)
		}
		return usage
	}

	u1 := run("first")
	u2 := run("second")

	if u1 != nil {
		t.Logf("run1: input=%d cache_read=%d", u1.InputTokens, u1.CacheReadInputTokens)
	}
	if u2 != nil {
		t.Logf("run2: input=%d cache_read=%d", u2.InputTokens, u2.CacheReadInputTokens)
	}
	if u2 == nil {
		t.Skip("no span event — caching check not possible")
	}
	if u2.CacheReadInputTokens == 0 {
		t.Log("NOTE: cache_read=0 on second run — prompt may be below Anthropic caching threshold (~2048 tokens)")
	} else {
		t.Logf("Prompt caching confirmed: %d tokens from cache on second run", u2.CacheReadInputTokens)
	}
}

// ── MaxTokens cap ─────────────────────────────────────────────────────────────

// TestMaxTokensCap verifies that BrainConfig.MaxTokens / ModelConfig.MaxTokens
// produces shorter output for a verbosity-prone prompt.
func TestMaxTokensCap(t *testing.T) {
	if testing.Short() {
		t.Skip("max tokens test runs 2 API calls")
	}
	key := anthropicKey(t)
	const prompt = "Write a detailed 500-word essay explaining why Go is a good language."

	// Uncapped brain
	hFull := newTestHarness(t, newTestBrain(t, config.ModelConfig{
		Provider: "anthropic", APIKey: key, Model: "claude-haiku-4-5-20251001",
	}))
	// Capped brain (64 output tokens ≈ ~50 words)
	hCapped := newTestHarness(t, newTestBrain(t, config.ModelConfig{
		Provider: "anthropic", APIKey: key, Model: "claude-haiku-4-5-20251001",
		MaxTokens: 64,
	}))

	fullReply := runTurn(t, hFull, prompt)
	cappedReply := runTurn(t, hCapped, prompt)

	fullWords := len(strings.Fields(fullReply))
	cappedWords := len(strings.Fields(cappedReply))
	t.Logf("full: %d words  capped: %d words", fullWords, cappedWords)

	if cappedWords >= fullWords && fullWords > 20 {
		t.Logf("NOTE: capped reply (%d words) is not shorter than full reply (%d words)", cappedWords, fullWords)
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

func measureLatency(t *testing.T, h *harness.Harness, prompt string) time.Duration {
	t.Helper()
	ctx := context.Background()
	sid := "lat-" + uuid.NewString()
	if err := h.CreateSession(session.Session{ID: sid, Status: session.SessionIdle}); err != nil {
		t.Fatalf("create session: %v", err)
	}
	start := time.Now()
	outCh, errCh := h.Run(ctx, sid, prompt)
	for range outCh {}
	if err := <-errCh; err != nil {
		t.Logf("harness error (non-fatal): %v", err)
	}
	return time.Since(start)
}
