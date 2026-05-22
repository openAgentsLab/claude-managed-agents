package brain

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"time"
)

// callWithRetry retries fn on 429 (rate_limit_error) and 529 (overloaded_error).
// All other errors are returned immediately without retry.
// Docs: API Errors (api/errors)
func callWithRetry(ctx context.Context, maxRetries int, fn func() error) error {
	for attempt := 0; attempt < maxRetries; attempt++ {
		err := fn()
		if err == nil {
			return nil
		}
		statusCode := extractStatusCode(err)
		switch statusCode {
		case http.StatusTooManyRequests: // 429 rate_limit_error
			// Prefer retry-after header; fall back to exponential backoff.
			delay := retryAfterDelay(err)
			if delay == 0 {
				delay = exponentialBackoff(attempt)
			}
			if sleepCtx(ctx, delay) != nil {
				return ctx.Err()
			}
		case 529: // overloaded_error (Anthropic custom status code)
			if sleepCtx(ctx, exponentialBackoff(attempt)) != nil {
				return ctx.Err()
			}
		default:
			return err // non-retryable error
		}
	}
	return fmt.Errorf("brain: max retries (%d) exceeded", maxRetries)
}

// exponentialBackoff returns 1s * 2^attempt, capped at 30s.
func exponentialBackoff(attempt int) time.Duration {
	d := time.Duration(math.Pow(2, float64(attempt))) * time.Second
	if d > 30*time.Second {
		d = 30 * time.Second
	}
	return d
}

// sleepCtx sleeps for d or returns ctx.Err() if context is cancelled first.
func sleepCtx(ctx context.Context, d time.Duration) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(d):
		return nil
	}
}

// extractStatusCode attempts to extract an HTTP status code from an error.
// Returns 0 if the error does not carry an HTTP status code.
func extractStatusCode(err error) int {
	if err == nil {
		return 0
	}
	// Try type-asserting to a common error interface that exposes StatusCode.
	type statusCoder interface {
		StatusCode() int
	}
	if sc, ok := err.(statusCoder); ok {
		return sc.StatusCode()
	}
	return 0
}

// retryAfterDelay reads the Retry-After value from an error (if the error
// type carries HTTP response headers). Returns 0 if unavailable.
func retryAfterDelay(err error) time.Duration {
	type headerProvider interface {
		Header(key string) string
	}
	if hp, ok := err.(headerProvider); ok {
		if s := hp.Header("Retry-After"); s != "" {
			if secs, err := strconv.ParseFloat(s, 64); err == nil {
				return time.Duration(secs * float64(time.Second))
			}
		}
	}
	return 0
}
