package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/LeandroLCD/sudoconsole/internal/domain"
)

// EvaluatePolicyInput is the input for EvaluatePolicyUseCase.
type EvaluatePolicyInput struct {
	Policy  domain.Policy
	Command domain.Command
	// Override, if true, bypasses a Block decision (used by the
	// --policy-override flag in the CLI). The use case still returns
	// the MatchResult so the caller can audit it.
	Override bool
}

// EvaluatePolicyOutput is the result.
type EvaluatePolicyOutput struct {
	Result   domain.MatchResult
	Override bool
}

// EvaluatePolicyUseCase runs the policy evaluator and decides whether
// the caller should proceed.
//
// It is the ONLY place that translates a MatchResult into a
// decision-to-proceed-or-not. All transport layers call this usecase
// rather than touching the PolicyEvaluator directly.
type EvaluatePolicyUseCase struct {
	Evaluator domain.PolicyEvaluator
}

// NewEvaluatePolicyUseCase constructs the use case with the given evaluator.
func NewEvaluatePolicyUseCase(ev domain.PolicyEvaluator) *EvaluatePolicyUseCase {
	if ev == nil {
		panic("EvaluatePolicyUseCase: evaluator is nil")
	}
	return &EvaluatePolicyUseCase{Evaluator: ev}
}

// Execute runs the policy check.
//
// Returns:
//   - result.Decision == DecisionAllow / DecisionAudit → caller proceeds
//   - result.Decision == DecisionBlock + Override==false → caller MUST abort
//   - result.Decision == DecisionBlock + Override==true  → caller may proceed (audited)
func (u *EvaluatePolicyUseCase) Execute(ctx context.Context, in EvaluatePolicyInput) (EvaluatePolicyOutput, error) {
	if u.Evaluator == nil {
		return EvaluatePolicyOutput{}, errors.New("policy evaluator not configured")
	}
	if in.Command.Path == "" {
		return EvaluatePolicyOutput{}, fmt.Errorf("%w: empty command path", domain.ErrInvalidCommand)
	}
	result, err := u.Evaluator.Evaluate(ctx, in.Policy, in.Command)
	if err != nil {
		return EvaluatePolicyOutput{}, fmt.Errorf("evaluate: %w", err)
	}
	out := EvaluatePolicyOutput{Result: result, Override: in.Override}
	if result.Decision == domain.DecisionBlock && !in.Override {
		return out, &domain.PolicyViolationError{Command: in.Command, Result: result}
	}
	return out, nil
}

// IsBlockedByPattern is a helper used by the transport layer to format
// user-facing messages. Returns the first matching pattern or "".
func IsBlockedByPattern(m domain.Policy, line string) string {
	for _, p := range m.Blocked.Patterns {
		if strings.Contains(line, p) {
			return p
		}
	}
	for _, c := range m.Blocked.Commands {
		if strings.Contains(line, c) {
			return c
		}
	}
	return ""
}
