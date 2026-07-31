package audit

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LeandroLCD/sudoconsole/internal/domain"
)

func TestNoopLogger(t *testing.T) {
	var l domain.AuditLogger = NoopLogger{}
	if err := l.Log(context.Background(), domain.AuditEvent{}); err != nil {
		t.Errorf("Log: %v", err)
	}
	if err := l.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}
}

func TestFileLogger_WritesJSONL(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.jsonl")
	l, err := NewFileLogger(path, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	evt := domain.AuditEvent{
		User:     "alice",
		Hostname: "host1",
		Command:  domain.Command{Path: "apt", Args: []string{"update"}},
		Decision: domain.DecisionAudit,
		Notes:    "test",
	}
	if err := l.Log(context.Background(), evt); err != nil {
		t.Fatalf("Log: %v", err)
	}
	if err := l.Log(context.Background(), evt); err != nil {
		t.Fatalf("Log 2: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	lines := bytes.Split(bytes.TrimRight(data, "\n"), []byte{'\n'})
	if len(lines) != 2 {
		t.Fatalf("want 2 lines, got %d", len(lines))
	}
	var got domain.AuditEvent
	if err := json.Unmarshal(lines[0], &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.User != "alice" || got.Hostname != "host1" || got.Decision != domain.DecisionAudit {
		t.Errorf("event mismatch: %+v", got)
	}
	if got.Command.Path != "apt" {
		t.Errorf("Command.Path = %q", got.Command.Path)
	}
}

func TestFileLogger_RotatesAtMaxBytes(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.jsonl")
	// tiny budget: a single record should already exceed it.
	l, err := NewFileLogger(path, 10)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	evt := domain.AuditEvent{User: "u", Notes: strings.Repeat("x", 50)}
	if err := l.Log(context.Background(), evt); err != nil {
		t.Fatalf("Log 1: %v", err)
	}
	if err := l.Log(context.Background(), evt); err != nil {
		t.Fatalf("Log 2 (post-rotate): %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("log file missing after rotation: %v", err)
	}
	if _, err := os.Stat(path + ".1"); err != nil {
		t.Fatalf("rotated file missing: %v", err)
	}
}

func TestFileLogger_OpenFailure(t *testing.T) {
	if _, err := NewFileLogger("", 0); err == nil {
		t.Error("empty path should fail")
	}
	if _, err := NewFileLogger(filepath.Join(t.TempDir(), "missing-dir", "x.toml"), 0); err == nil {
		t.Error("missing dir should fail")
	}
}

func TestFileLogger_CloseIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.jsonl")
	l, err := NewFileLogger(path, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := l.Close(); err != nil {
		t.Errorf("first close: %v", err)
	}
	if err := l.Close(); err != nil {
		t.Errorf("second close: %v", err)
	}
}

func TestFileLogger_ImplementsAuditLogger(t *testing.T) {
	var _ domain.AuditLogger = (*FileLogger)(nil)
}
