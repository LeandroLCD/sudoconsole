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

type runResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

// run executes the sudoconsole binary with args + env.
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

// runAsPasswordUser executes the binary as the password-protected
// user (which has the right cache after a real auth). The caller is
// expected to have already authenticated.
func runAsPasswordUser(t *testing.T, args []string, extraEnv []string) runResult {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()
	argv := append([]string{"-n", "-u", passwordUser, "-E", binaryPath(t)}, args...)
	cmd := exec.CommandContext(ctx, "sudo", argv...)
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
	t.Fatalf("runAsPasswordUser %v: %v\nstdout: %s\nstderr: %s", args, err,
		stdout.String(), stderr.String())
	return res
}

// freshHome creates an isolated HOME directory for the current test
// inside /tmp/sudoconsole-it (override via TEST_HOME_ROOT). The
// parent and child directories are world-readable / executable so
// the password-protected user (`sudopwd`) can traverse the path
// when invoked via `sudo -u`.
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
	if err := os.Chmod(dir, 0o755); err != nil {
		t.Fatalf("chmod %s: %v", dir, err)
	}
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, ".config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(dir, ".local", "share"))
	return dir, func() {
		_ = os.RemoveAll(dir)
	}
}

// writeConfig writes a sudoconsole TOML config. The file is
// world-readable so both `sudotest` and `sudopwd` (when invoked
// through `sudo -u`) can load it. Mode 0600 + group=other would
// fail the user-switch path.
func writeConfig(t *testing.T, home, content string) {
	t.Helper()
	dir := filepath.Join(home, ".config", "sudoconsole")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	t.Setenv(configPathEnvar, path)
}

// writeCacheConfig writes a config with valid [cache] defaults and
// the supplied policy/audit additions. Centralises the
// cache.timeout_seconds plumbing that the validator requires. The
// output format is forced to JSON so downstream assertions can
// parse the result.
func writeCacheConfig(t *testing.T, home, policyTOML string) {
	t.Helper()
	body := "[cache]\ntimeout_seconds = 900\nrefresh_before_seconds = 60\n\n[output]\nformat = \"json\"\n\n" + policyTOML
	writeConfig(t, home, body)
}

// auditLogPath returns the canonical audit log path. Creates the
// parent directory because the audit logger does not MkdirAll.
func auditLogPath(t *testing.T, home string) string {
	t.Helper()
	dir := filepath.Join(home, ".local", "share", "sudoconsole")
	if err := os.MkdirAll(dir, 0o755); err != nil {
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

// sudoNoPasswdAvailable returns true when the current user can call
// `sudo -n -v` without a password (the test rig must satisfy this).
func sudoNoPasswdAvailable() bool {
	cmd := exec.Command("sudo", "-n", "-v")
	cmd.Stderr = io.Discard
	return cmd.Run() == nil
}

func isRoot() bool { return os.Geteuid() == 0 }

func requireLinux(t *testing.T) {
	t.Helper()
	if runtime.GOOS != "linux" {
		t.Skipf("integration test requires linux; running on %s", runtime.GOOS)
	}
	if isRoot() {
		t.Skip("integration test must not run as root (would bypass sudo)")
	}
}

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
	if !sudoNoPasswdAvailable() {
		t.Skip("sudo NOPASSWD not available in this environment")
	}
}

func TestCheck_ReportsCacheState(t *testing.T) {
	requireLinux(t)
	if !sudoNoPasswdAvailable() {
		t.Skip("sudo NOPASSWD not available")
	}
	home, _ := freshHome(t)
	writeCacheConfig(t, home, `
[output]
format = "json"
`)
	out := run(t, []string{"check"}, nil)
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

func TestAuth_WrongPassword(t *testing.T) {
	requireLinux(t)
	freshHome(t)
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
	if exitErr.ExitCode() != 2 {
		t.Errorf("expected exit 2 (auth failed); got %d stderr=%s",
			exitErr.ExitCode(), stderr.String())
	}
}

func TestExec_AllowsSafeCommand(t *testing.T) {
	requireLinux(t)
	freshHome(t)
	// Chain: auth as sudopwd, then exec as sudopwd.
	authCmd := exec.CommandContext(context.Background(), "sudo", "-n", "-u", passwordUser, "-E",
		binaryPath(t), "auth", "--no-tty")
	authCmd.Env = append(os.Environ(), passwordEnvar+"="+testPassword())
	if out, err := authCmd.CombinedOutput(); err != nil {
		t.Fatalf("auth setup: %v out=%s", err, string(out))
	}
	cmdLine := detectSafeCommand(t)
	out := runAsPasswordUser(t, []string{"exec", "--format", "json", cmdLine}, nil)
	if out.ExitCode != 0 {
		t.Fatalf("exec exit=%d stderr=%s stdout=%s", out.ExitCode, out.Stderr, out.Stdout)
	}
}

func TestExec_BlocksSsh(t *testing.T) {
	requireLinux(t)
	freshHome(t)
	// auth first so the policy decision is what produces the exit 64
	// (not "cache expired").
	authCmd := exec.CommandContext(context.Background(), "sudo", "-n", "-u", passwordUser, "-E",
		binaryPath(t), "auth", "--no-tty")
	authCmd.Env = append(os.Environ(), passwordEnvar+"="+testPassword())
	if out, err := authCmd.CombinedOutput(); err != nil {
		t.Fatalf("auth setup: %v out=%s", err, string(out))
	}
	out := runAsPasswordUser(t, []string{"exec", "ssh", "user@host"}, nil)
	if out.ExitCode != 64 {
		t.Fatalf("expected exit 64 (policy block); got %d stderr=%s stdout=%s",
			out.ExitCode, out.Stderr, out.Stdout)
	}
}

func TestExec_OverrideRequiresConfirmOrYes(t *testing.T) {
	requireLinux(t)
	if !sudoNoPasswdAvailable() {
		t.Skip("sudo NOPASSWD not available")
	}
	freshHome(t)
	out := run(t, []string{"exec", "--policy-override", "integration-test", "ssh", "user@host"}, nil)
	if out.ExitCode == 0 {
		t.Errorf("expected non-zero exit when override rejected; got 0")
	}
}

func TestExec_OverrideRunsAndAudits(t *testing.T) {
	requireLinux(t)
	home, _ := freshHome(t)
	writeCacheConfig(t, home, `
[policy]
audit = { log_file = "`+auditLogPath(t, home)+`", log_blocked = true, log_allowed = true, log_warned = true }
`)
	authCmd := exec.CommandContext(context.Background(), "sudo", "-n", "-u", passwordUser, "-E",
		binaryPath(t), "auth", "--no-tty")
	authCmd.Env = append(os.Environ(), passwordEnvar+"="+testPassword())
	if out, err := authCmd.CombinedOutput(); err != nil {
		t.Fatalf("auth setup: %v out=%s", err, string(out))
	}
	out := runAsPasswordUser(t,
		[]string{"exec", "--policy-override", "integration-test", "--yes",
			"--format", "json", "id", "-u"}, nil)
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
	writeCacheConfig(t, home, `
[policy]
audit = { log_file = "`+auditLogPath(t, home)+`", log_blocked = true, log_allowed = true }
`)
	// prime the cache once.
	authCmd := exec.CommandContext(context.Background(), "sudo", "-n", "-u", passwordUser, "-E",
		binaryPath(t), "auth", "--no-tty")
	authCmd.Env = append(os.Environ(), passwordEnvar+"="+testPassword())
	if out, err := authCmd.CombinedOutput(); err != nil {
		t.Fatalf("auth setup: %v out=%s", err, string(out))
	}
	cmds := [][]string{
		{"exec", "--yes", "--format", "json", "id"},
		{"exec", "--yes", "--format", "json", "id", "-u"},
		{"exec", "--format", "json", "ssh", "user@host"},
	}
	for _, c := range cmds {
		_ = runAsPasswordUser(t, c, nil)
	}
	entries := readAuditLines(t, auditLogPath(t, home))
	if len(entries) != len(cmds) {
		t.Fatalf("expected %d audit entries; got %d", len(cmds), len(entries))
	}
	last := entries[len(entries)-1]
	if d, _ := last["decision"].(string); d != "block" {
		t.Errorf("expected last decision=block; got %q (entry=%+v)", d, last)
	}
}

func TestDetect_FindsKnownAgents(t *testing.T) {
	requireLinux(t)
	if !sudoNoPasswdAvailable() {
		t.Skip("sudo NOPASSWD not available")
	}
	home, _ := freshHome(t)
	writeCacheConfig(t, home, "")
	out := run(t, []string{"detect"}, nil)
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
	writeCacheConfig(t, home, `
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
	if !sudoNoPasswdAvailable() {
		t.Skip("sudo NOPASSWD not available")
	}
	home, _ := freshHome(t)
	writeCacheConfig(t, home, "")
	out := run(t, []string{"policy", "test", "ssh", "user@host"}, nil)
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

// detectSafeCommand returns a benign command that is universally
// available across the supported distros.
func detectSafeCommand(t *testing.T) string {
	t.Helper()
	candidates := []string{"apt-get", "dnf", "pacman", "id"}
	for _, c := range candidates {
		if _, err := exec.LookPath(c); err == nil {
			if c == "id" {
				return "id"
			}
			return c + " --version"
		}
	}
	return "true"
}
