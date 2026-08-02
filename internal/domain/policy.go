package domain

import (
	"strings"
	"time"
)

// Decision is the verdict returned by PolicyEvaluator.
type Decision int

const (
	// DecisionUnknown is the zero value.
	DecisionUnknown Decision = iota
	// DecisionAllow: command may proceed without extra output.
	DecisionAllow
	// DecisionWarn: command may proceed but a warning is printed.
	DecisionWarn
	// DecisionAudit: command may proceed and is recorded in the audit log.
	DecisionAudit
	// DecisionBlock: command MUST NOT proceed.
	DecisionBlock
)

// String returns the human-readable verdict.
func (d Decision) String() string {
	switch d {
	case DecisionAllow:
		return "allow"
	case DecisionWarn:
		return "warn"
	case DecisionAudit:
		return "audit"
	case DecisionBlock:
		return "block"
	default:
		return "unknown"
	}
}

// ExitCode returns the suggested exit code for this decision.
func (d Decision) ExitCode() int {
	switch d {
	case DecisionAllow, DecisionAudit:
		return 0
	case DecisionWarn:
		return 65
	case DecisionBlock:
		return 64
	default:
		return 2
	}
}

// Risk is a coarse classification of how dangerous a command is.
type Risk int

const (
	// RiskUnknown is the zero value.
	RiskUnknown Risk = iota
	// RiskLow: routine, low impact (ls, cat, echo).
	RiskLow
	// RiskMedium: privileged or network but commonly benign (apt, systemctl).
	RiskMedium
	// RiskHigh: privileged + sensitive (passwd, useradd, crontab).
	RiskHigh
	// RiskCritical: remote access, credential exposure, or persistent shell
	// (ssh, nc, visudo, bash, ssh-keygen).
	RiskCritical
)

// String returns the human-readable risk level.
func (r Risk) String() string {
	switch r {
	case RiskLow:
		return "low"
	case RiskMedium:
		return "medium"
	case RiskHigh:
		return "high"
	case RiskCritical:
		return "critical"
	default:
		return "unknown"
	}
}

// Category is a logical grouping for commands (used by policy matching).
type Category string

const (
	// CategoryPackageManager: apt, dnf, pacman, brew, etc.
	CategoryPackageManager Category = "package_manager"
	// CategoryServiceControl: systemctl, service, launchctl.
	CategoryServiceControl Category = "service_control"
	// CategoryFilesystem: cp, mv, rm, chmod, tee (when used on /etc or /root).
	CategoryFilesystem Category = "filesystem"
	// CategoryNetworkConfig: iptables, ufw, nft, ip.
	CategoryNetworkConfig Category = "network_config"
	// CategoryUserManagement: useradd, usermod, groupmod, passwd, chsh.
	CategoryUserManagement Category = "user_management"
	// CategoryRemoteAccess: ssh, scp, sftp, rsync, mosh, telnet.
	CategoryRemoteAccess Category = "remote_access"
	// CategoryCredentialExposure: ssh-keygen, visudo, gpg --export-secret-keys.
	CategoryCredentialExposure Category = "credential_exposure" // #nosec G101 -- identifiers are tool names, not credentials
	// CategoryShellSpawn: bash, sh, python, perl, ruby, nc -e, socat exec:.
	CategoryShellSpawn Category = "shell_spawn"
	// CategoryPersistence: crontab, systemctl enable (paired with reverse shell).
	CategoryPersistence Category = "persistence"
	// CategoryUnknown is the zero value.
	CategoryUnknown Category = ""
)

// PolicyMode is the top-level switch controlling what Policy does by default.
type PolicyMode int

const (
	// PolicyModeUnknown is the zero value.
	PolicyModeUnknown PolicyMode = iota
	// PolicyModeBlocklist: default-allow. Only explicitly listed commands
	// are blocked.
	PolicyModeBlocklist
	// PolicyModeAllowlist: default-deny. Only explicitly listed commands
	// are permitted.
	PolicyModeAllowlist
	// PolicyModeAudit: all commands run, every decision is logged.
	PolicyModeAudit
)

// String returns the human-readable mode.
func (m PolicyMode) String() string {
	switch m {
	case PolicyModeBlocklist:
		return "blocklist"
	case PolicyModeAllowlist:
		return "allowlist"
	case PolicyModeAudit:
		return "audit"
	default:
		return "unknown"
	}
}

// Policy is the runtime configuration for the policy engine.
//
// All fields are optional; the evaluator merges the active policy with
// sensible defaults from the categories data file.
type Policy struct {
	// Mode selects the overall behavior.
	Mode PolicyMode

	// Blocked contains commands that must never run regardless of mode.
	// Matched by basename (e.g. "ssh") or by full path.
	Blocked Blocklist

	// Allowed contains commands that bypass the blocklist. In allowlist
	// mode these are the ONLY commands permitted.
	Allowed Allowlist

	// Audit configures the audit log.
	Audit AuditConfig

	// ExtraPatterns are user-defined glob/regex patterns that always block.
	// Compiled at config load time.
	ExtraPatterns []string

	// RemoteAccess toggles specific remote-access checks.
	RemoteAccess RemoteAccessConfig

	// CredentialExposure toggles specific credential-exposure checks.
	CredentialExposure CredentialExposureConfig
}

// Blocklist groups block directives.
type Blocklist struct {
	// Categories: commands belonging to any of these categories are blocked.
	Categories []Category
	// Commands: explicit basenames or full paths that are blocked.
	Commands []string
	// Patterns: glob/regex matched against the full command line.
	Patterns []string
}

// Allowlist groups allow directives.
type Allowlist struct {
	Categories []Category
	Commands   []string
	Patterns   []string
}

// AuditConfig configures the audit logger.
type AuditConfig struct {
	// LogFile is the absolute path to the JSONL log. Empty = disabled.
	LogFile string
	// LogBlocked: log every blocked command.
	LogBlocked bool
	// LogAllowed: log every allowed command (verbose).
	LogAllowed bool
	// LogWarned: log every warned command.
	LogWarned bool
	// MaxBytes is the rotation threshold (default 10 MiB).
	MaxBytes int64
}

// RemoteAccessConfig toggles remote-access checks.
type RemoteAccessConfig struct {
	BlockReverseTunnels bool // ssh -R / ssh -L external
	BlockPortForward    bool
	BlockShellSpawn     bool
	BlockTunnels        bool
}

// CredentialExposureConfig toggles credential-exposure checks.
type CredentialExposureConfig struct {
	BlockPasswdChange bool
	BlockShadowEdit   bool
	BlockSudoersEdit  bool
	BlockSSHKeyExport bool
	BlockSecretExport bool
}

// DefaultPolicy returns a safe default policy.
//
// Mode: blocklist. Remote access + credential exposure + shell spawn + persistence
// are blocked. All entries are logged.
func DefaultPolicy() Policy {
	return Policy{
		Mode: PolicyModeBlocklist,
		Blocked: Blocklist{
			Categories: []Category{
				CategoryRemoteAccess,
				CategoryCredentialExposure,
				CategoryShellSpawn,
				CategoryPersistence,
			},
		},
		Audit: AuditConfig{
			LogBlocked: true,
			LogWarned:  true,
			MaxBytes:   10 * 1024 * 1024,
		},
		RemoteAccess: RemoteAccessConfig{
			BlockReverseTunnels: true,
			BlockPortForward:    true,
			BlockShellSpawn:     true,
			BlockTunnels:        true,
		},
		CredentialExposure: CredentialExposureConfig{
			BlockPasswdChange: true,
			BlockShadowEdit:   true,
			BlockSudoersEdit:  true,
			BlockSSHKeyExport: true,
			BlockSecretExport: true,
		},
	}
}

// MatchResult is the outcome of evaluating a single command against the
// active policy.
type MatchResult struct {
	// Decision is the verdict.
	Decision Decision
	// Reasons are human-readable explanations (e.g. "matched pattern: ssh -R *").
	Reasons []string
	// Categories the command belongs to.
	Categories []Category
	// Risk is the coarse danger level.
	Risk Risk
	// EvaluatedAt is when the decision was made.
	EvaluatedAt time.Time
}

// Reason returns Reasons joined with semicolons, or "no reason" if empty.
func (m MatchResult) Reason() string {
	if len(m.Reasons) == 0 {
		return "no reason"
	}
	return strings.Join(m.Reasons, "; ")
}

// IsTerminal returns true iff the decision is Allow or Block (i.e. not
// a Warn/Audit that requires further processing).
func (m MatchResult) IsTerminal() bool {
	return m.Decision == DecisionAllow || m.Decision == DecisionBlock
}
