// Package main is the entry point for the sudoconsole CLI.
//
// sudoconsole is a secure sudo wrapper for CLI agents that:
//   - Prompts for the sudo password on a real TTY (no echo, no log).
//   - Caches credentials via sudo's native timestamp mechanism.
//   - Evaluates a configurable policy before executing any command.
//   - Blocks remote-access and credential-exposure commands by default.
//   - Auto-installs adapters for detected CLI agents (Kilo, Claude Code, etc.).
//
// Architecture follows Clean Architecture:
//   - internal/domain: entities and ports (no external deps)
//   - internal/usecase: use cases (depends only on domain)
//   - internal/infrastructure: adapters implementing ports (PTY, sudo, cache, policy, audit)
//   - internal/transport/cli: cobra commands (composition root)
//   - cmd/sudoconsole: main wiring
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	"github.com/LeandroLCD/sudoconsole/internal/domain"
	"github.com/LeandroLCD/sudoconsole/internal/infrastructure/audit"
	"github.com/LeandroLCD/sudoconsole/internal/infrastructure/cache"
	"github.com/LeandroLCD/sudoconsole/internal/infrastructure/config"
	"github.com/LeandroLCD/sudoconsole/internal/infrastructure/policy"
	"github.com/LeandroLCD/sudoconsole/internal/infrastructure/pty"
	"github.com/LeandroLCD/sudoconsole/internal/infrastructure/sudo"
	"github.com/LeandroLCD/sudoconsole/internal/transport/cli"
)

// Version info. Injected at build time via -ldflags.
var (
	version   = "0.1.0-dev"
	commit    = "none"
	buildDate = "unknown"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(cli.ExitCode(err))
	}
}

// run is the composition root. It builds the App, then executes the
// cobra command tree.
func run() error {
	ctx, cancel := signal.NotifyContext(context.Background(),
		os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// Load config (CLI flags will overlay the file in cmd flags).
	store := config.NewAt("")
	if env := os.Getenv("SUDOCONSOLE_CONFIG"); env != "" {
		store = config.NewAt(env)
	}
	cfg, err := store.Load(ctx)
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}

	// Wire infrastructure adapters.
	ptyGw := pty.NewGateway()
	sudoGw := sudo.NewGateway(ptyGw)
	repo := cache.NewRepository("")
	eval, err := policy.NewEvaluator(cfg.Policy)
	if err != nil {
		return fmt.Errorf("policy: %w", err)
	}
	var aud domain.AuditLogger
	if cfg.Policy.Audit.LogFile != "" {
		fileAud, err := audit.NewFileLogger(cfg.Policy.Audit.LogFile, cfg.Policy.Audit.MaxBytes)
		if err != nil {
			return fmt.Errorf("audit: %w", err)
		}
		aud = fileAud
	} else {
		aud = audit.NoopLogger{}
	}

	// Wire output formatter + logger.
	format, err := cli.ParseFormat(cfg.Output.Format)
	if err != nil {
		return fmt.Errorf("format: %w", err)
	}
	colorOn := isColorOn(os.Stdout)
	fmtImpl, err := cli.NewFormatter(format, colorOn)
	if err != nil {
		return fmt.Errorf("formatter: %w", err)
	}
	logLevel, err := cli.ParseLogLevel(cfg.Output.LogLevel)
	if err != nil {
		return fmt.Errorf("log level: %w", err)
	}
	logger := cli.NewLogger(io.Discard, logLevel)

	app := &cli.App{
		Config:     cfg,
		Gateway:    sudoGw,
		Repository: repo,
		Evaluator:  eval,
		Audit:      aud,
		Logger:     logger,
		Formatter:  fmtImpl,
		Now:        cli.Now,
	}

	// Wire the auth prompt through the PTY gateway so the password is
	// never echoed.
	cli.SetDefaultPrompt(ptyGw.ReadSecret)

	root := cli.NewRootCmd(app, version, commit, buildDate)
	if err := root.ExecuteContext(ctx); err != nil {
		return err
	}
	return nil
}

// isColorOn returns true when stdout is a TTY. Centralised so tests
// can override it.
var isColorOn = func(_ *os.File) bool {
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

// versionInfo is exposed for the --version flag handling.
var _ = runtime.Version
