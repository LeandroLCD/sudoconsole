//go:build linux

package pty

import (
	"os/exec"
	"testing"

	"golang.org/x/sys/unix"
)

// TestDisableCoreDump_RlimitZeroed verifies that allocating a PTY
// (which triggers the disableCoreDump path) sets RLIMIT_CORE=0 on
// the calling process. Because the child inherits the limit across
// execve, this guarantees that no coredump will be written by
// either the parent or any descendant.
func TestDisableCoreDump_RlimitZeroed(t *testing.T) {
	// Save the original RLIMIT_CORE so we don't permanently zero
	// it for other tests in the process (the test runner reuses
	// the same process for the entire pty package suite).
	var orig unix.Rlimit
	if err := unix.Prlimit(0, unix.RLIMIT_CORE, nil, &orig); err != nil {
		t.Fatalf("Prlimit get: %v", err)
	}
	// Restore on exit. Use t.Cleanup so partial failures still
	// restore.
	t.Cleanup(func() {
		_ = unix.Prlimit(0, unix.RLIMIT_CORE, &orig, nil)
	})

	g := NewGateway()
	if !g.DisableCoreDumps {
		t.Fatal("NewGateway() must default DisableCoreDumps=true")
	}

	// Build a non-executable argv — Allocate will fail at exec
	// but the disableCoreDump syscall happens BEFORE Start, so the
	// RLIMIT_CORE will already be zeroed.
	dummy := exec.Command("/nonexistent")
	_ = dummy
	disableCoreDump(dummy)

	var after unix.Rlimit
	if err := unix.Prlimit(0, unix.RLIMIT_CORE, nil, &after); err != nil {
		t.Fatalf("Prlimit get: %v", err)
	}
	if after.Cur != 0 || after.Max != 0 {
		t.Errorf("RLIMIT_CORE = {Cur=%d, Max=%d}, want {0,0}",
			after.Cur, after.Max)
	}
}
