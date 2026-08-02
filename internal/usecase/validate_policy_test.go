package usecase

import (
	"context"
	"testing"

	"github.com/LeandroLCD/sudoconsole/internal/domain"
)

type stubValidator struct {
	errs map[string]string
}

func (s stubValidator) ValidatePatterns(patterns []string) []domain.PatternError {
	out := make([]domain.PatternError, 0)
	for _, p := range patterns {
		if reason, ok := s.errs[p]; ok {
			out = append(out, domain.PatternError{Pattern: p, Reason: reason})
		}
	}
	return out
}

func TestValidatePolicy_AllClean(t *testing.T) {
	uc := NewValidatePolicyUseCase(stubValidator{errs: nil})
	out := uc.Execute(context.Background(), ValidatePolicyInput{
		Policy: domain.DefaultPolicy(),
	})
	if !out.Valid {
		t.Errorf("expected Valid=true; got %+v", out)
	}
	if len(out.AllErrors()) != 0 {
		t.Errorf("expected no errors; got %+v", out.AllErrors())
	}
}

func TestValidatePolicy_PerSectionErrors(t *testing.T) {
	blockedErr := domain.PatternError{Pattern: "re:(a+)+", Reason: "redos"}
	allowedErr := domain.PatternError{Pattern: "[bad", Reason: "invalid glob"}
	extrasErr := domain.PatternError{Pattern: "  ", Reason: "empty pattern"}
	v := stubValidator{errs: map[string]string{
		"re:(a+)+": "redos",
		"[bad":     "invalid glob",
		"  ":       "empty pattern",
	}}
	uc := NewValidatePolicyUseCase(v)
	out := uc.Execute(context.Background(), ValidatePolicyInput{
		Policy: domain.Policy{
			Blocked:       domain.Blocklist{Patterns: []string{"re:(a+)+"}},
			Allowed:       domain.Allowlist{Patterns: []string{"[bad"}},
			ExtraPatterns: []string{"  "},
		},
	})
	if out.Valid {
		t.Error("expected Valid=false")
	}
	if len(out.Blocked) != 1 || out.Blocked[0] != blockedErr {
		t.Errorf("Blocked mismatch: %+v", out.Blocked)
	}
	if len(out.Allowed) != 1 || out.Allowed[0] != allowedErr {
		t.Errorf("Allowed mismatch: %+v", out.Allowed)
	}
	if len(out.Extras) != 1 || out.Extras[0] != extrasErr {
		t.Errorf("Extras mismatch: %+v", out.Extras)
	}
	all := out.AllErrors()
	if len(all) != 3 {
		t.Errorf("AllErrors should return 3 entries; got %d", len(all))
	}
}

func TestValidatePolicy_NilValidatorSafe(t *testing.T) {
	uc := NewValidatePolicyUseCase(nil)
	out := uc.Execute(context.Background(), ValidatePolicyInput{
		Policy: domain.Policy{
			Blocked: domain.Blocklist{Patterns: []string{"re:(a+)+"}},
		},
	})
	if !out.Valid {
		t.Errorf("nil validator should treat all patterns as valid; got %+v", out)
	}
}

func TestValidatePolicy_EmptyInput(t *testing.T) {
	uc := NewValidatePolicyUseCase(stubValidator{errs: nil})
	out := uc.Execute(context.Background(), ValidatePolicyInput{})
	if !out.Valid {
		t.Errorf("empty input should be Valid; got %+v", out)
	}
}
