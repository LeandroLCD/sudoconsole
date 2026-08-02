package cli

import (
	"github.com/spf13/cobra"

	"github.com/LeandroLCD/sudoconsole/internal/domain"
	"github.com/LeandroLCD/sudoconsole/internal/usecase"
)

func domainCacheActive() domain.CacheStatus { return domain.CacheActive }

// newCheckCmd builds `sudoconsole check`.
func newCheckCmd(app *App) *cobra.Command {
	deps := &checkDeps{
		UseCase: usecase.NewCheckUseCase(app.Repository),
		Fmt:     app.Formatter,
		Now:     app.Now,
	}
	return &cobra.Command{
		Use:   "check",
		Short: "Check whether the sudo cache is active",
		Long: `check inspects the sudo timestamp file and exits 0 if the
cache is currently valid, 1 otherwise. Useful for scripts that want to
gate a sudo call on a previous 'sudoconsole auth'.`,
		Example: "  # exit 0 if cache active, 1 otherwise\n  sudoconsole check && echo \"cached\" || echo \"needs auth\"\n\n  # machine-readable output\n  sudoconsole --format json check",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runCheck(cmd, deps, app)
		},
	}
}

type checkDeps struct {
	UseCase *usecase.CheckUseCase
	Fmt     Formatter
	Now     func() string
}

func runCheck(cmd *cobra.Command, d *checkDeps, app *App) error {
	out, err := d.UseCase.Execute(cmd.Context(), usecase.CheckInput{Config: app.Config})
	if err != nil {
		return d.Fmt.Print(cmd.ErrOrStderr(), &CheckResult{Active: false, Time: d.Now()})
	}
	_ = d.Fmt.Print(cmd.OutOrStdout(), &CheckResult{
		Active:           out.Status == domainCacheActive(),
		RemainingSeconds: out.Remaining.Seconds(),
		Time:             d.Now(),
	})
	return nil
}
