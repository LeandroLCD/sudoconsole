package usecase

import (
	"context"
	"fmt"
	"os"

	"github.com/LeandroLCD/sudoconsole/internal/domain"
)

// AuthInput configures the AuthUseCase.
type AuthInput struct {
	// Config holds the user's effective configuration. Only the Cache
	// section is consulted at the moment.
	Config domain.Config

	// Secret is the password bytes supplied by the PTY prompt. It is
	// zeroized by the caller immediately after the use case returns.
	Secret []byte

	// Username identifies the user whose sudo cache should be primed.
	// Empty means "the current OS user".
	Username string
}

// AuthUseCase authenticates the user with sudo and primes the cache.
type AuthUseCase struct {
	Gateway domain.SudoGateway
	Audit   domain.AuditLogger
	Now     func() string // injectable clock for tests
}

// NewAuthUseCase wires the use case. Either Gateway or Audit may be
// nil: nil Audit is treated as a noop logger so the use case remains
// usable in tests that do not care about audit.
func NewAuthUseCase(gw domain.SudoGateway, audit domain.AuditLogger) *AuthUseCase {
	if audit == nil {
		audit = nilAudit{}
	}
	return &AuthUseCase{Gateway: gw, Audit: audit, Now: defaultNow}
}

// Execute runs `sudo -v` and records the outcome in the audit log.
func (u *AuthUseCase) Execute(ctx context.Context, in AuthInput) error {
	if u.Gateway == nil {
		return fmt.Errorf("auth: gateway not configured")
	}
	if err := u.Gateway.Authenticate(ctx, in.Secret); err != nil {
		_ = u.Audit.Log(ctx, domain.AuditEvent{
			Decision: domain.DecisionBlock,
			Notes:    "auth failed: " + err.Error(),
		})
		return err
	}
	_ = u.Audit.Log(ctx, domain.AuditEvent{
		Decision: domain.DecisionAudit,
		Notes:    "auth ok",
	})
	return nil
}

// nilAudit is a domain.AuditLogger that discards every event.
type nilAudit struct{}

func (nilAudit) Log(_ context.Context, _ domain.AuditEvent) error { return nil }
func (nilAudit) Close() error                                     { return nil }

// defaultNow returns the current user (best-effort) and hostname so
// audit events carry enough context for triage. It is overridable in
// tests via the Now field.
func defaultNow() string {
	u, _ := os.Hostname()
	return u
}
