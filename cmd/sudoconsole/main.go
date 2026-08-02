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
	"strings"
	"syscall"

	"github.com/LeandroLCD/sudoconsole/internal/domain"
	"github.com/LeandroLCD/sudoconsole/internal/infrastructure/agent"
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
	cfgPath := resolveConfigPath()
	store := config.NewAt(cfgPath)
	cfg, err := store.Load(ctx)
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}
	applyFlagOverrides(&cfg)

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
		fileAud.SetPolicyHash(audit.HashPolicy(cfg.Policy))
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
		Config:        cfg,
		Gateway:       sudoGw,
		Repository:    repo,
		Evaluator:     eval,
		Audit:         aud,
		AgentDetector: agent.NewDetector(),
		Logger:        logger,
		Formatter:     fmtImpl,
		Now:           cli.Now,
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

// resolveConfigPath inspects os.Args and the SUDOCONSOLE_CONFIG env
// var for the --config flag value. Returns "" to use the platform
// default.
func resolveConfigPath() string {
	if env := os.Getenv("SUDOCONSOLE_CONFIG"); env != "" {
		return env
	}
	args := os.Args[1:]
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--config" && i+1 < len(args):
			return args[i+1]
		case a == "-config" && i+1 < len(args):
			return args[i+1]
		case strings.HasPrefix(a, "--config="):
			return strings.TrimPrefix(a, "--config=")
		case strings.HasPrefix(a, "-config="):
			return strings.TrimPrefix(a, "-config=")
		}
	}
	return ""
}

// applyFlagOverrides reads the persistent flags directly from os.Args
// and layers them on top of cfg. Empty/zero values are ignored so the
// underlying config stays intact.
func applyFlagOverrides(cfg *domain.Config) {
	args := os.Args[1:]
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--cache-timeout" && i+1 < len(args):
			if v, err := strconvAtoi(args[i+1]); err == nil && v > 0 {
				cfg.Cache.TimeoutSeconds = v
			}
		case strings.HasPrefix(a, "--cache-timeout="):
			if v, err := strconvAtoi(strings.TrimPrefix(a, "--cache-timeout=")); err == nil && v > 0 {
				cfg.Cache.TimeoutSeconds = v
			}
		case a == "--format" && i+1 < len(args):
			cfg.Output.Format = args[i+1]
		case strings.HasPrefix(a, "--format="):
			cfg.Output.Format = strings.TrimPrefix(a, "--format=")
		case a == "--log-level" && i+1 < len(args):
			cfg.Output.LogLevel = args[i+1]
		case strings.HasPrefix(a, "--log-level="):
			cfg.Output.LogLevel = strings.TrimPrefix(a, "--log-level=")
		}
	}
}

func strconvAtoi(s string) (int, error) {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("not a number: %q", s)
		}
		n = n*10 + int(c-'0')
	}
	return n, nil
}

// versionInfo is exposed for the --version flag handling.
var _ = runtime.Version
