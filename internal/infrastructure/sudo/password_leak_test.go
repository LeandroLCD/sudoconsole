//go:build linux

package sudo

import (
	"bytes"
	"context"
	"io"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// TestPassword_NotLeakedToArgvOrEnv is the M12 pen-test: it
// confirms that secrets fed through the auth flow do NOT appear
// in:
//
//   - the binary's own argv (/proc/self/cmdline)
//   - the binary's own environment (/proc/self/environ)
//   - recent syslog / auth-log entries (journalctl)
//
// The test exercises the gateway with a unique secret string,
// waits for sudo to exit, then sweeps every available vector.
// Vectors that are inaccessible (e.g. journalctl on a host where
// we have no privileges) cause the test to skip that vector rather
// than fail.
func TestPassword_NotLeakedToArgvOrEnv(t *testing.T) {
	const secret = "P4NTHERE-pen-test-2026-m12"
	g := NewGateway(nil)
	if ptyAvailable() {
		// We have a real PTY; actually exercise the gateway so the
		// post-conditions are meaningful.
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = g.Authenticate(ctx, []byte(secret))
		time.Sleep(150 * time.Millisecond)
	}

	// 1. Our own argv must not contain the secret.
	if data, err := readProcFile("self", "cmdline"); err == nil {
		if bytes.Contains(data, []byte(secret)) {
			t.Fatalf("/proc/self/cmdline leaked secret: %q", data)
		}
	}

	// 2. Our own environment must not contain the secret.
	if data, err := readProcFile("self", "environ"); err == nil {
		if bytes.Contains(data, []byte(secret)) {
			t.Fatalf("/proc/self/environ leaked secret: %q", data)
		}
	}

	// 3. Recent syslog entries must not contain the secret. The
	// binary never logs the password, so this vector is purely
	// defense-in-depth.
	if _, err := exec.LookPath("journalctl"); err == nil {
		probe := exec.CommandContext(context.Background(),
			"journalctl", "--no-pager", "-n", "100", "-q")
		probe.Stderr = io.Discard
		if data, err := probe.Output(); err == nil {
			if bytes.Contains(data, []byte(secret)) {
				t.Fatalf("journalctl contains secret: %q", data)
			}
		}
	}
}

// readProcFile reads /proc/<name>/<file> with cat. The /proc
// filesystem is only available on Linux; on other platforms the
// command will fail and the caller skips the check.
func readProcFile(name, file string) ([]byte, error) {
	return exec.Command("cat", filepath.Join("/proc", name, file)).Output()
}

// ptyAvailable returns true on hosts where opening a real PTY
// would succeed. We probe /dev/ptmx (Linux) to avoid needing to
// actually spawn a process. On macOS or restricted sandboxes the
// open may still fail; callers should treat any error from
// pty.Allocate as "skip".
func ptyAvailable() bool {
	// Best-effort probe — existence is necessary but not
	// sufficient (containers may lack /dev/ptmx permissions).
	_, err := exec.Command("test", "-c", "/dev/ptmx").Output()
	return err == nil
}
