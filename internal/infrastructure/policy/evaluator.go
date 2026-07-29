package policy

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/LeandroLCD/sudoconsole/internal/domain"
)

// Evaluator implements domain.PolicyEvaluator.
//
// It owns:
//   - A category registry (binary → category)
//   - Two compiled matchers: blocked and allowed extra patterns
//
// Evaluate flows:
//  1. Resolve the command's category from the registry.
//  2. Apply the active policy mode (allowlist/blocklist/audit).
//  3. Match against user-defined patterns.
//  4. Apply specific remote-access / credential-exposure toggles.
type Evaluator struct {
	registry     *CategoryRegistry
	policy       domain.Policy
	blockedExtra *Matcher
	allowedExtra *Matcher
}

// NewEvaluator constructs an evaluator with the given policy.
//
// Patterns are compiled eagerly; an invalid pattern returns an error.
func NewEvaluator(p domain.Policy) (*Evaluator, error) {
	b, err := Compile(p.Blocked.Patterns)
	if err != nil {
		return nil, fmt.Errorf("blocked patterns: %w", err)
	}
	a, err := Compile(p.Allowed.Patterns)
	if err != nil {
		return nil, fmt.Errorf("allowed patterns: %w", err)
	}
	if b == nil || a == nil {
		return nil, errors.New("matcher construction returned nil")
	}
	return &Evaluator{
		registry:     DefaultCategoryRegistry(),
		policy:       p,
		blockedExtra: b,
		allowedExtra: a,
	}, nil
}

// NewEvaluatorWithRegistry allows injecting a custom registry (used by tests
// to override the built-in categories).
func NewEvaluatorWithRegistry(p domain.Policy, reg *CategoryRegistry) (*Evaluator, error) {
	e, err := NewEvaluator(p)
	if err != nil {
		return nil, err
	}
	if reg != nil {
		e.registry = reg
	}
	return e, nil
}

// Evaluate returns a MatchResult for the given command.
//
// Algorithm:
//  1. Always: apply explicit category block (regardless of mode)
//  2. Always: apply explicit command blocklist (regardless of mode)
//  3. Always: apply blocked extra patterns
//  4. Mode-specific:
//     - Blocklist: default-allow → most commands pass
//     - Allowlist: default-deny → only explicitly allowed commands pass
//     - Audit: default-allow but log everything
func (e *Evaluator) Evaluate(_ context.Context, cmd domain.Command) (domain.MatchResult, error) {
	result := domain.MatchResult{
		EvaluatedAt: time.Now(),
		Decision:    domain.DecisionAllow,
	}

	// 1. Categorize.
	entry, known := e.registry.Lookup(cmd.Basename())
	if known {
		result.Categories = append(result.Categories, entry.Category)
		result.Risk = entry.Risk
	} else {
		// Unknown binaries default to RiskMedium so the audit log can
		// flag unusual activity, even though the decision is Allow in
		// blocklist mode.
		result.Risk = domain.RiskMedium
	}

	// 2. Explicit category block.
	for _, cat := range result.Categories {
		for _, blocked := range e.policy.Blocked.Categories {
			if cat == blocked {
				result.Decision = domain.DecisionBlock
				result.Reasons = append(result.Reasons,
					fmt.Sprintf("category %q is blocked", cat))
				return result, nil
			}
		}
	}

	// 3. Explicit command blocklist (by basename or full path).
	for _, blocked := range e.policy.Blocked.Commands {
		if cmd.Basename() == blocked || cmd.Path == blocked {
			result.Decision = domain.DecisionBlock
			result.Reasons = append(result.Reasons,
				fmt.Sprintf("command %q is blocked", blocked))
			return result, nil
		}
	}

	// 4. Blocked extra patterns.
	if hit := e.blockedExtra.Match(cmd.String()); hit != "" {
		result.Decision = domain.DecisionBlock
		result.Reasons = append(result.Reasons,
			fmt.Sprintf("matched blocked pattern %q", hit))
		return result, nil
	}

	// 5. Mode-specific logic.
	switch e.policy.Mode {
	case domain.PolicyModeAllowlist:
		if !e.isAllowed(cmd, result.Categories) {
			result.Decision = domain.DecisionBlock
			result.Reasons = append(result.Reasons,
				"allowlist mode: command is not in the explicit allow set")
			return result, nil
		}
	case domain.PolicyModeBlocklist, domain.PolicyModeAudit, domain.PolicyModeUnknown:
		// Blocklist: already allowed unless above triggered.
		// Audit: same as blocklist but logs everything.
		// Unknown: treat as blocklist for safety.
	}

	// 6. Specific toggles (remote_access + credential_exposure).
	if t, reason := e.applyToggles(cmd, result); t {
		result.Decision = domain.DecisionBlock
		result.Reasons = append(result.Reasons, reason)
		return result, nil
	}

	// 7. Audit mode: flip the decision to DecisionAudit so the logger records it.
	if e.policy.Mode == domain.PolicyModeAudit {
		result.Decision = domain.DecisionAudit
	}

	return result, nil
}

// isAllowed returns true if the command matches any explicit allow directive.
func (e *Evaluator) isAllowed(cmd domain.Command, cats []domain.Category) bool {
	for _, allowed := range e.policy.Allowed.Commands {
		if cmd.Basename() == allowed || cmd.Path == allowed {
			return true
		}
	}
	if hit := e.allowedExtra.Match(cmd.String()); hit != "" {
		return true
	}
	for _, cat := range cats {
		for _, allowed := range e.policy.Allowed.Categories {
			if cat == allowed {
				return true
			}
		}
	}
	return false
}

// applyToggles checks the specific policy.RemoteAccess and
// policy.CredentialExposure toggles for matching command line patterns.
// Returns (blocked, reason).
func (e *Evaluator) applyToggles(cmd domain.Command, _ domain.MatchResult) (bool, string) {
	line := cmd.String()

	if e.policy.RemoteAccess.BlockReverseTunnels {
		if strings.Contains(line, "-R ") || strings.Contains(line, "-R\t") {
			return true, "remote_access.BlockReverseTunnels: detected ssh -R"
		}
	}
	if e.policy.RemoteAccess.BlockPortForward {
		if strings.Contains(line, "-L ") || strings.Contains(line, "-L\t") {
			return true, "remote_access.BlockPortForward: detected ssh -L"
		}
	}
	if e.policy.RemoteAccess.BlockTunnels {
		if strings.Contains(line, "-D ") || strings.Contains(line, "-D\t") {
			return true, "remote_access.BlockTunnels: detected ssh -D (SOCKS)"
		}
	}
	if e.policy.RemoteAccess.BlockShellSpawn {
		// Toggles for any shell-spawn category match.
		shell := []string{"bash", "sh", "zsh", "fish", "ksh", "csh", "tcsh", "dash"}
		if cmd.Is("nc") || cmd.Is("ncat") {
			for _, a := range cmd.Args {
				if a == "-e" || a == "-c" || strings.HasPrefix(a, "--exec") {
					return true, "remote_access.BlockShellSpawn: detected nc -e/-c/--exec"
				}
			}
		}
		if cmd.Is("socat") {
			for _, a := range cmd.Args {
				if strings.HasPrefix(a, "exec:") || strings.HasPrefix(a, "system:") {
					return true, "remote_access.BlockShellSpawn: detected socat exec:/system:"
				}
			}
		}
		if cmd.Is("python") || cmd.Is("python2") || cmd.Is("python3") || cmd.Is("perl") || cmd.Is("ruby") {
			for _, a := range cmd.Args {
				if a == "-c" {
					return true, "remote_access.BlockShellSpawn: detected interpreter -c"
				}
			}
		}
		_ = shell
	}

	if e.policy.CredentialExposure.BlockPasswdChange {
		if cmd.Is("passwd") || cmd.Is("chpasswd") {
			return true, "credential_exposure.BlockPasswdChange"
		}
	}
	if e.policy.CredentialExposure.BlockShadowEdit {
		if strings.Contains(line, "/etc/shadow") && (cmd.Is("tee") || cmd.Is("cp") || cmd.Is("mv") || cmd.Is("cat") || cmd.Is("dd")) {
			return true, "credential_exposure.BlockShadowEdit"
		}
	}
	if e.policy.CredentialExposure.BlockSudoersEdit {
		if strings.Contains(line, "/etc/sudoers") && (cmd.Is("tee") || cmd.Is("cp") || cmd.Is("mv") || cmd.Is("cat") || cmd.Is("visudo")) {
			return true, "credential_exposure.BlockSudoersEdit"
		}
	}
	if e.policy.CredentialExposure.BlockSSHKeyExport {
		if cmd.Is("ssh-keygen") {
			return true, "credential_exposure.BlockSSHKeyExport"
		}
	}
	if e.policy.CredentialExposure.BlockSecretExport {
		if cmd.Is("gpg") {
			for _, a := range cmd.Args {
				if strings.HasPrefix(a, "--export-secret") {
					return true, "credential_exposure.BlockSecretExport: gpg --export-secret-keys"
				}
			}
		}
		if cmd.Is("openssl") {
			for i, a := range cmd.Args {
				if a == "rsa" || a == "dsa" || a == "ec" {
					// Detect: openssl rsa -in key -out exported
					_ = i
					if hasExport(cmd.Args) {
						return true, "credential_exposure.BlockSecretExport: openssl key export"
					}
				}
			}
		}
	}
	return false, ""
}

// hasExport returns true if any of the args looks like a write/export flag.
func hasExport(args []string) bool {
	for _, a := range args {
		if a == "-export" || a == "-out" || a == "-outfile" {
			return true
		}
	}
	return false
}

// ListCategories returns all registered categories.
func (e *Evaluator) ListCategories(_ context.Context) ([]domain.CategoryEntry, error) {
	if e == nil || e.registry == nil {
		return nil, nil
	}
	return e.registry.All(), nil
}

// Evaluate is a convenience wrapper for the domain.PolicyEvaluator signature.
func (e *Evaluator) EvaluateCtx(ctx context.Context, policy domain.Policy, cmd domain.Command) (domain.MatchResult, error) {
	old := e.policy
	e.policy = policy
	defer func() { e.policy = old }()
	return e.Evaluate(ctx, cmd)
}
