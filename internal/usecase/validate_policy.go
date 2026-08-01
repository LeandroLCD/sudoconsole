package usecase

import (
	"context"

	"github.com/LeandroLCD/sudoconsole/internal/domain"
)

// ValidatePolicyInput is the input to ValidatePolicyUseCase.
type ValidatePolicyInput struct {
	// Policy is the effective policy (defaults + user + flag overrides).
	Policy domain.Policy
}

// ValidatePolicyOutput groups the per-list failures so the CLI can
// render them per section.
type ValidatePolicyOutput struct {
	// Blocked covers policy.Blocked.Patterns.
	Blocked []domain.PatternError `json:"blocked,omitempty"`
	// Allowed covers policy.Allowed.Patterns.
	Allowed []domain.PatternError `json:"allowed,omitempty"`
	// Extras covers policy.ExtraPatterns.
	Extras []domain.PatternError `json:"extras,omitempty"`

	// Valid is true when all three lists compiled cleanly.
	Valid bool `json:"valid"`
}

// AllErrors flattens the per-section slices into a single list,
// preserving order (Blocked, then Allowed, then Extras).
func (o ValidatePolicyOutput) AllErrors() []domain.PatternError {
	if o.Valid {
		return nil
	}
	total := len(o.Blocked) + len(o.Allowed) + len(o.Extras)
	if total == 0 {
		return nil
	}
	out := make([]domain.PatternError, 0, total)
	out = append(out, o.Blocked...)
	out = append(out, o.Allowed...)
	out = append(out, o.Extras...)
	return out
}

// ValidatePolicyUseCase compiles every pattern list in a Policy and
// returns a structured report. It never errors out: invalid patterns
// are reported in the output, not as an error.
type ValidatePolicyUseCase struct {
	Validator domain.PolicyValidator
}

// NewValidatePolicyUseCase constructs the use case. A nil Validator
// is replaced with a no-op stub so the use case remains usable in
// tests that don't care about pattern validation.
func NewValidatePolicyUseCase(v domain.PolicyValidator) *ValidatePolicyUseCase {
	if v == nil {
		v = noopValidator{}
	}
	return &ValidatePolicyUseCase{Validator: v}
}

// Execute runs validation and returns the structured report.
func (u *ValidatePolicyUseCase) Execute(_ context.Context, in ValidatePolicyInput) ValidatePolicyOutput {
	out := ValidatePolicyOutput{}
	out.Blocked = u.Validator.ValidatePatterns(in.Policy.Blocked.Patterns)
	out.Allowed = u.Validator.ValidatePatterns(in.Policy.Allowed.Patterns)
	out.Extras = u.Validator.ValidatePatterns(in.Policy.ExtraPatterns)
	out.Valid = len(out.Blocked)+len(out.Allowed)+len(out.Extras) == 0
	return out
}

// noopValidator is the nil-safe fallback.
type noopValidator struct{}

func (noopValidator) ValidatePatterns(_ []string) []domain.PatternError { return nil }
