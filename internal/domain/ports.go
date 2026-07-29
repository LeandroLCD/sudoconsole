// Package domain contains the pure business types and port interfaces.
//
// Ports are the contracts that infrastructure adapters implement.
// Use cases depend only on these ports; they do not know about concrete
// adapters such as creack/pty, sudo, or any specific TOML library.
package domain

import (
	"context"
	"io"
	"time"
)

// PtyGateway is the contract for allocating a pseudo-terminal and reading
// a line of secret input from it without echoing back to the user.
//
// Implementations MUST:
//   - allocate a real PTY (not just pipe stdin)
//   - disable terminal echo (ECHO off, ICANON on) while reading
//   - restore the original terminal settings on Close
//   - zeroize the returned []byte as soon as the caller is done with it
type PtyGateway interface {
	// Allocate spawns a child process attached to a PTY.
	// The returned Closer releases the PTY file descriptors.
	Allocate(ctx context.Context, argv []string, env []string) (PtySession, error)

	// ReadSecret reads one line of input from the user with echo disabled.
	// The returned slice MUST be zeroized by the caller after use.
	ReadSecret(ctx context.Context, prompt string) ([]byte, error)
}

// PtySession is an active PTY-attached process.
//
// Implementations may embed io.ReadWriteCloser for stdio access; the
// explicit Read/Write/Close methods are duplicated here so consumers can
// depend only on the domain package.
type PtySession interface {
	io.ReadWriteCloser
	// Resize updates the PTY window size (signals SIGWINCH).
	Resize(rows, cols uint16) error
	// Pid returns the child process ID.
	Pid() int
	// Wait blocks until the child exits and returns the exit code.
	Wait() (int, error)
}

// SudoGateway is the contract for invoking commands under sudo.
//
// Implementations use the PtyGateway (or an equivalent secure channel) to
// authenticate, then exec the requested command. The returned stream
// contains combined stdout+stderr or separated streams; the contract is
// implementation-specific but MUST surface the exit code.
type SudoGateway interface {
	// Authenticate runs sudo -v (validate-only) and caches the credentials.
	// Returns ErrAuthFailed if the password is rejected.
	Authenticate(ctx context.Context, secret []byte) error

	// Execute runs cmd under sudo using the cached credentials.
	// Returns ErrCacheMiss if the cache is not active; callers should
	// Authenticate first or fall back to a one-shot flow.
	Execute(ctx context.Context, cmd Command) (SudoResult, error)

	// ExecuteWithAuth is the one-shot flow: authenticate then execute.
	// Convenience for the no-cache case.
	ExecuteWithAuth(ctx context.Context, secret []byte, cmd Command) (SudoResult, error)
}

// SudoResult is the outcome of a sudo command invocation.
type SudoResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
	Duration time.Duration
}

// CacheRepository inspects and manipulates the credentials cache.
//
// On Linux the implementation reads the sudo timestamp file in
// /var/db/sudo/ts/. On macOS it reads /var/run/sudo/ts/.
type CacheRepository interface {
	// IsActive returns CacheActive if the timestamp file is present and
	// not older than cfg.TimeoutSeconds.
	IsActive(ctx context.Context, cfg CacheConfig) (CacheStatus, error)

	// TimeRemaining returns how long the cache is still valid.
	TimeRemaining(ctx context.Context, cfg CacheConfig) (time.Duration, error)

	// Refresh silently extends the cache (sudo -v without prompt). It is
	// the responsibility of the caller to obtain the credential; this
	// method re-validates an existing one or fails with ErrCacheMiss.
	Refresh(ctx context.Context, cfg CacheConfig) error
}

// PolicyEvaluator decides whether a command is allowed to run.
type PolicyEvaluator interface {
	// Evaluate returns a MatchResult describing the decision.
	Evaluate(ctx context.Context, policy Policy, cmd Command) (MatchResult, error)

	// ListCategories returns the registered (binary → category) mapping.
	// Useful for `sudoconsole policy list`.
	ListCategories(ctx context.Context) ([]CategoryEntry, error)
}

// CategoryEntry is one row of the binary → category table.
type CategoryEntry struct {
	Binary    string
	Category  Category
	Risk      Risk
	Notes     string
	BuiltIn   bool
	Overrides bool
}

// AuditLogger records policy decisions and command executions.
//
// Implementations should append to a file in JSONL format with mode 0600.
// AuditLogger is best-effort: a logger failure MUST NOT block command
// execution; callers should log the failure and continue.
type AuditLogger interface {
	// Log records a single event.
	Log(ctx context.Context, event AuditEvent) error

	// Close flushes and closes any underlying resources.
	Close() error
}

// AuditEvent is the structured payload of an audit log entry.
type AuditEvent struct {
	Timestamp  time.Time
	User       string
	Hostname   string
	Command    Command
	Decision   Decision
	Result     MatchResult
	ExitCode   int
	SessionID  string
	OverrideBy string // empty if no override
	Notes      string
}

// ConfigStore loads and saves the user's sudoconsole configuration.
type ConfigStore interface {
	// Load returns the effective config, merging defaults + user file.
	Load(ctx context.Context) (Config, error)

	// Save persists the config to the user's config file.
	Save(ctx context.Context, cfg Config) error

	// DefaultPath returns the OS-specific path to the user config file.
	DefaultPath() string
}

// Config is the top-level user-facing configuration.
//
// It is composed of the smaller configs declared in cache.go and
// policy.go to keep each file self-contained.
type Config struct {
	Cache    CacheConfig
	Security SecurityConfig
	Policy   Policy
	Agent    AgentConfig
	Output   OutputConfig
}

// SecurityConfig holds cross-cutting safety settings.
type SecurityConfig struct {
	// PurgeMemory: zeroize credentials after use.
	PurgeMemory bool
	// DisableCoreDumps: set RLIMIT_CORE=0 during authentication.
	DisableCoreDumps bool
	// RequireTTY: reject non-TTY stdin for password prompts.
	RequireTTY bool
}

// DefaultSecurityConfig returns safe defaults.
func DefaultSecurityConfig() SecurityConfig {
	return SecurityConfig{
		PurgeMemory:      true,
		DisableCoreDumps: true,
		RequireTTY:       true,
	}
}

// AgentConfig holds settings for CLI agent auto-detection.
type AgentConfig struct {
	AutoDetect bool
	Install    []AgentKind
	BinDir     string
}

// DefaultAgentConfig returns safe defaults.
func DefaultAgentConfig() AgentConfig {
	return AgentConfig{
		AutoDetect: true,
		Install:    nil, // all detected
		BinDir:     "~/bin",
	}
}

// OutputConfig holds formatting and logging settings.
type OutputConfig struct {
	Format   string // "human" | "json"
	LogLevel string // "silent" | "error" | "warn" | "info" | "debug"
}

// DefaultOutputConfig returns safe defaults.
func DefaultOutputConfig() OutputConfig {
	return OutputConfig{
		Format:   "human",
		LogLevel: "info",
	}
}

// DefaultConfig composes all the per-area defaults into a single config.
func DefaultConfig() Config {
	return Config{
		Cache:    DefaultCacheConfig(),
		Security: DefaultSecurityConfig(),
		Policy:   DefaultPolicy(),
		Agent:    DefaultAgentConfig(),
		Output:   DefaultOutputConfig(),
	}
}

// AgentDetector scans the host for installed CLI agents.
type AgentDetector interface {
	// Detect returns descriptors for all agents found.
	Detect(ctx context.Context) ([]AgentDescriptor, error)
	// DetectOne returns the descriptor for a single kind, or ErrAgentNotDetected.
	DetectOne(ctx context.Context, kind AgentKind) (AgentDescriptor, error)
}

// AgentInstaller registers sudoconsole with a specific CLI agent.
type AgentInstaller interface {
	// Name returns the canonical kind identifier.
	Name() AgentKind

	// Detect reports whether this agent is installed on the host.
	Detect(ctx context.Context) (bool, error)

	// Install registers sudoconsole with the agent (idempotent).
	Install(ctx context.Context, opts InstallOptions) error

	// Uninstall removes the integration. Idempotent.
	Uninstall(ctx context.Context) error

	// AdapterCommand returns the command string the agent should call
	// to use sudoconsole (e.g. "sudoconsole exec ...").
	AdapterCommand() string
}

// InstallOptions configures a single install invocation.
type InstallOptions struct {
	// Force overwrites existing integration files.
	Force bool
	// BinDir is where to place wrapper binaries (default ~/bin).
	BinDir string
	// DryRun reports what would be installed without modifying the FS.
	DryRun bool
	// Config is the active sudoconsole config (for context).
	Config Config
}
