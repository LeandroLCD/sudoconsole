package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
)

// LogLevel is the parsed representation of the --log-level flag.
type LogLevel slog.Level

const (
	LogSilent LogLevel = LogLevel(slog.LevelError + 1)
	LogError  LogLevel = LogLevel(slog.LevelError)
	LogWarn   LogLevel = LogLevel(slog.LevelWarn)
	LogInfo   LogLevel = LogLevel(slog.LevelInfo)
	LogDebug  LogLevel = LogLevel(slog.LevelDebug)
)

// ParseLogLevel maps the CLI flag to a LogLevel. The empty string maps
// to LogInfo so the documented default keeps emitting operational logs.
func ParseLogLevel(s string) (LogLevel, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "info":
		return LogInfo, nil
	case "silent":
		return LogSilent, nil
	case "error":
		return LogError, nil
	case "warn", "warning":
		return LogWarn, nil
	case "debug":
		return LogDebug, nil
	default:
		return LogInfo, fmt.Errorf("invalid log level %q (want silent, error, warn, info, debug)", s)
	}
}

// Logger wraps slog.Logger with a level filter and a couple of
// convenience methods used by the CLI commands.
type Logger struct {
	*slog.Logger
	level LogLevel
}

// NewLogger builds a Logger writing to w at the given level. When
// level is LogSilent the logger never emits anything; the underlying
// slog handler is still constructed to keep behaviour consistent.
func NewLogger(w io.Writer, level LogLevel) *Logger {
	opts := &slog.HandlerOptions{Level: slog.Level(level)}
	var h slog.Handler = slog.NewTextHandler(w, opts)
	if level == LogSilent {
		h = silentHandler{}
	}
	return &Logger{
		Logger: slog.New(h),
		level:  level,
	}
}

// Enabled reports whether lvl would produce output.
func (l *Logger) Enabled(lvl LogLevel) bool {
	if l == nil {
		return false
	}
	return l.level <= lvl
}

// Error logs an error message and returns the wrapped error so callers
// can do `return logger.Error(...)` idiomatically.
func (l *Logger) Error(msg string, err error, attrs ...any) error {
	if l.Enabled(LogError) && err != nil {
		attrs = append(attrs, "error", err.Error())
	}
	l.Log(context.Background(), slog.LevelError, msg, attrs...)
	return err
}

// silentHandler satisfies slog.Handler while discarding every record.
type silentHandler struct{}

func (silentHandler) Enabled(_ context.Context, _ slog.Level) bool  { return false }
func (silentHandler) Handle(_ context.Context, _ slog.Record) error { return nil }
func (silentHandler) WithAttrs(_ []slog.Attr) slog.Handler          { return silentHandler{} }
func (silentHandler) WithGroup(_ string) slog.Handler               { return silentHandler{} }

// ErrInvalidLogLevel is returned by ParseLogLevel when the input does
// not match any known level.
var ErrInvalidLogLevel = errors.New("invalid log level")
