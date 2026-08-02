package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/LeandroLCD/sudoconsole/internal/domain"
)

// ExecInput configures ExecUseCase.
type ExecInput struct {
	// Config holds the user's effective configuration. The Policy and
	// Cache sections are consulted.
	Config domain.Config

	// Command is the parsed process invocation.
	Command domain.Command

	// Secret is supplied only on the no-cache path. May be nil when
	// the cache is already active.
	Secret []byte

	// Override bypasses a policy Block decision. The decision is still
	// recorded in the audit log so the operator can see when and why.
	Override bool

	// OverrideReason is the audit-tag recorded on OverrideBy when
	// Override is true. CLI passes the value of --policy-override.
	// Empty string falls back to "--policy-override".
	OverrideReason string

	// CacheActive is true when the caller has already validated the
	// cache. False triggers a silent refresh (sudo -v) before execute.
	CacheActive bool
}

// ExecOutput is the outcome of running a single command under sudo.
type ExecOutput struct {
	Result    domain.SudoResult
	Decision  domain.MatchResult
	Blocked   bool
	Overrode  bool
	CacheUsed bool
}

// ExecUseCase ties together policy evaluation, cache management and
// the sudo gateway. It is the single entry point used by the
// `sudoconsole exec` command.
type ExecUseCase struct {
	Gateway    domain.SudoGateway
	Evaluator  domain.PolicyEvaluator
	Cache      domain.CacheRepository
	Policy     domain.Policy
	Audit      domain.AuditLogger
	Evaluator2 *EvaluatePolicyUseCase
}

// NewExecUseCase constructs the use case. evaluator2 may be nil; if
// nil, the use case constructs a default EvaluatePolicyUseCase using
// the Evaluator field.
func NewExecUseCase(gw domain.SudoGateway, ev domain.PolicyEvaluator,
	cache domain.CacheRepository, policy domain.Policy, audit domain.AuditLogger,
) *ExecUseCase {
	if audit == nil {
		audit = nilAudit{}
	}
	uc := &ExecUseCase{
		Gateway:   gw,
		Evaluator: ev,
		Cache:     cache,
		Policy:    policy,
		Audit:     audit,
	}
	if ev != nil {
		uc.Evaluator2 = NewEvaluatePolicyUseCase(ev)
	}
	return uc
}

// Execute runs the command subject to policy + cache + audit.
//
// Return contract:
//
//   - Decision Block  + Override=false → ErrPolicyBlocked (or
//     *domain.PolicyViolationError). The command is not run.
//   - Decision Block  + Override=true  → command runs; Overrode=true.
//   - Decision Allow  / Warn / Audit  → command runs.
func (u *ExecUseCase) Execute(ctx context.Context, in ExecInput) (ExecOutput, error) {
	if u.Gateway == nil {
		return ExecOutput{}, fmt.Errorf("exec: gateway not configured")
	}
	// 1) Policy check first; refuse to even attempt an unauthorised run.
	policyInput := EvaluatePolicyInput{
		Policy:   u.Policy,
		Command:  in.Command,
		Override: in.Override,
	}
	policyOut, err := u.evaluator().Execute(ctx, policyInput)
	if err != nil {
		// Blocked + override disabled → bubbled up here.
		var pve *domain.PolicyViolationError
		if errors.As(err, &pve) {
			_ = u.Audit.Log(ctx, domain.AuditEvent{
				Command:  in.Command,
				Decision: domain.DecisionBlock,
				Result:   pve.Result,
			})
			return ExecOutput{Decision: pve.Result, Blocked: true}, err
		}
		return ExecOutput{}, fmt.Errorf("policy: %w", err)
	}

	// 2) Cache check / refresh.
	cached := in.CacheActive
	if !cached && !in.Config.Cache.Disabled && u.Cache != nil {
		status, cacheErr := u.Cache.IsActive(ctx, in.Config.Cache)
		switch {
		case cacheErr == nil && status == domain.CacheActive:
			cached = true
		case in.Secret != nil:
			// Cache miss → silent refresh via Authenticate (sudo -v).
			if authErr := u.Gateway.Authenticate(ctx, in.Secret); authErr != nil {
				return ExecOutput{}, fmt.Errorf("cache refresh: %w", authErr)
			}
			cached = true
		case in.Config.Cache.Disabled:
			cached = false
		}
		// else: cache expired AND no secret supplied → fall through to
		// ExecuteWithAuth below.
	}

	// 3) Execute under sudo.
	var (
		res   domain.SudoResult
		exErr error
	)
	switch {
	case cached:
		res, exErr = u.Gateway.Execute(ctx, in.Command)
	default:
		if in.Secret == nil {
			return ExecOutput{}, fmt.Errorf("%w: cache expired and no secret supplied", domain.ErrCacheMiss)
		}
		res, exErr = u.Gateway.ExecuteWithAuth(ctx, in.Secret, in.Command)
	}
	if exErr != nil {
		_ = u.Audit.Log(ctx, domain.AuditEvent{
			Command:  in.Command,
			Decision: domain.DecisionAudit,
			Result:   policyOut.Result,
			Notes:    "exec failed: " + exErr.Error(),
		})
		return ExecOutput{Decision: policyOut.Result, Blocked: false, Overrode: policyOut.Override, CacheUsed: cached},
			fmt.Errorf("exec: %w", exErr)
	}

	// 4) Audit.
	_ = u.Audit.Log(ctx, domain.AuditEvent{
		Command:    in.Command,
		Decision:   policyOut.Result.Decision,
		Result:     policyOut.Result,
		ExitCode:   res.ExitCode,
		OverrideBy: overrideBy(policyOut.Override, in.OverrideReason),
	})
	return ExecOutput{
		Result:    res,
		Decision:  policyOut.Result,
		Blocked:   false,
		Overrode:  policyOut.Override,
		CacheUsed: cached,
	}, nil
}

func (u *ExecUseCase) evaluator() *EvaluatePolicyUseCase {
	if u.Evaluator2 != nil {
		return u.Evaluator2
	}
	if u.Evaluator == nil {
		return NewEvaluatePolicyUseCase(nil)
	}
	return NewEvaluatePolicyUseCase(u.Evaluator)
}

func overrideBy(active bool, reason string) string {
	if !active {
		return ""
	}
	if r := strings.TrimSpace(reason); r != "" {
		return r
	}
	return "--policy-override"
}
