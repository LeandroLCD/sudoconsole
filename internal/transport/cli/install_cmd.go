package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/LeandroLCD/sudoconsole/internal/domain"
	"github.com/LeandroLCD/sudoconsole/internal/infrastructure/agent"
)

// InstallResult is the JSON output of `sudoconsole install`.
type InstallResult struct {
	Installed []InstallOutcome `json:"installed"`
	Skipped   []string         `json:"skipped,omitempty"`
	Failed    []InstallError   `json:"failed,omitempty"`
	Time      string           `json:"time,omitempty"`
}

// InstallOutcome is one successful install.
type InstallOutcome struct {
	Kind   string `json:"kind"`
	Name   string `json:"name"`
	Target string `json:"target,omitempty"`
	DryRun bool   `json:"dry_run,omitempty"`
}

// InstallError is one failed install.
type InstallError struct {
	Kind  string `json:"kind"`
	Error string `json:"error"`
}

// newInstallCmd builds `sudoconsole install` and
// `sudoconsole uninstall`.
func newInstallCmd(app *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install sudoconsole adapters for detected CLI agents",
		Long: `install registers sudoconsole with every detected CLI agent
(Kilo, Claude Code, Gemini, Aider, Codex, Copilot). The operation is
idempotent: running it twice yields the same state. Use --force to
overwrite an existing integration, --dry-run to preview the changes,
or --kind to restrict the operation to a specific agent.`,
		Example: "  sudoconsole install\n  sudoconsole install --kind kilo --kind claude\n  sudoconsole install --dry-run --force",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runInstall(cmd, app, false)
		},
	}
	cmd.Flags().StringSlice("kind", nil, "agent kinds to install (kilo, claude, gemini, aider, codex, copilot, generic)")
	cmd.Flags().Bool("force", false, "overwrite existing integration files")
	cmd.Flags().Bool("dry-run", false, "report what would change without touching the filesystem")
	cmd.AddCommand(newUninstallCmd(app))
	return cmd
}

func newUninstallCmd(app *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "uninstall",
		Short: "Remove sudoconsole adapters from every detected CLI agent",
		Long: `uninstall removes every integration that 'sudoconsole install'
created. It is idempotent: missing files are silently skipped.`,
		Example: "  sudoconsole uninstall\n  sudoconsole uninstall --kind kilo",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runInstall(cmd, app, true)
		},
	}
	cmd.Flags().StringSlice("kind", nil, "agent kinds to uninstall")
	cmd.Flags().Bool("dry-run", false, "report what would change without touching the filesystem")
	return cmd
}

func runInstall(cmd *cobra.Command, app *App, uninstall bool) error {
	kinds, _ := cmd.Flags().GetStringSlice("kind")
	force, _ := cmd.Flags().GetBool("force")
	dryRun, _ := cmd.Flags().GetBool("dry-run")

	// Resolve target kinds: explicit --kind wins; otherwise we scan
	// every installer from the registry (Generic is opt-in, so it
	// stays out of auto-install).
	var installers []domain.AgentInstaller
	if len(kinds) > 0 {
		for _, name := range kinds {
			kind, ok := parseAgentKind(name)
			if !ok {
				return fmt.Errorf("install: unknown kind %q", name)
			}
			inst, err := agent.NewForKind(kind, app.Config)
			if err != nil {
				return fmt.Errorf("install: %w", err)
			}
			installers = append(installers, inst)
		}
	} else {
		all, err := agent.All(app.Config)
		if err != nil {
			return fmt.Errorf("install: %w", err)
		}
		installers = all
	}

	opts := domain.InstallOptions{
		Force:  force,
		DryRun: dryRun,
		Config: app.Config,
	}
	res := &InstallResult{Time: app.Now()}

	// Sequential is fine — there are at most 6 adapters and each is
	// a few file writes; parallelism would just complicate error
	// reporting.
	for _, inst := range installers {
		kind := inst.Name().String()
		if uninstall {
			if err := inst.Uninstall(context.Background()); err != nil {
				res.Failed = append(res.Failed, InstallError{Kind: kind, Error: err.Error()})
				continue
			}
			res.Installed = append(res.Installed, InstallOutcome{Kind: kind, Name: inst.Name().DisplayName(), DryRun: dryRun})
		} else {
			if err := inst.Install(context.Background(), opts); err != nil {
				res.Failed = append(res.Failed, InstallError{Kind: kind, Error: err.Error()})
				continue
			}
			res.Installed = append(res.Installed, InstallOutcome{Kind: kind, Name: inst.Name().DisplayName(), DryRun: dryRun})
		}
	}
	if dryRun {
		// Avoid printing a misleading "installed" list when nothing
		// was actually written.
		res.Installed = nil
	}

	return app.Formatter.Print(cmd.OutOrStdout(), res)
}

// parseAgentKind maps a CLI string to a domain.AgentKind. Mirrors the
// parser inside the config package.
func parseAgentKind(s string) (domain.AgentKind, bool) {
	switch s {
	case "kilo":
		return domain.AgentKilo, true
	case "claude":
		return domain.AgentClaude, true
	case "gemini":
		return domain.AgentGemini, true
	case "aider":
		return domain.AgentAider, true
	case "codex":
		return domain.AgentCodex, true
	case "copilot":
		return domain.AgentCopilot, true
	case "generic":
		return domain.AgentGeneric, true
	}
	return domain.AgentUnknown, false
}

// silence unused import warnings.
var _ = time.Second
