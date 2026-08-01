package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// runInstallCmd runs the install/uninstall cobra command with the
// given args and returns captured stdout, stderr, and the exit error.
// HOME is redirected to dir for the duration of the call so adapter
// writes land in a temp directory.
func runInstallCmd(t *testing.T, app *App, dir string, args ...string) (string, string, error) {
	t.Helper()
	t.Setenv("HOME", dir)
	cmd := NewRootCmd(app, "test", "abc", "now")
	var out, errBuf bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errBuf)
	cmd.SetArgs(append([]string{"install"}, args...))
	err := cmd.Execute()
	return out.String(), errBuf.String(), err
}

func runUninstallCmd(t *testing.T, app *App, dir string, args ...string) (string, string, error) {
	t.Helper()
	t.Setenv("HOME", dir)
	cmd := NewRootCmd(app, "test", "abc", "now")
	var out, errBuf bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errBuf)
	cmd.SetArgs(append([]string{"install", "uninstall"}, args...))
	err := cmd.Execute()
	return out.String(), errBuf.String(), err
}

func TestInstallCmd_ExplicitKind_WritesFile(t *testing.T) {
	dir := t.TempDir()
	app := stubApp(t)
	fmtImpl, _ := NewFormatter(FormatHuman, false)
	app.Formatter = fmtImpl

	stdout, _, err := runInstallCmd(t, app, dir,
		"--kind", "kilo",
		"--yes",
	)
	if err != nil {
		t.Fatalf("install: %v", err)
	}
	if !strings.Contains(stdout, "Kilo CLI") {
		t.Errorf("expected Kilo CLI in output; got %q", stdout)
	}
	// Verify the file actually landed in the redirected HOME.
	target := filepath.Join(dir, ".config/kilo/commands/sudoconsole.md")
	data, err := os.ReadFile(target) // #nosec G304 -- test asserts file was written by adapter
	if err != nil {
		t.Fatalf("expected %s to exist: %v", target, err)
	}
	if !strings.Contains(string(data), "sudoconsole-marker: kilo") {
		t.Errorf("missing marker in %s; got %q", target, string(data))
	}
}

func TestInstallCmd_DryRun_DoesNotWriteFile(t *testing.T) {
	dir := t.TempDir()
	app := stubApp(t)
	fmtImpl, _ := NewFormatter(FormatJSON, false)
	app.Formatter = fmtImpl

	stdout, _, err := runInstallCmd(t, app, dir,
		"--kind", "kilo",
		"--dry-run",
		"--yes",
	)
	if err != nil {
		t.Fatalf("install --dry-run: %v", err)
	}

	var got InstallResult
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("json: %v body=%q", err, stdout)
	}
	if !got.Planned {
		t.Errorf("expected Planned=true; got %+v", got)
	}
	if len(got.Installed) != 1 || got.Installed[0].Kind != "kilo" {
		t.Errorf("expected kilo in Installed; got %+v", got.Installed)
	}
	if !got.Installed[0].DryRun {
		t.Errorf("expected DryRun flag on outcome; got %+v", got.Installed[0])
	}

	target := filepath.Join(dir, ".config/kilo/commands/sudoconsole.md")
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Errorf("dry-run should NOT write %s; stat err = %v", target, err)
	}
}

func TestInstallCmd_Idempotent(t *testing.T) {
	dir := t.TempDir()
	app := stubApp(t)
	fmtImpl, _ := NewFormatter(FormatJSON, false)
	app.Formatter = fmtImpl

	for i := 0; i < 2; i++ {
		_, _, err := runInstallCmd(t, app, dir,
			"--kind", "kilo",
			"--yes",
		)
		if err != nil {
			t.Fatalf("install #%d: %v", i, err)
		}
	}
	target := filepath.Join(dir, ".config/kilo/commands/sudoconsole.md")
	data, err := os.ReadFile(target) // #nosec G304 -- test reads file written by adapter
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	// Marker must appear exactly once (no duplicate writes).
	if c := strings.Count(string(data), "sudoconsole-marker: kilo"); c != 1 {
		t.Errorf("expected exactly 1 marker; got %d in:\n%s", c, string(data))
	}
}

func TestInstallCmd_UnknownKind(t *testing.T) {
	dir := t.TempDir()
	app := stubApp(t)
	_, _, err := runInstallCmd(t, app, dir,
		"--kind", "bogus",
		"--yes",
	)
	if err == nil {
		t.Fatal("expected error for unknown kind")
	}
	if !strings.Contains(err.Error(), "unknown kind") {
		t.Errorf("expected unknown-kind error; got %v", err)
	}
}

func TestInstallCmd_PolicyModeOverride(t *testing.T) {
	dir := t.TempDir()
	app := stubApp(t)
	fmtImpl, _ := NewFormatter(FormatHuman, false)
	app.Formatter = fmtImpl

	stdout, _, err := runInstallCmd(t, app, dir,
		"--kind", "kilo",
		"--policy-mode", "audit",
		"--yes",
	)
	if err != nil {
		t.Fatalf("install: %v", err)
	}
	if !strings.Contains(stdout, "Kilo CLI") {
		t.Errorf("expected Kilo CLI; got %q", stdout)
	}
}

func TestInstallCmd_PolicyModeInvalid(t *testing.T) {
	dir := t.TempDir()
	app := stubApp(t)
	_, _, err := runInstallCmd(t, app, dir,
		"--kind", "kilo",
		"--policy-mode", "nonsense",
		"--yes",
	)
	if err == nil {
		t.Fatal("expected error for invalid policy mode")
	}
	if !strings.Contains(err.Error(), "invalid --policy-mode") {
		t.Errorf("expected invalid-policy-mode error; got %v", err)
	}
}

func TestUninstallCmd_RemovesMarker(t *testing.T) {
	dir := t.TempDir()
	app := stubApp(t)
	fmtImpl, _ := NewFormatter(FormatJSON, false)
	app.Formatter = fmtImpl

	// First install.
	if _, _, err := runInstallCmd(t, app, dir, "--kind", "kilo", "--yes"); err != nil {
		t.Fatalf("install: %v", err)
	}
	target := filepath.Join(dir, ".config/kilo/commands/sudoconsole.md")
	if _, err := os.Stat(target); err != nil {
		t.Fatalf("post-install: file missing: %v", err)
	}

	// Then uninstall.
	stdout, _, err := runUninstallCmd(t, app, dir, "--kind", "kilo", "--yes")
	if err != nil {
		t.Fatalf("uninstall: %v", err)
	}
	var got InstallResult
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("json: %v body=%q", err, stdout)
	}
	if len(got.Installed) != 1 {
		t.Errorf("expected 1 uninstalled; got %+v", got.Installed)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Errorf("expected file removed; stat err = %v", err)
	}

	// Idempotent: second uninstall is also OK.
	if _, _, err := runUninstallCmd(t, app, dir, "--kind", "kilo", "--yes"); err != nil {
		t.Errorf("second uninstall should be idempotent: %v", err)
	}
}

func TestInstallCmd_BinDirFlag(t *testing.T) {
	dir := t.TempDir()
	app := stubApp(t)
	fmtImpl, _ := NewFormatter(FormatHuman, false)
	app.Formatter = fmtImpl

	// bin-dir is propagated to InstallOptions.BinDir (no assertion on
	// the FS since most adapters don't actually write wrappers yet —
	// the smoke test is that the flag is accepted and install runs).
	if _, _, err := runInstallCmd(t, app, dir,
		"--kind", "kilo",
		"--bin-dir", "/tmp/sudoconsole-bin",
		"--yes",
	); err != nil {
		t.Fatalf("install with --bin-dir: %v", err)
	}
}

func TestInstallCmd_YesFlag_SkipsConfirm(t *testing.T) {
	dir := t.TempDir()
	app := stubApp(t)
	fmtImpl, _ := NewFormatter(FormatHuman, false)
	app.Formatter = fmtImpl

	stdout, _, err := runInstallCmd(t, app, dir,
		"--kind", "kilo",
		"--yes",
	)
	if err != nil {
		t.Fatalf("install --yes: %v", err)
	}
	if strings.Contains(stdout, "Proceed?") {
		t.Errorf("--yes should suppress confirm prompt; got %q", stdout)
	}
}

func TestInstallCmd_NoYes_RejectsOnNoAnswer(t *testing.T) {
	dir := t.TempDir()
	app := stubApp(t)

	// Inject a stdin that returns EOF immediately, simulating a
	// non-interactive run without --yes. The CLI's buildConfirm will
	// return false on EOF, and the use case should return
	// ErrInstallAborted.
	cmd := NewRootCmd(app, "test", "abc", "now")
	var out, errBuf bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errBuf)
	cmd.SetArgs([]string{"install", "--kind", "kilo"})
	// Replace stdin with an empty reader.
	cmd.SetIn(strings.NewReader(""))
	t.Setenv("HOME", dir)
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected ErrInstallAborted on empty stdin")
	}
	if !strings.Contains(err.Error(), "aborted") {
		t.Errorf("expected aborted error; got %v", err)
	}
}

func TestInstallCmd_NoYes_AcceptsOnYesAnswer(t *testing.T) {
	dir := t.TempDir()
	app := stubApp(t)

	cmd := NewRootCmd(app, "test", "abc", "now")
	var out, errBuf bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errBuf)
	cmd.SetArgs([]string{"install", "--kind", "kilo"})
	cmd.SetIn(strings.NewReader("y\n"))
	t.Setenv("HOME", dir)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("install with 'y' answer: %v", err)
	}
	target := filepath.Join(dir, ".config/kilo/commands/sudoconsole.md")
	if _, err := os.Stat(target); err != nil {
		t.Errorf("expected file written: %v", err)
	}
}

// silence unused import warnings if io is not consumed by some builds.
var _ = io.Discard
