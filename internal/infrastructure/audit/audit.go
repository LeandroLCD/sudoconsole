// Package audit implements domain.AuditLogger.
//
// Two backends are provided:
//
//   - NoopLogger: discards every event. Used when the user has not
//     configured a log file and `--format json` does not require
//     auditing.
//
//   - FileLogger: appends JSONL records to a regular file, rotating
//     when the file exceeds MaxBytes. Mode is 0600 because audit
//     records may contain command lines that hint at credentials.
//
// Both backends satisfy the contract that a failure MUST NOT block
// command execution; the caller logs the error and continues.
package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/LeandroLCD/sudoconsole/internal/domain"
)

// NoopLogger discards every event. Safe for concurrent use.
type NoopLogger struct{}

// Log implements domain.AuditLogger.
func (NoopLogger) Log(_ context.Context, _ domain.AuditEvent) error { return nil }

// Close implements domain.AuditLogger.
func (NoopLogger) Close() error { return nil }

// FileLogger writes JSONL records to a file, rotating when the file
// exceeds MaxBytes. Safe for concurrent use.
type FileLogger struct {
	mu       sync.Mutex
	path     string
	maxBytes int64
	out      io.WriteCloser
	curSize  int64
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
	}, nil
}

// Log serializes the event as JSONL and appends it. On rotation, the
// file is closed, renamed to "<path>.1", and a fresh file is opened.
func (l *FileLogger) Log(_ context.Context, event domain.AuditEvent) error {
	if l == nil || l.out == nil {
		return nil
	}
	rec, err := json.Marshal(event)
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

// rotate closes the file, renames it to "<path>.1" (overwriting any
// previous rotation), and opens a fresh one. Must be called with mu held.
func (l *FileLogger) rotate() error {
	if err := l.out.Close(); err != nil {
		return fmt.Errorf("close: %w", err)
	}
	rotated := l.path + ".1"
	// Best-effort: ignore the failure from removing a non-existent old
	// rotation; any real error is caught by the rename below.
	_ = os.Remove(rotated)
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
