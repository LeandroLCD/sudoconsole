package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"runtime"

	"github.com/spf13/cobra"

	"github.com/LeandroLCD/sudoconsole/internal/domain"
	"github.com/LeandroLCD/sudoconsole/internal/usecase"
)

// App is the composition root exposed to every cobra command. Each
// command builds its use case from the dependencies below.
type App struct {
	// Config is the effective merged configuration (file + flags).
	Config domain.Config

	// Gateway authenticates with sudo and runs commands.
	Gateway domain.SudoGateway

	// Repository inspects the sudo cache.
	Repository domain.CacheRepository

	// Evaluator implements domain.PolicyEvaluator.
	Evaluator domain.PolicyEvaluator

	// Audit records every decision and outcome.
	Audit domain.AuditLogger

	// AgentDetector scans the host for installed CLI agents.
	AgentDetector domain.AgentDetector

	// Logger is the structured logger for the CLI.
	Logger *Logger

	// Formatter prints results in human or JSON form.
	Formatter Formatter

	// Now returns the current timestamp formatted for result structs.
	Now func() string
}

// authDeps captures the closures needed by the auth command. Tests
// inject overrides via the fields below.
type authDeps struct {
	UseCase *usecase.AuthUseCase
	Prompt  func(ctx context.Context, prompt string) ([]byte, error)
	Logger  *Logger
	Fmt     Formatter
	Now     func() string
}

// newAuthCmd builds `sudoconsole auth`.
func newAuthCmd(app *App) *cobra.Command {
	deps := &authDeps{
		UseCase: usecase.NewAuthUseCase(app.Gateway, app.Audit),
		Prompt:  defaultPrompt,
		Logger:  app.Logger,
		Fmt:     app.Formatter,
		Now:     app.Now,
	}
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Authenticate with sudo and cache the credentials",
		Long: `auth prompts for the sudo password on a secure PTY and
caches the credentials for the configured timeout. Safe to run from any
script; the password is never echoed, logged or written to disk.`,
		Example: `  # interactive prompt
  sudoconsole auth

  # non-interactive (testing only) via env var
  SUDOCONSOLE_PASSWORD=secret sudoconsole auth --no-tty`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runAuth(cmd.Context(), deps, cmd)
		},
	}
	cmd.Flags().Bool("no-tty", false,
		"read the password from $SUDOCONSOLE_PASSWORD instead of the TTY (testing only)")
	return cmd
}

func runAuth(ctx context.Context, d *authDeps, cmd *cobra.Command) error {
	noTTY, _ := cmd.Flags().GetBool("no-tty")

	secret, err := readSecret(ctx, d.Prompt, noTTY)
	if err != nil {
		_ = d.Fmt.Print(cmd.OutOrStdout(), &AuthResult{OK: false, Message: err.Error(), Time: d.Now()})
		return d.Logger.Error("auth: read secret", err)
	}
	// Zeroize immediately after the use case returns.
	defer func() {
		for i := range secret {
			secret[i] = 0
		}
	}()

	in := usecase.AuthInput{
		Config: domain.Config{Cache: domain.DefaultCacheConfig()},
		Secret: secret,
	}
	if err := d.UseCase.Execute(ctx, in); err != nil {
		_ = d.Fmt.Print(cmd.OutOrStdout(), &AuthResult{OK: false, Message: err.Error(), Time: d.Now()})
		return d.Logger.Error("auth: authenticate", err)
	}
	_ = d.Fmt.Print(cmd.OutOrStdout(), &AuthResult{OK: true, Time: d.Now()})
	return nil
}

// readSecret returns the password either from the PTY prompt or from
// the env var (for tests).
func readSecret(ctx context.Context, prompt func(context.Context, string) ([]byte, error), noTTY bool) ([]byte, error) {
	if noTTY {
		v := os.Getenv("SUDOCONSOLE_PASSWORD")
		if v == "" {
			return nil, errors.New("--no-tty requires $SUDOCONSOLE_PASSWORD")
		}
		return []byte(v), nil
	}
	return prompt(ctx, fmt.Sprintf("[sudo] password for %s: ", runtime.GOOS))
}

// defaultPrompt is a placeholder; the real prompt is wired by main.go
// using the PtyGateway from the composition root.
var defaultPrompt = func(_ context.Context, _ string) ([]byte, error) {
	return nil, errors.New("auth: no PTY gateway configured")
}

// exitCode maps a use-case error to the documented exit codes:
//
//	0  OK
//	1  cache miss / runtime failure
//	2  authentication failed
//	64 policy blocked
//	65 policy warn
//	66 policy override required
func exitCode(err error) int {
	switch {
	case err == nil:
		return 0
	case errors.Is(err, domain.ErrAuthFailed):
		return 2
	case errors.Is(err, domain.ErrCacheMiss):
		return 1
	case errors.Is(err, domain.ErrPolicyBlocked):
		return 64
	}
	var pve *domain.PolicyViolationError
	if errors.As(err, &pve) {
		switch pve.Result.Decision {
		case domain.DecisionBlock:
			return 64
		case domain.DecisionWarn:
			return 65
		}
	}
	return 1
}
