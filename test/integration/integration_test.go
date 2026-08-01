//go:build integration
// +build integration

// Package integration holds the end-to-end suite that drives the
// compiled `sudoconsole` binary against a real sudo environment.
//
// The suite is gated behind the `integration` build tag so it never
// runs as part of the fast unit-test loop. Invoke it with:
//
//	make test-integration
//
// or directly:
//
//	go test -tags=integration ./test/integration/... -v
//
// Required environment:
//
//   - SUDOCONSOLE_BIN        — absolute path to the compiled binary
//     (default: ./bin/sudoconsole, which is also the layout produced
//     by `make build`).
//   - SUDOCONSOLE_TEST_PASSWORD — the password for the
//     password-protected test user (default: "sudopwd").
//
// Tests run as the NOPASSWD user (`sudotest`) and switch to the
// password-protected user (`sudopwd`) via `sudo -u sudopwd -E` when
// exercising the password prompt path.
package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Configuration knobs (env vars)
// ---------------------------------------------------------------------------

const (
	defaultBinary     = "./bin/sudoconsole"
	containerBinary   = "/usr/local/bin/sudoconsole"
	defaultPassword   = "sudopwd"
	nopasswdUser      = "sudotest"
	passwordUser      = "sudopwd"
	auditLogEnvar     = "SUDOCONSOLE_AUDIT_LOG"
	configPathEnvar   = "SUDOCONSOLE_CONFIG"
	passwordEnvar     = "SUDOCONSOLE_PASSWORD"
	testTimeout       = 60 * time.Second
	integrationPrefix = "[integration]"
)

func binaryPath(t *testing.T) string {
	t.Helper()
	if p := os.Getenv("SUDOCONSOLE_BIN"); p != "" {
		return p
	}
	// Inside the Docker container the binary is bind-mounted at
	// /usr/local/bin/sudoconsole (see .github/workflows/integration.yml).
	if _, err := os.Stat(containerBinary); err == nil {
		return containerBinary
	}
	if _, err := os.Stat(defaultBinary); err == nil {
		return defaultBinary
	}
	t.Fatalf("SUDOCONSOLE_BIN not set and %s / %s not found; run `make build` first",
		containerBinary, defaultBinary)
	return ""
}

func testPassword() string {
	if p := os.Getenv("SUDOCONSOLE_TEST_PASSWORD"); p != "" {
		return p
	}
	return defaultPassword
}

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

// runResult captures the outcome of a single binary invocation.
type runResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

// run executes the sudoconsole binary with args + env. The PATH is
// inherited so the binary can locate `sudo` / `id` etc.
func run(t *testing.T, args []string, extraEnv []string) runResult {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, binaryPath(t), args...)
	cmd.Env = append(os.Environ(), extraEnv...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	res := runResult{Stdout: stdout.String(), Stderr: stderr.String()}
	if err == nil {
		res.ExitCode = 0
		return res
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		res.ExitCode = exitErr.ExitCode()
		return res
	}
	t.Fatalf("run %v: %v\nstdout: %s\nstderr: %s", args, err, stdout.String(), stderr.String())
	return res
}

// sudoRun executes a command under sudo as the password-protected user.
// Used only when we need to simulate a different user identity.
func sudoRun(t *testing.T, args []string, password string) runResult {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "sudo", append([]string{"-n", "-u", passwordUser, "-E"}, args...)...)
	cmd.Env = append(os.Environ(), "SUDOCONSOLE_PASSWORD="+password)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	res := runResult{Stdout: stdout.String(), Stderr: stderr.String()}
	if err == nil {
		res.ExitCode = 0
		return res
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		res.ExitCode = exitErr.ExitCode()
		return res
	}
	t.Fatalf("sudoRun %v: %v\nstdout: %s\nstderr: %s", args, err, stdout.String(), stderr.String())
	return res
}

// freshHome creates an isolated HOME directory for the current test.
// All sudoconsole invocations should use this as HOME so the audit
// log and config file land in a predictable, per-test location.
//
// The directory lives inside the container under $TEST_HOME_ROOT
// (defaults to /tmp/sudoconsole-it) so the host's t.TempDir (which
// is only meaningful on the host) is never used as the runtime HOME.
func freshHome(t *testing.T) (home string, cleanup func()) {
	t.Helper()
	root := os.Getenv("TEST_HOME_ROOT")
	if root == "" {
		root = "/tmp/sudoconsole-it"
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", root, err)
	}
	dir, err := os.MkdirTemp(root, strings.ReplaceAll(t.Name(), "/", "_")+"-")
	if err != nil {
		t.Fatalf("mkdirtemp: %v", err)
	}
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, ".config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(dir, ".local", "share"))
	return dir, func() {
		_ = os.RemoveAll(dir)
	}
}

// writeConfig writes a sudoconsole TOML config to the user's
// $XDG_CONFIG_HOME/sudoconsole/config.toml.
func writeConfig(t *testing.T, home, content string) {
	t.Helper()
	dir := filepath.Join(home, ".config", "sudoconsole")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.toml"), []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	t.Setenv(configPathEnvar, filepath.Join(dir, "config.toml"))
}

// auditLogPath returns the canonical audit log path for the test user.
// The parent directory is created as a side-effect so the audit
// logger (which does NOT MkdirAll itself) can open the file directly.
func auditLogPath(t *testing.T, home string) string {
	t.Helper()
	dir := filepath.Join(home, ".local", "share", "sudoconsole")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("mkdir audit dir: %v", err)
	}
	return filepath.Join(dir, "audit.log")
}

// readAuditLines slurps every JSONL line into a slice.
func readAuditLines(t *testing.T, path string) []map[string]any {
	t.Helper()
	data, err := os.ReadFile(path) // #nosec G304 -- test owns the file
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatalf("read audit: %v", err)
	}
	out := make([]map[string]any, 0)
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if line == "" {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			t.Fatalf("invalid JSONL line %q: %v", line, err)
		}
		out = append(out, m)
	}
	return out
}

// isRoot reports whether the current process has uid 0. CI runs the
// test as the `sudotest` user, never as root, so this is purely a
// safety guard.
func isRoot() bool { return os.Geteuid() == 0 }

// requireLinux skips the test when the host kernel is not Linux. The
// Docker matrix only ships Linux images; macOS has its own file
// (macos_test.go) gated by `darwin`.
func requireLinux(t *testing.T) {
	t.Helper()
	if runtime.GOOS != "linux" {
		t.Skipf("integration test requires linux; running on %s", runtime.GOOS)
	}
	if isRoot() {
		t.Skip("integration test must not run as root (would bypass sudo)")
	}
}

// logf is a small wrapper around t.Logf that prefixes every message
// with [integration] so the matrix output is grep-friendly.
func logf(t *testing.T, format string, args ...any) {
	t.Helper()
	t.Logf(integrationPrefix+" "+format, args...)
}

// ---------------------------------------------------------------------------
// Test suite
// ---------------------------------------------------------------------------

func TestSudoVersion(t *testing.T) {
	requireLinux(t)
	out := run(t, []string{"version"}, nil)
	if !strings.Contains(out.Stdout, "sudoconsole") {
		t.Errorf("expected version banner; got %q", out.Stdout)
	}
	// Quick smoke test that sudo is callable too.
	cmd := exec.CommandContext(context.Background(), "sudo", "-n", "-v") // #nosec G204 -- arg is constant
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err != nil {
		t.Skipf("sudo NOPASSWD not available in this environment: %v", err)
	}
}

func TestCheck_ReportsCacheState(t *testing.T) {
	requireLinux(t)
	freshHome(t)

	out := run(t, []string{"check", "--format", "json"}, nil)
	if out.ExitCode != 0 {
		t.Fatalf("exit=%d stderr=%s", out.ExitCode, out.Stderr)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(out.Stdout), &got); err != nil {
		t.Fatalf("json: %v body=%q", err, out.Stdout)
	}
	if _, ok := got["active"]; !ok {
		t.Errorf("missing active field; got %+v", got)
	}
	logf(t, "cache check OK: %+v", got)
}

func TestAuth_NopasswdUser(t *testing.T) {
	requireLinux(t)
	freshHome(t)
	// sudo -n -v succeeds silently because sudotest has NOPASSWD.
	cmd := exec.CommandContext(context.Background(), "sudo", "-n", "-v") // #nosec G204 -- fixed args
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err != nil {
		t.Skipf("NOPASSWD sudo unavailable: %v", err)
	}
	out := run(t, []string{"auth", "--no-tty"}, []string{passwordEnvar + "="})
	if out.ExitCode != 0 {
		t.Fatalf("auth exit=%d stderr=%s", out.ExitCode, out.Stderr)
	}
}

func TestAuth_WrongPassword(t *testing.T) {
	requireLinux(t)
	if runtime.GOOS != "linux" {
		t.Skip("requires linux")
	}
	freshHome(t)
	// We can't run as sudopwd in-process, so we exec `sudo -u sudopwd -E
	// sudoconsole auth --no-tty` with the wrong password. The test
	// relies on the runner being sudotest who can invoke `sudo -u sudopwd`
	// without a password (NOPASSWD ALL).
	cmd := exec.CommandContext(context.Background(), "sudo", "-n", "-u", passwordUser, "-E",
		binaryPath(t), "auth", "--no-tty")
	cmd.Env = append(os.Environ(), passwordEnvar+"=wrong-password-here")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err == nil {
		t.Fatalf("expected auth to fail; stdout=%s stderr=%s", stdout.String(), stderr.String())
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected ExitError, got %v", err)
	}
	// sudoconsole auth returns exit 2 on auth failure (ErrAuthFailed).
	if exitErr.ExitCode() != 2 {
		t.Errorf("expected exit 2; got %d stderr=%s", exitErr.ExitCode(), stderr.String())
	}
}

func TestAuth_RightPassword(t *testing.T) {
	requireLinux(t)
	freshHome(t)
	cmd := exec.CommandContext(context.Background(), "sudo", "-n", "-u", passwordUser, "-E",
		binaryPath(t), "auth", "--no-tty")
	cmd.Env = append(os.Environ(), passwordEnvar+"="+testPassword())
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			t.Fatalf("expected auth to succeed; exit=%d stderr=%s stdout=%s",
				exitErr.ExitCode(), stderr.String(), stdout.String())
		}
		t.Fatalf("auth: %v", err)
	}
}

func TestExec_AllowsAptVersion(t *testing.T) {
	requireLinux(t)
	freshHome(t)
	// apt-get --version is safe and returns 0 on every Debian-derived
	// distro. On non-apt distros the test still runs `id` instead.
	cmdLine := detectSafeCommand(t)
	out := run(t, []string{"exec", "--format", "json", cmdLine}, []string{passwordEnvar + "="})
	if out.ExitCode != 0 {
		t.Fatalf("exec exit=%d stderr=%s", out.ExitCode, out.Stderr)
	}
}

func TestExec_BlocksSsh(t *testing.T) {
	requireLinux(t)
	freshHome(t)
	out := run(t, []string{"exec", "ssh", "user@host"}, []string{passwordEnvar + "="})
	if out.ExitCode != 64 {
		t.Fatalf("expected exit 64 (policy block); got %d stderr=%s", out.ExitCode, out.Stderr)
	}
	if !strings.Contains(out.Stderr+out.Stdout, "ssh") {
		t.Errorf("output should mention ssh; got stdout=%q stderr=%q", out.Stdout, out.Stderr)
	}
}

func TestExec_OverrideRequiresConfirmOrYes(t *testing.T) {
	requireLinux(t)
	freshHome(t)
	// No --yes, no stdin answer → must abort.
	out := run(t, []string{"exec", "--policy-override", "integration-test", "ssh", "user@host"}, nil)
	if out.ExitCode == 0 {
		t.Errorf("expected non-zero exit when override rejected; got 0")
	}
}

func TestExec_OverrideRunsAndAudits(t *testing.T) {
	requireLinux(t)
	home, _ := freshHome(t)
	writeConfig(t, home, `
[output]
format = "json"

[policy]
audit = { log_file = "`+auditLogPath(t, home)+`", log_blocked = true, log_allowed = true, log_warned = true }
`)
	// --yes skips the interactive confirm; --policy-override <reason>
	// records the reason on OverrideBy. We target `id` (always
	// allowed) so the override path runs even though nothing was
	// actually blocked — this exercises the audit plumbing.
	out := run(t, []string{"exec", "--policy-override", "integration-test", "--yes",
		"--format", "json", "id", "-u"}, []string{passwordEnvar + "="})
	if out.ExitCode != 0 {
		t.Fatalf("exec exit=%d stderr=%s stdout=%s", out.ExitCode, out.Stderr, out.Stdout)
	}
	entries := readAuditLines(t, auditLogPath(t, home))
	if len(entries) == 0 {
		t.Fatalf("audit log empty after exec; expected at least one entry")
	}
	last := entries[len(entries)-1]
	if got, _ := last["override_by"].(string); got != "integration-test" {
		t.Errorf("expected override_by=integration-test; got %q (entry=%+v)", got, last)
	}
}

func TestAuditLog_OneEntryPerCommand(t *testing.T) {
	requireLinux(t)
	home, _ := freshHome(t)
	writeConfig(t, home, `
[output]
format = "json"

[policy]
audit = { log_file = "`+auditLogPath(t, home)+`", log_blocked = true, log_allowed = true }
`)
	cmds := [][]string{
		{"exec", "--yes", "--format", "json", "id"},
		{"exec", "--yes", "--format", "json", "id", "-u"},
		{"exec", "--format", "json", "ssh", "user@host"}, // expect block + audit entry
	}
	for _, c := range cmds {
		_ = run(t, c, []string{passwordEnvar + "="})
	}
	entries := readAuditLines(t, auditLogPath(t, home))
	if len(entries) != len(cmds) {
		t.Fatalf("expected %d audit entries; got %d", len(cmds), len(entries))
	}
	// Verify the blocked command was recorded as such.
	last := entries[len(entries)-1]
	if d, _ := last["decision"].(string); d != "block" {
		t.Errorf("expected last decision=block; got %q (entry=%+v)", d, last)
	}
}

func TestDetect_FindsKnownAgents(t *testing.T) {
	requireLinux(t)
	freshHome(t)
	out := run(t, []string{"detect", "--format", "json"}, nil)
	if out.ExitCode != 0 {
		t.Fatalf("detect exit=%d stderr=%s", out.ExitCode, out.Stderr)
	}
	var got struct {
		Detected []map[string]any `json:"detected"`
	}
	if err := json.Unmarshal([]byte(out.Stdout), &got); err != nil {
		t.Fatalf("json: %v body=%q", err, out.Stdout)
	}
	logf(t, "detect found %d agents", len(got.Detected))
}

func TestPolicyValidate_BadPatterns(t *testing.T) {
	requireLinux(t)
	home, _ := freshHome(t)
	writeConfig(t, home, `
[policy]
extra_patterns = ["re:(a+)+", "[bad", "ok-pattern"]
`)
	out := run(t, []string{"policy", "validate"}, nil)
	if out.ExitCode == 0 {
		t.Errorf("expected non-zero exit on bad patterns; stdout=%s stderr=%s",
			out.Stdout, out.Stderr)
	}
	if !strings.Contains(out.Stdout, "(a+)+") {
		t.Errorf("expected ReDoS pattern in output; got %q", out.Stdout)
	}
}

func TestPolicyTest_ReportsBlocked(t *testing.T) {
	requireLinux(t)
	freshHome(t)
	out := run(t, []string{"policy", "test", "--format", "json", "ssh", "user@host"}, nil)
	if out.ExitCode != 64 {
		t.Errorf("expected exit 64 (block); got %d stderr=%s", out.ExitCode, out.Stderr)
	}
	var got struct {
		Decision string `json:"decision"`
	}
	if err := json.Unmarshal([]byte(out.Stdout), &got); err != nil {
		t.Fatalf("json: %v body=%q", err, out.Stdout)
	}
	if got.Decision != "block" {
		t.Errorf("expected decision=block; got %q", got.Decision)
	}
}

// detectSafeCommand returns a benign sudo-safe command that succeeds
// on every supported distro. `id` is POSIX; `-u` prints the numeric
// uid which is universally available.
func detectSafeCommand(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("apt-get"); err == nil {
		return "apt-get --version"
	}
	if _, err := exec.LookPath("dnf"); err == nil {
		return "dnf --version"
	}
	if _, err := exec.LookPath("pacman"); err == nil {
		return "pacman --version"
	}
	// POSIX fallback.
	return "id"
}
