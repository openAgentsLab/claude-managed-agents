package orchestration

import (
	"context"
	"log/slog"
	"os"
	"sync"
	"time"

	"forge/internal/harness"
	anthropicprovider "forge/internal/provider/anthropic"
)

// jailbreakRecord tracks repeated jailbreak attempts for one user.
type jailbreakRecord struct {
	count   int
	resetAt time.Time
}

// jailbreakRateLimiter is a simple in-memory per-user counter that blocks users
// who repeatedly trigger jailbreak detection (regex or LLM classifier).
// Counts reset after windowDur; users are blocked when count >= maxAttempts.
type jailbreakRateLimiter struct {
	mu          sync.Mutex
	records     map[string]*jailbreakRecord
	maxAttempts int
	windowDur   time.Duration
}

func newJailbreakRateLimiter() jailbreakRateLimiter {
	return jailbreakRateLimiter{
		records:     make(map[string]*jailbreakRecord),
		maxAttempts: 5,
		windowDur:   1 * time.Hour,
	}
}

// Record increments the attempt counter for userID and returns whether the
// user is now blocked (i.e. has reached maxAttempts within the current window).
func (l *jailbreakRateLimiter) Record(userID string) (blocked bool) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	rec, ok := l.records[userID]
	if !ok || now.After(rec.resetAt) {
		l.records[userID] = &jailbreakRecord{count: 1, resetAt: now.Add(l.windowDur)}
		return false
	}
	rec.count++
	return rec.count >= l.maxAttempts
}

// IsBlocked reports whether userID is currently rate-limited without
// incrementing the counter.
func (l *jailbreakRateLimiter) IsBlocked(userID string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	rec, ok := l.records[userID]
	if !ok {
		return false
	}
	if time.Now().After(rec.resetAt) {
		delete(l.records, userID)
		return false
	}
	return rec.count >= l.maxAttempts
}

// buildSafetyClassifier creates a Haiku-backed SafetyClassifier when an
// Anthropic API key is available.
//
// Key resolution order:
//  1. ANTHROPIC_API_KEY environment variable (dev / single-tenant)
//
// In multi-tenant production deployments the per-tenant API key is stored
// in the encrypted vault and not accessible at startup time. Operators that
// want per-tenant classifiers should call harness.NewSafetyClassifier with
// the tenant model at request time. This startup-time variant provides a
// best-effort global classifier for single-tenant and development scenarios.
//
// Returns nil when no key is available — callers must nil-check before use.
func buildSafetyClassifier(ctx context.Context) *harness.SafetyClassifier {
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		return nil
	}
	haikuModel, err := anthropicprovider.NewChatModel(ctx, anthropicprovider.Config{
		APIKey:    apiKey,
		Model:     "claude-haiku-4-5-20251001",
		MaxTokens: 128, // keep classification responses short for low latency
	})
	if err != nil {
		slog.WarnContext(ctx, "safety: failed to create Haiku classifier model", "error", err)
		return nil
	}
	return harness.NewSafetyClassifier(haikuModel)
}
