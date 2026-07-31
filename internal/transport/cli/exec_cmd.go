package cli

import (
	"errors"
	"strings"

	"github.com/spf13/cobra"

	"github.com/LeandroLCD/sudoconsole/internal/domain"
	"github.com/LeandroLCD/sudoconsole/internal/usecase"
)

// newExecCmd builds `sudoconsole exec <cmd...>`.
//
// The command takes a variable number of positional arguments that are
// concatenated (with single spaces) into a Raw command line, then
// parsed via domain.NewCommandFromRaw. Shell features (quoting,
// pipes, redirects) are intentionally NOT supported: the policy
// engine operates on the raw line so dangerous patterns remain
// visible to the matcher.
func newExecCmd(app *App) *cobra.Command {
	deps := &execDeps{
		UseCase: usecase.NewExecUseCase(app.Gateway, app.Evaluator,
			app.Repository, app.Config.Policy, app.Audit),
		Fmt:   app.Formatter,
		Now:   app.Now,
		Audit: app.Audit,
	}
	cmd := &cobra.Command{
		Use:   "exec [--] <cmd...>",
		Short: "Run a command under sudo, applying the policy",
		Long: `exec runs the supplied command under sudo. The command line is
evaluated against the active policy; on Block the command does not run
unless --policy-override is supplied. The cache is consulted first; if
expired, exec either fails fast (no secret supplied) or silently
refreshes via the password supplied via $SUDOCONSOLE_PASSWORD.`,
		Example: "  sudoconsole exec apt update\n  sudoconsole exec --policy-override systemctl restart nginx\n  sudoconsole --format json exec apt update",
		Args:    cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runExec(cmd, args, deps, app)
		},
	}
	cmd.Flags().Bool("policy-override", false,
		"bypass a Block decision and proceed anyway (audited)")
	cmd.Flags().String("user", "", "sudo --user override (advanced)")
	return cmd
}

type execDeps struct {
	UseCase *usecase.ExecUseCase
	Fmt     Formatter
	Now     func() string
	Audit   domain.AuditLogger
}

func runExec(cmd *cobra.Command, args []string, d *execDeps, app *App) error {
	override, _ := cmd.Flags().GetBool("policy-override")

	raw := strings.Join(args, " ")
	command, err := domain.NewCommandFromRaw(raw)
	if err != nil {
		_ = d.Fmt.Print(cmd.ErrOrStderr(), &ExecResult{
			Decision: "invalid",
			Reason:   err.Error(),
			Time:     d.Now(),
		})
		return err
	}

	out, err := d.UseCase.Execute(cmd.Context(), usecase.ExecInput{
		Config:   app.Config,
		Command:  command,
		Override: override,
	})
	if err != nil {
		var pve *domain.PolicyViolationError
		if errors.As(err, &pve) {
			_ = d.Fmt.Print(cmd.OutOrStdout(), &ExecResult{
				Decision:   "block",
				Reason:     pve.Result.Reason(),
				ExitCode:   pve.Result.Decision.ExitCode(),
				DurationMs: 0,
				Cached:     false,
				Time:       d.Now(),
			})
			return err
		}
		_ = d.Fmt.Print(cmd.ErrOrStderr(), &ExecResult{
			Decision: "error",
			Reason:   err.Error(),
			Time:     d.Now(),
		})
		return err
	}

	// Success path: surface the result.
	res := &ExecResult{
		Decision:   out.Decision.Decision.String(),
		Reason:     out.Decision.Reason(),
		Stdout:     out.Result.Stdout,
		Stderr:     out.Result.Stderr,
		ExitCode:   out.Result.ExitCode,
		DurationMs: out.Result.Duration.Milliseconds(),
		Cached:     out.CacheUsed,
		Time:       d.Now(),
	}
	if err := d.Fmt.Print(cmd.OutOrStdout(), res); err != nil {
		return err
	}
	if out.Result.ExitCode != 0 {
		return &execExitError{Code: out.Result.ExitCode}
	}
	return nil
}

// execExitError is returned by runExec so cobra + exitCode() can map a
// non-zero command exit to a proper status code.
type execExitError struct{ Code int }

func (e *execExitError) Error() string { return "command exited non-zero" }
