package permission

import (
	"context"
	"crypto/sha256"
	"fmt"
	"log/slog"
	"time"

	"forge/internal/reqctx"
)

// AuditEvent records a single permission decision for compliance logging.
// ArgsDigest stores a SHA-256 prefix of the tool arguments rather than the
// raw arguments to avoid persisting secrets or sensitive payloads.
type AuditEvent struct {
	Timestamp   time.Time
	TenantID    string
	UserID      string
	Tool        string
	Decision    string // "allow" | "deny" | "ask"
	MatchedRule string // empty when no rule matched
	ArgsDigest  string // hex-encoded first 8 bytes of SHA-256(argsJSON)
}

// AuditLogger persists AuditEvents for compliance and observability.
type AuditLogger interface {
	Log(AuditEvent)
}

// SlogAuditLogger writes audit events as structured slog log records at Info level.
type SlogAuditLogger struct {
	logger *slog.Logger
}

// NewSlogAuditLogger returns an AuditLogger backed by the provided slog.Logger.
func NewSlogAuditLogger(l *slog.Logger) *SlogAuditLogger {
	return &SlogAuditLogger{logger: l}
}

func (a *SlogAuditLogger) Log(e AuditEvent) {
	a.logger.LogAttrs(context.Background(), slog.LevelInfo, "audit",
		slog.Time("ts", e.Timestamp),
		slog.String("tenant_id", e.TenantID),
		slog.String("user_id", e.UserID),
		slog.String("tool", e.Tool),
		slog.String("decision", e.Decision),
		slog.String("matched_rule", e.MatchedRule),
		slog.String("args_digest", e.ArgsDigest),
	)
}

// newAuditEvent builds an AuditEvent from a permission decision and the
// current request context.
func newAuditEvent(ctx context.Context, toolName, argsJSON string, d Decision) AuditEvent {
	matched := ""
	if d.MatchedRule != nil {
		matched = d.MatchedRule.String()
	}
	return AuditEvent{
		Timestamp:   time.Now(),
		TenantID:    reqctx.TenantIDFromContext(ctx),
		UserID:      reqctx.UserIDFromContext(ctx),
		Tool:        toolName,
		Decision:    string(d.Behavior),
		MatchedRule: matched,
		ArgsDigest:  argsDigest(argsJSON),
	}
}

func argsDigest(argsJSON string) string {
	h := sha256.Sum256([]byte(argsJSON))
	return fmt.Sprintf("%x", h[:8])
}
