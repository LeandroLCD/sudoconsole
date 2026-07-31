package audit

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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

func TestFileLogger_WritesEnrichedJSONL(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.jsonl")
	l, err := NewFileLogger(path, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	evt := domain.AuditEvent{
		Command:  domain.Command{Path: "apt", Args: []string{"update"}},
		Decision: domain.DecisionAudit,
		Notes:    "test",
	}
	if err := l.Log(context.Background(), evt); err != nil {
		t.Fatalf("Log: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var got domain.AuditEvent
	if err := json.Unmarshal(bytes.TrimRight(data, "\n"), &got); err != nil {
		t.Fatalf("unmarshal: %v: %q", err, data)
	}
	if got.Command.Path != "apt" {
		t.Errorf("Command.Path = %q", got.Command.Path)
	}
	if got.User == "" {
		t.Errorf("User not enriched")
	}
	if got.Hostname == "" {
		t.Errorf("Hostname not enriched")
	}
	if got.SessionID == "" {
		t.Errorf("SessionID not enriched")
	}
	if got.Timestamp.IsZero() {
		t.Errorf("Timestamp not enriched")
	}
}

func TestFileLogger_PolicyHashHonoured(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.jsonl")
	l, err := NewFileLogger(path, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	l.SetPolicyHash("deadbeef")
	if err := l.Log(context.Background(), domain.AuditEvent{}); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	var got domain.AuditEvent
	_ = json.Unmarshal(bytes.TrimRight(data, "\n"), &got)
	if got.PolicyHash != "deadbeef" {
		t.Errorf("PolicyHash = %q", got.PolicyHash)
	}
}

func TestFileLogger_RotatesWithTimestampSuffix(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.jsonl")
	l, err := NewFileLogger(path, 10)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	// Pin the clock so we can assert the suffix.
	l.now = func() time.Time { return time.Date(2026, 7, 31, 12, 0, 0, 0, time.UTC) }
	evt := domain.AuditEvent{User: "u", Notes: strings.Repeat("x", 50)}
	if err := l.Log(context.Background(), evt); err != nil {
		t.Fatalf("Log 1: %v", err)
	}
	if err := l.Log(context.Background(), evt); err != nil {
		t.Fatalf("Log 2 (post-rotate): %v", err)
	}
	rotated := path + ".20260731T120000Z"
	if _, err := os.Stat(rotated); err != nil {
		t.Fatalf("rotated file missing: %v (want %s)", err, rotated)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("fresh file missing after rotation: %v", err)
	}
}

func TestFileLogger_OpenFailure(t *testing.T) {
	if _, err := NewFileLogger("", 0); err == nil {
		t.Error("empty path should fail")
	}
	if _, err := NewFileLogger(filepath.Join(t.TempDir(), "missing-dir", "x.jsonl"), 0); err == nil {
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

// --- redaction ---------------------------------------------------------

func TestRedactCommand_NoChange(t *testing.T) {
	c := domain.Command{Path: "apt", Args: []string{"update"}}
	got, redacted := RedactCommand(c)
	if redacted {
		t.Errorf("expected no redaction for %v", c)
	}
	if got.Path != "apt" {
		t.Errorf("Path mutated: %q", got.Path)
	}
}

func TestRedactCommand_LongFlag(t *testing.T) {
	c := domain.Command{Path: "curl", Args: []string{"--data-urlencode", "password=hunter2", "https://x"}}
	got, ok := RedactCommand(c)
	if !ok {
		t.Fatal("expected redaction")
	}
	if !strings.Contains(got.Args[1], "<REDACTED>") {
		t.Errorf("arg 1 not redacted: %q", got.Args[1])
	}
	if strings.Contains(got.Args[1], "hunter2") {
		t.Errorf("password leaked: %q", got.Args[1])
	}
}

func TestRedactCommand_MysqlPasswordFlag(t *testing.T) {
	c := domain.Command{Path: "mysql", Args: []string{"-psecret", "-u", "root"}}
	got, ok := RedactCommand(c)
	if !ok {
		t.Fatal("expected redaction")
	}
	if got.Args[0] != "-p<REDACTED>" {
		t.Errorf("arg 0 = %q", got.Args[0])
	}
}

func TestRedactCommand_EnvVar(t *testing.T) {
	c := domain.Command{Path: "sh", Args: []string{"-c", "echo $TOKEN"}}
	// Arg is "echo $TOKEN", no key=value pair → not redacted.
	_, ok := RedactCommand(c)
	if ok {
		t.Error("echo without key=value should not be redacted")
	}
	// Now with a direct env assignment in argv.
	c2 := domain.Command{Path: "env", Args: []string{"SUDO_PASSWORD=hunter2", "echo", "hi"}}
	got, ok := RedactCommand(c2)
	if !ok {
		t.Fatal("expected env redaction")
	}
	if !strings.Contains(got.Args[0], "<REDACTED>") || strings.Contains(got.Args[0], "hunter2") {
		t.Errorf("env arg not redacted: %q", got.Args[0])
	}
}

func TestRedactCommand_SudoDashS(t *testing.T) {
	// `sudo -S` reads password from stdin (the next arg is NOT the
	// password in argv); we should not redact the binary name itself.
	c := domain.Command{Path: "sudo", Args: []string{"-S", "-p", "", "apt", "update"}}
	_, ok := RedactCommand(c)
	if ok {
		t.Error("sudo -S with no password in argv should not redact")
	}
}

func TestHashPolicy_Stable(t *testing.T) {
	p := domain.DefaultPolicy()
	h1 := HashPolicy(p)
	h2 := HashPolicy(p)
	if h1 != h2 {
		t.Errorf("HashPolicy not stable: %q != %q", h1, h2)
	}
	if len(h1) != 8 {
		t.Errorf("HashPolicy length = %d, want 8", len(h1))
	}
	p2 := p
	p2.Mode = domain.PolicyModeAllowlist
	if HashPolicy(p2) == h1 {
		t.Error("HashPolicy should change when policy changes")
	}
}

// --- tail --------------------------------------------------------------

func writeAuditLines(t *testing.T, path string, n int) {
	t.Helper()
	f, err := os.Create(path) // #nosec G304 -- test fixture path.
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	for i := 0; i < n; i++ {
		_ = enc.Encode(domain.AuditEvent{
			Timestamp: time.Unix(int64(i), 0).UTC(),
			Notes:     "line",
			Command:   domain.Command{Path: "echo", Args: []string{"hi"}},
			Decision:  domain.DecisionAllow,
		})
	}
}

func TestTail_LastN(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.jsonl")
	writeAuditLines(t, path, 100)
	got, err := Tail(TailOptions{Path: path, N: 5})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 5 {
		t.Fatalf("want 5, got %d", len(got))
	}
	if got[0].Notes != "line" || got[4].Notes != "line" {
		t.Errorf("tail contents unexpected: %+v", got)
	}
}

func TestTail_ZeroReturnsAll(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.jsonl")
	writeAuditLines(t, path, 10)
	got, err := Tail(TailOptions{Path: path, N: 0})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 10 {
		t.Errorf("want 10, got %d", len(got))
	}
}

func TestTail_SkipsMalformed(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.jsonl")
	if err := os.WriteFile(path, []byte("not json\n"+`{"Notes":"ok"}`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := Tail(TailOptions{Path: path, N: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Notes != "ok" {
		t.Errorf("got %+v", got)
	}
}

func TestTail_MissingFile(t *testing.T) {
	if _, err := Tail(TailOptions{Path: "/nonexistent/audit.jsonl"}); err == nil {
		t.Error("expected error for missing file")
	}
}

func TestTail_EmptyPath(t *testing.T) {
	if _, err := Tail(TailOptions{}); err == nil {
		t.Error("expected error for empty path")
	}
}

func TestFileLogger_LogAfterClose(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.jsonl")
	l, err := NewFileLogger(path, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
	// After close the logger becomes a no-op; should not error.
	if err := l.Log(context.Background(), domain.AuditEvent{}); err != nil {
		t.Errorf("Log after Close: %v", err)
	}
}

func TestFileLogger_Path(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.jsonl")
	l, _ := NewFileLogger(path, 0)
	defer l.Close()
	if l.Path() != path {
		t.Errorf("Path() = %q", l.Path())
	}
}
