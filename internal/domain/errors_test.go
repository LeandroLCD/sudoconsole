package domain

import (
	"context"
	"errors"
	"fmt"
	"testing"
)

func TestExitError_Error(t *testing.T) {
	c, _ := NewCommand("apt", []string{"update"})
	e := &ExitError{Command: c, Code: 100, Stderr: "boom"}
	if e.Error() == "" {
		t.Fatal("Error() must not be empty")
	}
	if !errors.Is(e, ErrSudoNotFound) {
		t.Fatal("ExitError should match ErrSudoNotFound")
	}
	if !errors.Is(e, ErrPTYFailed) {
		t.Fatal("ExitError should match ErrPTYFailed")
	}
	if err := e.Unwrap(); err != nil {
		t.Fatalf("Unwrap should return nil, got %v", err)
	}
}

func TestPolicyViolationError(t *testing.T) {
	c, _ := NewCommand("ssh", []string{"user@host"})
	r := MatchResult{Decision: DecisionBlock, Reasons: []string{"matched: ssh"}}
	e := &PolicyViolationError{Command: c, Result: r}
	if !errors.Is(e, ErrPolicyBlocked) {
		t.Fatal("should match ErrPolicyBlocked")
	}
	if e.Error() == "" {
		t.Fatal("Error must not be empty")
	}
	if err := e.Unwrap(); err != nil {
		t.Fatalf("Unwrap should return nil, got %v", err)
	}
}

func TestCancellationError(t *testing.T) {
	wrapped := fmt.Errorf("operation failed: %w", ErrContextCancelled)
	if !CancellationError(wrapped) {
		t.Fatal("CancellationError should match wrapped ErrContextCancelled")
	}
	if !CancellationError(context.Canceled) {
		t.Fatal("CancellationError should match context.Canceled")
	}
	if !CancellationError(context.DeadlineExceeded) {
		t.Fatal("CancellationError should match context.DeadlineExceeded")
	}
	if CancellationError(nil) {
		t.Fatal("CancellationError(nil) should be false")
	}
	if CancellationError(errors.New("other")) {
		t.Fatal("CancellationError should not match unrelated errors")
	}
}

func TestSentinelErrors(t *testing.T) {
	sentinels := []error{
		ErrAuthFailed, ErrCacheMiss, ErrPolicyBlocked, ErrPolicyInvalid,
		ErrAgentNotDetected, ErrAgentInstallFailed, ErrInvalidCommand,
		ErrInvalidCacheConfig, ErrConfigNotFound, ErrConfigInvalid,
		ErrPTYFailed, ErrSudoNotFound, ErrContextCancelled, ErrTimeout,
	}
	for _, s := range sentinels {
		if s == nil {
			t.Fatal("sentinel error must not be nil")
		}
		if s.Error() == "" {
			t.Fatalf("sentinel %v has empty message", s)
		}
	}
}
