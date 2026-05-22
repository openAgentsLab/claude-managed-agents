package memory

import "context"

type sessionStoresKey struct{}
type systemContextKey struct{}

// WithSessionStores attaches a SessionStores to the context.
func WithSessionStores(ctx context.Context, ss *SessionStores) context.Context {
	return context.WithValue(ctx, sessionStoresKey{}, ss)
}

// SessionStoresFromContext retrieves SessionStores from the context.
// Returns nil when none is attached.
func SessionStoresFromContext(ctx context.Context) *SessionStores {
	ss, _ := ctx.Value(sessionStoresKey{}).(*SessionStores)
	return ss
}

// WithSystemContext attaches a memory system-prompt section to the context.
// Brain reads this via SystemContextFromContext to prepend it to the system prompt.
func WithSystemContext(ctx context.Context, sysCtx string) context.Context {
	return context.WithValue(ctx, systemContextKey{}, sysCtx)
}

// SystemContextFromContext returns the memory section string, or "" when absent.
func SystemContextFromContext(ctx context.Context) string {
	s, _ := ctx.Value(systemContextKey{}).(string)
	return s
}
