package domain

import (
	"context"
	"errors"
	"fmt"
)

// Sentinel errors. All errors returned by domain ports MUST wrap one of
// these via fmt.Errorf("...: %w", Err...) so callers can use errors.Is.
var (
	// ErrAuthFailed: provided credentials were rejected.
	ErrAuthFailed = errors.New("authentication failed")
	// ErrCacheMiss: cache is not currently active.
	ErrCacheMiss = errors.New("cache miss")
	// ErrPolicyBlocked: command was blocked by the active policy.
	ErrPolicyBlocked = errors.New("blocked by policy")
	// ErrPolicyInvalid: policy configuration is invalid.
	ErrPolicyInvalid = errors.New("invalid policy")
	// ErrAgentNotDetected: no agent of the requested kind was found.
	ErrAgentNotDetected = errors.New("agent not detected")
	// ErrAgentInstallFailed: installation step failed.
	ErrAgentInstallFailed = errors.New("agent install failed")
	// ErrInvalidCommand: command is malformed.
	ErrInvalidCommand = errors.New("invalid command")
	// ErrInvalidCacheConfig: cache configuration is invalid.
	ErrInvalidCacheConfig = errors.New("invalid cache config")
	// ErrConfigNotFound: configuration file not found.
	ErrConfigNotFound = errors.New("config not found")
	// ErrConfigInvalid: configuration file is present but malformed.
	ErrConfigInvalid = errors.New("config invalid")
	// ErrPTYFailed: PTY allocation failed.
	ErrPTYFailed = errors.New("pty failed")
	// ErrSudoNotFound: sudo is not installed on this host.
	ErrSudoNotFound = errors.New("sudo not found")
	// ErrContextCancelled: operation cancelled via context.
	ErrContextCancelled = errors.New("context cancelled")
	// ErrTimeout: operation timed out.
	ErrTimeout = errors.New("timeout")
)

// CancellationError returns true if err is or wraps ErrContextCancelled
// or the standard context.Canceled / context.DeadlineExceeded.
func CancellationError(err error) bool {
	return errors.Is(err, ErrContextCancelled) ||
		errors.Is(err, context.Canceled) ||
		errors.Is(err, context.DeadlineExceeded)
}

// ExitError indicates a command failed with a non-zero exit code.
type ExitError struct {
	Command Command
	Code    int
	Stderr  string
}

func (e *ExitError) Error() string {
	return fmt.Sprintf("command %q exited with code %d", e.Command.String(), e.Code)
}

// Is allows errors.Is(err, ErrSudoNotFound)-style usage via wrapper types.
func (e *ExitError) Is(target error) bool {
	return target == ErrSudoNotFound || target == ErrPTYFailed
}

// Unwrap implements errors.Unwrap for compatibility.
func (e *ExitError) Unwrap() error {
	return nil
}

// PolicyViolationError carries details about a policy block.
type PolicyViolationError struct {
	Command Command
	Result  MatchResult
}

func (e *PolicyViolationError) Error() string {
	return fmt.Sprintf("policy blocked %q: %s", e.Command.String(), e.Result.Reason())
}

// Is targets ErrPolicyBlocked.
func (e *PolicyViolationError) Is(target error) bool {
	return target == ErrPolicyBlocked
}

// Unwrap returns nil; consumers should inspect Result directly.
func (e *PolicyViolationError) Unwrap() error {
	return nil
}
