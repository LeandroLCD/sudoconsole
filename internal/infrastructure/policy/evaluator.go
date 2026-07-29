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
	checks := []func(domain.Command) (bool, string){
		e.checkReverseTunnel,
		e.checkPortForward,
		e.checkSOCKSTunnel,
		e.checkShellSpawn,
		e.checkPasswdChange,
		e.checkShadowEdit,
		e.checkSudoersEdit,
		e.checkSSHKeyExport,
		e.checkSecretExport,
	}
	for _, c := range checks {
		if c == nil {
			continue
		}
		if blocked, reason := c(cmd); blocked {
			return true, reason
		}
	}
	return false, ""
}

// checkReverseTunnel detects `ssh -R` reverse port forwarding.
func (e *Evaluator) checkReverseTunnel(cmd domain.Command) (bool, string) {
	if !e.policy.RemoteAccess.BlockReverseTunnels {
		return false, ""
	}
	if lineContainsFlag(cmd.String(), "-R") {
		return true, "remote_access.BlockReverseTunnels: detected ssh -R"
	}
	return false, ""
}

// checkPortForward detects `ssh -L` local port forwarding.
func (e *Evaluator) checkPortForward(cmd domain.Command) (bool, string) {
	if !e.policy.RemoteAccess.BlockPortForward {
		return false, ""
	}
	if lineContainsFlag(cmd.String(), "-L") {
		return true, "remote_access.BlockPortForward: detected ssh -L"
	}
	return false, ""
}

// checkSOCKSTunnel detects `ssh -D` dynamic SOCKS proxy.
func (e *Evaluator) checkSOCKSTunnel(cmd domain.Command) (bool, string) {
	if !e.policy.RemoteAccess.BlockTunnels {
		return false, ""
	}
	if lineContainsFlag(cmd.String(), "-D") {
		return true, "remote_access.BlockTunnels: detected ssh -D (SOCKS)"
	}
	return false, ""
}

// checkShellSpawn detects reverse-shell patterns: nc -e, socat exec:, python -c, etc.
func (e *Evaluator) checkShellSpawn(cmd domain.Command) (bool, string) {
	if !e.policy.RemoteAccess.BlockShellSpawn {
		return false, ""
	}
	if blocked, reason := e.checkNcatExec(cmd); blocked {
		return true, reason
	}
	if blocked, reason := e.checkSocatExec(cmd); blocked {
		return true, reason
	}
	if blocked, reason := e.checkInterpreterOneLiner(cmd); blocked {
		return true, reason
	}
	return false, ""
}

// checkNcatExec flags `nc -e`, `nc -c`, `nc --exec`.
func (e *Evaluator) checkNcatExec(cmd domain.Command) (bool, string) {
	if !cmd.Is("nc") && !cmd.Is("ncat") {
		return false, ""
	}
	for _, a := range cmd.Args {
		if a == "-e" || a == "-c" || strings.HasPrefix(a, "--exec") {
			return true, "remote_access.BlockShellSpawn: detected nc -e/-c/--exec"
		}
	}
	return false, ""
}

// checkSocatExec flags `socat exec:` or `socat system:`.
func (e *Evaluator) checkSocatExec(cmd domain.Command) (bool, string) {
	if !cmd.Is("socat") {
		return false, ""
	}
	for _, a := range cmd.Args {
		if strings.HasPrefix(a, "exec:") || strings.HasPrefix(a, "system:") {
			return true, "remote_access.BlockShellSpawn: detected socat exec:/system:"
		}
	}
	return false, ""
}

// checkInterpreterOneLiner flags `python -c`, `perl -e`, `ruby -e`.
func (e *Evaluator) checkInterpreterOneLiner(cmd domain.Command) (bool, string) {
	switch cmd.Basename() {
	case "python", "python2", "python3", "perl", "ruby":
	default:
		return false, ""
	}
	for _, a := range cmd.Args {
		if a == "-c" || a == "-e" {
			return true, "remote_access.BlockShellSpawn: detected interpreter -c/-e"
		}
	}
	return false, ""
}

// checkPasswdChange blocks passwd/chpasswd invocations.
func (e *Evaluator) checkPasswdChange(cmd domain.Command) (bool, string) {
	if !e.policy.CredentialExposure.BlockPasswdChange {
		return false, ""
	}
	if cmd.Is("passwd") || cmd.Is("chpasswd") {
		return true, "credential_exposure.BlockPasswdChange"
	}
	return false, ""
}

// checkShadowEdit blocks writes/copies targeting /etc/shadow.
func (e *Evaluator) checkShadowEdit(cmd domain.Command) (bool, string) {
	if !e.policy.CredentialExposure.BlockShadowEdit {
		return false, ""
	}
	if strings.Contains(cmd.String(), "/etc/shadow") && isPrivilegedCopy(cmd) {
		return true, "credential_exposure.BlockShadowEdit"
	}
	return false, ""
}

// checkSudoersEdit blocks writes/copies targeting /etc/sudoers* paths.
func (e *Evaluator) checkSudoersEdit(cmd domain.Command) (bool, string) {
	if !e.policy.CredentialExposure.BlockSudoersEdit {
		return false, ""
	}
	if strings.Contains(cmd.String(), "/etc/sudoers") && (isPrivilegedCopy(cmd) || cmd.Is("visudo")) {
		return true, "credential_exposure.BlockSudoersEdit"
	}
	return false, ""
}

// checkSSHKeyExport blocks ssh-keygen invocations (which can export keys).
func (e *Evaluator) checkSSHKeyExport(cmd domain.Command) (bool, string) {
	if !e.policy.CredentialExposure.BlockSSHKeyExport {
		return false, ""
	}
	if cmd.Is("ssh-keygen") {
		return true, "credential_exposure.BlockSSHKeyExport"
	}
	return false, ""
}

// checkSecretExport blocks gpg --export-secret and openssl key export.
func (e *Evaluator) checkSecretExport(cmd domain.Command) (bool, string) {
	if !e.policy.CredentialExposure.BlockSecretExport {
		return false, ""
	}
	if cmd.Is("gpg") {
		for _, a := range cmd.Args {
			if strings.HasPrefix(a, "--export-secret") {
				return true, "credential_exposure.BlockSecretExport: gpg --export-secret-keys"
			}
		}
	}
	if cmd.Is("openssl") {
		for _, a := range cmd.Args {
			if (a == "rsa" || a == "dsa" || a == "ec") && hasExport(cmd.Args) {
				return true, "credential_exposure.BlockSecretExport: openssl key export"
			}
		}
	}
	return false, ""
}

// lineContainsFlag returns true if the given flag appears as a separate
// argument in the line ("-R " or "-R\t" ensures it isn't part of another
// word, e.g. --Rfile).
func lineContainsFlag(line, flag string) bool {
	return strings.Contains(line, flag+" ") || strings.Contains(line, flag+"\t")
}

// isPrivilegedCopy returns true for commands that can write to privileged
// paths (tee, cp, mv, cat, dd).
func isPrivilegedCopy(cmd domain.Command) bool {
	for _, b := range []string{"tee", "cp", "mv", "cat", "dd"} {
		if cmd.Is(b) {
			return true
		}
	}
	return false
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

// EvaluateCtx is a convenience wrapper for the domain.PolicyEvaluator signature.
func (e *Evaluator) EvaluateCtx(ctx context.Context, policy domain.Policy, cmd domain.Command) (domain.MatchResult, error) {
	old := e.policy
	e.policy = policy
	defer func() { e.policy = old }()
	return e.Evaluate(ctx, cmd)
}
