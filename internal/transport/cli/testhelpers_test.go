package cli

import (
	"io"
	"testing"

	"github.com/LeandroLCD/sudoconsole/internal/domain"
	"github.com/LeandroLCD/sudoconsole/internal/infrastructure/audit"
)

// stubApp returns a minimal App suitable for tests that don't exercise
// the use cases (e.g. flag parsing, help text).
func stubApp(_ *testing.T) *App {
	fmt, _ := NewFormatter(FormatHuman, false)
	return &App{
		Config:        domain.DefaultConfig(),
		Audit:         audit.NoopLogger{},
		AgentDetector: nil,
		Formatter:     fmt,
		Logger:        NewLogger(io.Discard, LogSilent),
		Now:           func() string { return "2026-01-01T00:00:00Z" },
	}
}
