//go:build integration && darwin
// +build integration,darwin

// macOS integration test. macOS has a different sudo timestamp
// directory (/var/db/sudo/ts/<user>) and ships its own auth flow.
// We still drive the same binary but skip user-switching tests since
// creating throw-away users requires admin rights on macOS.
package integration

import (
	"context"
	"errors"
	"os/exec"
	"strings"
	"testing"
	"time"
)

const macOSTestTimeout = 30 * time.Second

func requireMacOSRoot(t *testing.T) {
	t.Helper()
	// The macOS test runner must already be root or sudo-capable; we
	// do not create extra users.
	cmd := exec.CommandContext(context.Background(), "sudo", "-n", "-v")
	cmd.Timeout = macOSTestTimeout
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			t.Skipf("macOS integration test requires sudo NOPASSWD; skipping (%v)", err)
		}
		t.Skipf("macOS integration test setup failed: %v", err)
	}
}

func TestMacOS_CheckVersion(t *testing.T) {
	requireMacOSRoot(t)
	out := run(t, []string{"version"}, nil)
	if out.ExitCode != 0 {
		t.Fatalf("version exit=%d stderr=%s", out.ExitCode, out.Stderr)
	}
	if !strings.Contains(out.Stdout, "darwin") {
		t.Errorf("expected GOOS=darwin in version output; got %q", out.Stdout)
	}
}

func TestMacOS_PolicyTestBlocksSsh(t *testing.T) {
	requireMacOSRoot(t)
	freshHome(t)
	out := run(t, []string{"policy", "test", "--format", "json", "ssh", "user@host"}, nil)
	if out.ExitCode != 64 {
		t.Errorf("expected exit 64; got %d stderr=%s", out.ExitCode, out.Stderr)
	}
}

func TestMacOS_ExecBlocksSsh(t *testing.T) {
	requireMacOSRoot(t)
	freshHome(t)
	out := run(t, []string{"exec", "ssh", "user@host"}, []string{"SUDOCONSOLE_PASSWORD="})
	if out.ExitCode != 64 {
		t.Errorf("expected exit 64 (block); got %d stderr=%s", out.ExitCode, out.Stderr)
	}
}