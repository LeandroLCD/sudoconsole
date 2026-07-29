package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/LeandroLCD/sudoconsole/internal/domain"
)

type stubEvaluator struct {
	result domain.MatchResult
	err    error
}

func (s stubEvaluator) Evaluate(context.Context, domain.Policy, domain.Command) (domain.MatchResult, error) {
	return s.result, s.err
}
func (s stubEvaluator) ListCategories(context.Context) ([]domain.CategoryEntry, error) {
	return nil, nil
}

func TestEvaluatePolicyUseCase_Allow(t *testing.T) {
	u := NewEvaluatePolicyUseCase(stubEvaluator{
		result: domain.MatchResult{Decision: domain.DecisionAllow},
	})
	out, err := u.Execute(context.Background(), EvaluatePolicyInput{
		Policy:  domain.DefaultPolicy(),
		Command: mustCmd(t, "apt update"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.Result.Decision != domain.DecisionAllow {
		t.Fatalf("Decision = %v", out.Result.Decision)
	}
}

func TestEvaluatePolicyUseCase_Block_NoOverride(t *testing.T) {
	u := NewEvaluatePolicyUseCase(stubEvaluator{
		result: domain.MatchResult{Decision: domain.DecisionBlock, Reasons: []string{"ssh"}},
	})
	_, err := u.Execute(context.Background(), EvaluatePolicyInput{
		Policy:  domain.DefaultPolicy(),
		Command: mustCmd(t, "ssh user@host"),
	})
	if err == nil {
		t.Fatal("block should produce error")
	}
	if !errors.Is(err, domain.ErrPolicyBlocked) {
		t.Fatalf("err = %v, want ErrPolicyBlocked", err)
	}
	var pve *domain.PolicyViolationError
	if !errors.As(err, &pve) {
		t.Fatal("err should be PolicyViolationError")
	}
}

func TestEvaluatePolicyUseCase_Block_WithOverride(t *testing.T) {
	u := NewEvaluatePolicyUseCase(stubEvaluator{
		result: domain.MatchResult{Decision: domain.DecisionBlock},
	})
	out, err := u.Execute(context.Background(), EvaluatePolicyInput{
		Policy:   domain.DefaultPolicy(),
		Command:  mustCmd(t, "ssh user@host"),
		Override: true,
	})
	if err != nil {
		t.Fatalf("override should not error, got %v", err)
	}
	if !out.Override {
		t.Fatal("Override flag should be propagated")
	}
}

func TestEvaluatePolicyUseCase_EmptyCommand(t *testing.T) {
	u := NewEvaluatePolicyUseCase(stubEvaluator{})
	_, err := u.Execute(context.Background(), EvaluatePolicyInput{
		Policy:  domain.DefaultPolicy(),
		Command: domain.Command{},
	})
	if !errors.Is(err, domain.ErrInvalidCommand) {
		t.Fatalf("err = %v", err)
	}
}

func TestEvaluatePolicyUseCase_EvaluatorError(t *testing.T) {
	boom := errors.New("boom")
	u := NewEvaluatePolicyUseCase(stubEvaluator{err: boom})
	_, err := u.Execute(context.Background(), EvaluatePolicyInput{
		Policy:  domain.DefaultPolicy(),
		Command: mustCmd(t, "apt update"),
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, boom) {
		t.Fatalf("err should wrap evaluator error, got %v", err)
	}
}

func TestEvaluatePolicyUseCase_NilEvaluator(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("NewEvaluatePolicyUseCase(nil) should panic")
		}
	}()
	NewEvaluatePolicyUseCase(nil)
}

func TestIsBlockedByPattern(t *testing.T) {
	p := domain.Policy{
		Blocked: domain.Blocklist{
			// Use literal substrings (not globs) for this naive check.
			Patterns: []string{"bad-pattern"},
			Commands: []string{"ssh"},
		},
	}
	if got := IsBlockedByPattern(p, "ssh -R 8080 user@host"); got != "ssh" {
		t.Fatalf("command match should win, got %q", got)
	}
	if got := IsBlockedByPattern(p, "echo bad-pattern here"); got != "bad-pattern" {
		t.Fatalf("pattern match should hit (literal contains), got %q", got)
	}
	if got := IsBlockedByPattern(p, "nothing"); got != "" {
		t.Fatalf("no match expected, got %q", got)
	}
}

func mustCmd(t *testing.T, line string) domain.Command {
	t.Helper()
	c, err := domain.NewCommandFromRaw(line)
	if err != nil {
		t.Fatal(err)
	}
	return c
}
