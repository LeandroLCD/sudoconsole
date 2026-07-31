// Package audit implements domain.AuditLogger.
//
// Two backends are provided:
//
//   - NoopLogger: discards every event. Used when the user has not
//     configured a log file.
//
//   - FileLogger: appends JSONL records to a regular file, rotating
//     when the file exceeds MaxBytes. Mode is 0600 because audit
//     records may contain command lines that hint at credentials.
//
// All implementations share the enrichment contract: a Logger that
// supports enrichment (interface Enricher) fills in the timestamp,
// hostname, user and session ID before writing, so callers can leave
// those fields zero.
package audit

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/LeandroLCD/sudoconsole/internal/domain"
)

// NoopLogger discards every event. Safe for concurrent use.
type NoopLogger struct{}

// Log implements domain.AuditLogger.
func (NoopLogger) Log(_ context.Context, _ domain.AuditEvent) error { return nil }

// Close implements domain.AuditLogger.
func (NoopLogger) Close() error { return nil }

// Enricher is the contract satisfied by FileLogger: a Logger that
// fills in the host/user/session metadata before writing.
type Enricher interface {
	domain.AuditLogger
}

// Context holds the per-process metadata auto-populated by the
// enrichment layer (hostname, user, session id). It is intentionally
// not exported — only the audit package fills these fields in.
type meta struct {
	Hostname   string
	User       string
	SessionID  string
	PolicyHash string
}

// detect populates the Context from the host environment. Errors are
// non-fatal: blank fields are accepted.
func detect() meta {
	host, _ := os.Hostname()
	user := os.Getenv("USER")
	if user == "" {
		user = os.Getenv("LOGNAME")
	}
	return meta{
		Hostname:  host,
		User:      user,
		SessionID: newSessionID(),
	}
}

func newSessionID() string {
	var b [16]byte
	now := uint64(time.Now().UnixNano()) // #nosec G115 -- session id is not security-critical; truncating to 8 bytes is intentional.
	for i := 0; i < 8; i++ {
		b[i] = byte(now >> (8 * i)) // #nosec G115 -- intentional truncation; session id is not security-critical.
	}
	return hex.EncodeToString(b[:])
}

// HashPolicy returns a short identifier (first 8 hex chars of a
// sha256) for the supplied policy. The hash covers every field so any
// user-facing change produces a new value.
func HashPolicy(p domain.Policy) string {
	// Stable, deterministic projection. We rely on Go's default JSON
	// encoding of the struct (field order is fixed by the type).
	raw, _ := json.Marshal(p)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:4])
}

// --- FileLogger ---------------------------------------------------------

// FileLogger writes JSONL records to a file, rotating when the file
// exceeds MaxBytes. Safe for concurrent use.
type FileLogger struct {
	mu       sync.Mutex
	path     string
	maxBytes int64
	out      io.WriteCloser
	curSize  int64
	meta     meta
	now      func() time.Time
}

// NewFileLogger opens (or creates) the log file at path. maxBytes <= 0
// disables rotation. The file is opened with mode 0600.
func NewFileLogger(path string, maxBytes int64) (*FileLogger, error) {
	if path == "" {
		return nil, fmt.Errorf("audit: empty path")
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600) // #nosec G304 -- path is the user-supplied audit log location.
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	info, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("stat %s: %w", path, err)
	}
	return &FileLogger{
		path:     path,
		maxBytes: maxBytes,
		out:      f,
		curSize:  info.Size(),
		meta:     detect(),
		now:      time.Now,
	}, nil
}

// SetPolicyHash updates the policy hash recorded on subsequent events.
// Safe to call from any goroutine.
func (l *FileLogger) SetPolicyHash(h string) {
	l.mu.Lock()
	l.meta.PolicyHash = h
	l.mu.Unlock()
}

// Log serializes the event as JSONL and appends it. Enrichment happens
// here so callers can leave the host/user/session fields zero.
//
// On rotation, the file is closed and renamed with a UTC timestamp
// suffix (".YYYYMMDDTHHMMSSZ"); a fresh file is opened in its place.
func (l *FileLogger) Log(_ context.Context, event domain.AuditEvent) error {
	if l == nil || l.out == nil {
		return nil
	}
	enriched := l.enrich(event)
	rec, err := json.Marshal(enriched)
	if err != nil {
		return fmt.Errorf("audit marshal: %w", err)
	}
	rec = append(rec, '\n')

	l.mu.Lock()
	defer l.mu.Unlock()

	if l.maxBytes > 0 && l.curSize+int64(len(rec)) > l.maxBytes {
		if err := l.rotate(); err != nil {
			return fmt.Errorf("audit rotate: %w", err)
		}
	}
	n, err := l.out.Write(rec)
	l.curSize += int64(n)
	return err
}

// enrich fills the empty fields with values from meta and redacts any
// secret-looking argument from the command line.
func (l *FileLogger) enrich(event domain.AuditEvent) domain.AuditEvent {
	if event.Timestamp.IsZero() {
		event.Timestamp = l.now().UTC()
	}
	if event.Hostname == "" {
		event.Hostname = l.meta.Hostname
	}
	if event.User == "" {
		event.User = l.meta.User
	}
	if event.SessionID == "" {
		event.SessionID = l.meta.SessionID
	}
	if event.PolicyHash == "" {
		event.PolicyHash = l.meta.PolicyHash
	}
	if event.Command.Path != "" {
		event.Command, event.Redacted = RedactCommand(event.Command)
	}
	return event
}

// Close flushes and closes the underlying file.
func (l *FileLogger) Close() error {
	if l == nil || l.out == nil {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	err := l.out.Close()
	l.out = nil
	return err
}

// rotate closes the file, renames it to "<path>.YYYYMMDDTHHMMSSZ" (UTC)
// and opens a fresh one. Must be called with mu held.
func (l *FileLogger) rotate() error {
	if err := l.out.Close(); err != nil {
		return fmt.Errorf("close: %w", err)
	}
	suffix := l.now().UTC().Format("20060102T150405Z")
	rotated := l.path + "." + suffix
	if err := os.Rename(l.path, rotated); err != nil {
		return fmt.Errorf("rename: %w", err)
	}
	f, err := os.OpenFile(l.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return fmt.Errorf("reopen: %w", err)
	}
	l.out = f
	l.curSize = 0
	return nil
}

// RedactCommand replaces secret-looking arguments in the command line
// with `<REDACTED>`. It mutates a copy of the command and reports
// whether anything was changed.
//
// Heuristics:
//
//   - The first positional argument following `sudo -S` (the password
//     read from stdin) is NOT visible in argv; we do not redact it.
//   - Any argument starting with `--password=`, `-p=` or `-p` followed
//     by a value is redacted.
//   - `curl --data-urlencode pass=<x>` style arguments are redacted by
//     matching the substrings `password=`, `passwd=`, `token=` and
//     `secret=` followed by the value (best-effort).
//   - `env VAR=secret` style assignments are scrubbed when the value
//     is non-empty and the key matches one of the above.
func RedactCommand(c domain.Command) (domain.Command, bool) {
	if c.Path == "" {
		return c, false
	}
	redacted := false
	args := make([]string, len(c.Args))
	copy(args, c.Args)
	for i, a := range args {
		new, ok := redactArg(c.Basename(), a)
		if ok {
			args[i] = new
			redacted = true
		}
	}
	if !redacted {
		return c, false
	}
	c.Args = args
	c.Raw = buildRedactedRaw(c)
	return c, true
}

func redactArg(binary, arg string) (string, bool) {
	lower := strings.ToLower(arg)
	if new, ok := redactFlag(lower, arg); ok {
		return new, true
	}
	if new, ok := redactEmbedded(lower, arg); ok {
		return new, true
	}
	if new, ok := redactEnvVar(arg); ok {
		return new, true
	}
	if new, ok := redactShortPassword(binary, arg); ok {
		return new, true
	}
	return arg, false
}

// redactFlag handles --password=, --passwd=, --token=, --secret=.
func redactFlag(lower, arg string) (string, bool) {
	for _, prefix := range []string{"--password=", "--passwd=", "--token=", "--secret="} {
		if strings.HasPrefix(lower, prefix) {
			return prefix + "<REDACTED>", true
		}
	}
	return arg, false
}

// redactEmbedded redacts value fragments inside `--data` style args.
func redactEmbedded(lower, arg string) (string, bool) {
	for _, marker := range []string{"password=", "passwd=", "token=", "secret="} {
		if !strings.Contains(lower, marker) {
			continue
		}
		idx := strings.Index(lower, marker)
		value := arg[idx+len(marker):]
		end := len(value)
		for j, r := range value {
			if r == ' ' || r == '\'' || r == '"' || r == '\\' {
				end = j
				break
			}
		}
		if end > 0 {
			return arg[:idx+len(marker)] + "<REDACTED>" + arg[idx+len(marker)+end:], true
		}
	}
	return arg, false
}

// redactEnvVar redacts KEY=VALUE style assignments when KEY matches a
// sensitive key list.
func redactEnvVar(arg string) (string, bool) {
	eq := strings.IndexByte(arg, '=')
	if eq <= 0 {
		return arg, false
	}
	key := strings.ToLower(arg[:eq])
	switch key {
	case "password", "passwd", "token", "secret", "sudo_password":
		if arg[eq+1:] == "" {
			return arg, false
		}
		return arg[:eq+1] + "<REDACTED>", true
	}
	return arg, false
}

// redactShortPassword handles `mysql -p<secret>` style flags.
func redactShortPassword(binary, arg string) (string, bool) {
	switch binary {
	case "mysqldump", "mysql", "pg_dump":
	default:
		return arg, false
	}
	if strings.HasPrefix(arg, "-p") && len(arg) > 2 {
		return "-p<REDACTED>", true
	}
	return arg, false
}

func buildRedactedRaw(c domain.Command) string {
	parts := make([]string, 0, 1+len(c.Args))
	parts = append(parts, c.Path)
	parts = append(parts, c.Args...)
	return strings.Join(parts, " ")
}

// Path returns the resolved file path the logger is writing to. Used
// by the `sudoconsole audit tail` command.
func (l *FileLogger) Path() string {
	if l == nil {
		return ""
	}
	return l.path
}
