package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/LeandroLCD/sudoconsole/internal/domain"
	"github.com/LeandroLCD/sudoconsole/internal/infrastructure/audit"
)

// fileAudit is a tiny wrapper around audit.FileLogger used by the CLI
// tests to simulate a populated log file on disk.
func fileAudit(t *testing.T, dir string, lines int) (path string, fl *audit.FileLogger) {
	t.Helper()
	path = filepath.Join(dir, "audit.jsonl")
	fl, err := audit.NewFileLogger(path, 0)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < lines; i++ {
		if err := fl.Log(t.Context(), domain.AuditEvent{
			Command:  domain.Command{Path: "echo", Args: []string{"hi"}},
			Decision: domain.DecisionAllow,
			Notes:    "test",
		}); err != nil {
			t.Fatal(err)
		}
	}
	return path, fl
}

func TestAuditTailCmd_JSON(t *testing.T) {
	dir := t.TempDir()
	path, fl := fileAudit(t, dir, 5)
	defer fl.Close()
	app := stubApp(t)
	app.Config.Policy.Audit.LogFile = path
	fmtImpl, _ := NewFormatter(FormatJSON, false)
	app.Formatter = fmtImpl
	stdout, _, err := cmdFromArgs(app, []string{"audit", "tail", "--count", "3"})
	if err != nil {
		t.Fatal(err)
	}
	var got AuditTailResult
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("json: %v body=%q", err, stdout)
	}
	if got.Path != path {
		t.Errorf("Path = %q", got.Path)
	}
	if got.Count != 3 {
		t.Errorf("Count = %d, want 3", got.Count)
	}
}

func TestAuditTailCmd_Human(t *testing.T) {
	dir := t.TempDir()
	path, fl := fileAudit(t, dir, 2)
	defer fl.Close()
	app := stubApp(t)
	app.Config.Policy.Audit.LogFile = path
	fmtImpl, _ := NewFormatter(FormatHuman, false)
	app.Formatter = fmtImpl
	stdout, _, err := cmdFromArgs(app, []string{"audit", "tail"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout, "audit log:") {
		t.Errorf("expected header; got %q", stdout)
	}
}

func TestAuditTailCmd_NoLogFile(t *testing.T) {
	app := stubApp(t)
	app.Config.Policy.Audit.LogFile = ""
	_, _, err := cmdFromArgs(app, []string{"audit", "tail"})
	if err == nil {
		t.Fatal("expected error when no log file configured")
	}
	if !strings.Contains(err.Error(), "policy.audit.log_file") {
		t.Errorf("error should mention the missing field; got %v", err)
	}
}

func TestAuditTailCmd_MissingFile(t *testing.T) {
	app := stubApp(t)
	app.Config.Policy.Audit.LogFile = "/tmp/sudoconsole-audit-does-not-exist.jsonl"
	_, _, err := cmdFromArgs(app, []string{"audit", "tail"})
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

// silence unused imports
var _ = bytes.NewBuffer
var _ = errors.New
var _ cobra.Command
