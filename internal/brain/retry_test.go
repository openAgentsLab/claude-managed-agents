package brain

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

// mockStatusErr implements StatusCode() int so extractStatusCode can read it.
type mockStatusErr struct {
	code int
	msg  string
}

func (e *mockStatusErr) Error() string   { return e.msg }
func (e *mockStatusErr) StatusCode() int { return e.code }

// mockHeaderErr additionally implements Header(key) string for retryAfterDelay.
type mockHeaderErr struct {
	code       int
	retryAfter string
}

func (e *mockHeaderErr) Error() string           { return fmt.Sprintf("status %d", e.code) }
func (e *mockHeaderErr) StatusCode() int         { return e.code }
func (e *mockHeaderErr) Header(key string) string {
	if key == "Retry-After" {
		return e.retryAfter
	}
	return ""
}

// ── callWithRetry ─────────────────────────────────────────────────────────

func TestCallWithRetry_SuccessOnFirstCall(t *testing.T) {
	calls := 0
	err := callWithRetry(context.Background(), 3, func() error {
		calls++
		return nil
	})
	if err != nil {
		t.Errorf("expected nil, got %v", err)
	}
	if calls != 1 {
		t.Errorf("expected 1 call, got %d", calls)
	}
}

func TestCallWithRetry_NonRetryableError_ReturnsImmediately(t *testing.T) {
	// Any error whose StatusCode() is 0 (or not implemented) hits the default
	// branch and is returned immediately without retry.
	sentinel := errors.New("some non-retryable failure")
	calls := 0

	err := callWithRetry(context.Background(), 5, func() error {
		calls++
		return sentinel
	})
	if !errors.Is(err, sentinel) {
		t.Errorf("expected sentinel error, got %v", err)
	}
	if calls != 1 {
		t.Errorf("expected exactly 1 call (no retry for non-retryable), got %d", calls)
	}
}

func TestCallWithRetry_ZeroMaxRetries_NeverCallsFn(t *testing.T) {
	calls := 0
	err := callWithRetry(context.Background(), 0, func() error {
		calls++
		return nil
	})
	// Loop body never executes → falls through to "max retries exceeded"
	if err == nil {
		t.Error("expected error for maxRetries=0, got nil")
	}
	if calls != 0 {
		t.Errorf("fn should not be called when maxRetries=0, got %d calls", calls)
	}
}

func TestCallWithRetry_429_ContextCancelledDuringSleep(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	calls := 0
	done := make(chan error, 1)
	go func() {
		done <- callWithRetry(ctx, 10, func() error {
			calls++
			return &mockStatusErr{code: 429, msg: "rate limit"}
		})
	}()

	// Wait briefly for the goroutine to enter sleepCtx, then cancel.
	time.Sleep(15 * time.Millisecond)
	cancel()

	err := <-done
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled after cancellation, got %v", err)
	}
	if calls != 1 {
		t.Errorf("expected fn called once before context cancel, got %d", calls)
	}
}

func TestCallWithRetry_529_ContextCancelledDuringSleep(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() {
		done <- callWithRetry(ctx, 10, func() error {
			return &mockStatusErr{code: 529, msg: "overloaded"}
		})
	}()

	time.Sleep(15 * time.Millisecond)
	cancel()

	err := <-done
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled for 529 + cancel, got %v", err)
	}
}

func TestCallWithRetry_AlreadyCancelledContext_429(t *testing.T) {
	// If the context is already cancelled before the sleep, sleepCtx
	// returns immediately with ctx.Err().
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	calls := 0
	err := callWithRetry(ctx, 5, func() error {
		calls++
		return &mockStatusErr{code: 429, msg: "rate limit"}
	})
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
	if calls != 1 {
		t.Errorf("expected 1 call before immediate sleep cancellation, got %d", calls)
	}
}

// ── exponentialBackoff ────────────────────────────────────────────────────

func TestExponentialBackoff_Values(t *testing.T) {
	cases := []struct {
		attempt int
		want    time.Duration
	}{
		{0, 1 * time.Second},
		{1, 2 * time.Second},
		{2, 4 * time.Second},
		{3, 8 * time.Second},
		{4, 16 * time.Second},
		{5, 30 * time.Second}, // 32s → capped at 30s
		{10, 30 * time.Second},
	}
	for _, tc := range cases {
		got := exponentialBackoff(tc.attempt)
		if got != tc.want {
			t.Errorf("exponentialBackoff(%d) = %v, want %v", tc.attempt, got, tc.want)
		}
	}
}

// ── extractStatusCode ─────────────────────────────────────────────────────

func TestExtractStatusCode_WithStatusCoder(t *testing.T) {
	for _, code := range []int{400, 429, 500, 529} {
		if got := extractStatusCode(&mockStatusErr{code: code}); got != code {
			t.Errorf("extractStatusCode(%d) = %d, want %d", code, got, code)
		}
	}
}

func TestExtractStatusCode_PlainError_ReturnsZero(t *testing.T) {
	if got := extractStatusCode(errors.New("plain")); got != 0 {
		t.Errorf("extractStatusCode(plain) = %d, want 0", got)
	}
}

func TestExtractStatusCode_Nil_ReturnsZero(t *testing.T) {
	if got := extractStatusCode(nil); got != 0 {
		t.Errorf("extractStatusCode(nil) = %d, want 0", got)
	}
}

// ── retryAfterDelay ───────────────────────────────────────────────────────

func TestRetryAfterDelay_WithValidHeader(t *testing.T) {
	err := &mockHeaderErr{code: 429, retryAfter: "5"}
	if d := retryAfterDelay(err); d != 5*time.Second {
		t.Errorf("retryAfterDelay = %v, want 5s", d)
	}
}

func TestRetryAfterDelay_WithDecimalHeader(t *testing.T) {
	err := &mockHeaderErr{code: 429, retryAfter: "2.5"}
	if d := retryAfterDelay(err); d != 2500*time.Millisecond {
		t.Errorf("retryAfterDelay = %v, want 2.5s", d)
	}
}

func TestRetryAfterDelay_NoHeaderProvider_ReturnsZero(t *testing.T) {
	// mockStatusErr does not implement headerProvider
	if d := retryAfterDelay(&mockStatusErr{code: 429}); d != 0 {
		t.Errorf("retryAfterDelay without header = %v, want 0", d)
	}
}

func TestRetryAfterDelay_InvalidHeaderValue_ReturnsZero(t *testing.T) {
	err := &mockHeaderErr{code: 429, retryAfter: "not-a-number"}
	if d := retryAfterDelay(err); d != 0 {
		t.Errorf("retryAfterDelay invalid header = %v, want 0", d)
	}
}

// ── sleepCtx ──────────────────────────────────────────────────────────────

func TestSleepCtx_CompletesNormally(t *testing.T) {
	if err := sleepCtx(context.Background(), time.Millisecond); err != nil {
		t.Errorf("expected nil for short sleep, got %v", err)
	}
}

func TestSleepCtx_AlreadyCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := sleepCtx(ctx, time.Hour); !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

func TestSleepCtx_DeadlineExceeded(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()
	time.Sleep(5 * time.Millisecond) // ensure deadline is past

	if err := sleepCtx(ctx, time.Hour); !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected context.DeadlineExceeded, got %v", err)
	}
}
