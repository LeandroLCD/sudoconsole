package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/LeandroLCD/sudoconsole/internal/domain"
	"github.com/LeandroLCD/sudoconsole/internal/infrastructure/agent"
	"github.com/LeandroLCD/sudoconsole/internal/usecase"
)

// InstallResult is the JSON output of `sudoconsole install`.
type InstallResult struct {
	Installed []InstallOutcome `json:"installed"`
	Skipped   []string         `json:"skipped,omitempty"`
	Failed    []InstallError   `json:"failed,omitempty"`
	Time      string           `json:"time,omitempty"`
	Planned   bool             `json:"planned,omitempty"`
}

// InstallOutcome is one successful (or planned) install.
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
--kind to restrict the operation to a specific agent, and --yes to skip
the interactive confirmation.`,
		Example: "  sudoconsole install\n  sudoconsole install --kind kilo --kind claude\n  sudoconsole install --dry-run --force",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runInstall(cmd, app, false)
		},
	}
	cmd.Flags().StringSlice("kind", nil, "agent kinds to install (kilo, claude, gemini, aider, codex, copilot, generic)")
	cmd.Flags().Bool("force", false, "overwrite existing integration files")
	cmd.Flags().Bool("dry-run", false, "report what would change without touching the filesystem")
	cmd.Flags().Bool("yes", false, "skip the interactive confirmation prompt")
	cmd.Flags().String("bin-dir", "", "override the bin directory for wrappers (default: config agent.bin_dir)")
	cmd.Flags().String("policy-mode", "",
		"override the policy mode for this invocation (blocklist|allowlist|audit)")
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
	cmd.Flags().Bool("yes", false, "skip the interactive confirmation prompt")
	return cmd
}

func runInstall(cmd *cobra.Command, app *App, uninstall bool) error {
	flags, err := readInstallFlags(cmd, uninstall)
	if err != nil {
		return err
	}

	parsedKinds, err := parseInstallKinds(flags.kinds)
	if err != nil {
		return err
	}

	if err := requireDetectorForAuto(app.AgentDetector, parsedKinds, uninstall); err != nil {
		return err
	}

	resolve := func(kind domain.AgentKind) (domain.AgentInstaller, error) {
		return agent.NewForKind(kind, app.Config)
	}
	confirm := buildConfirm(cmd.ErrOrStderr(), cmd.InOrStdin(), flags.yes)

	deps := &installDeps{
		UseCase: usecase.NewInstallAdapterUseCase(app.AgentDetector, resolve, app.Audit),
		Fmt:     app.Formatter,
		Now:     app.Now,
	}
	out, err := deps.UseCase.Execute(cmd.Context(), usecase.InstallAdapterInput{
		Config:             app.Config,
		Kinds:              parsedKinds,
		Force:              flags.force,
		DryRun:             flags.dryRun,
		BinDir:             flags.binDir,
		PolicyModeOverride: flags.policyMode,
		Uninstall:          uninstall,
		Yes:                flags.yes,
		Confirm:            confirm,
	})
	if err != nil && !errors.Is(err, usecase.ErrInstallAborted) {
		return fmt.Errorf("install: %w", err)
	}

	if perr := deps.Fmt.Print(cmd.OutOrStdout(), buildInstallResult(out, flags.dryRun)); perr != nil {
		return perr
	}
	if errors.Is(err, usecase.ErrInstallAborted) {
		return err
	}
	return nil
}

// installFlags captures the cobra flag values needed by runInstall so
// the function stays under the gocyclo threshold.
type installFlags struct {
	kinds      []string
	force      bool
	dryRun     bool
	yes        bool
	binDir     string
	policyMode domain.PolicyMode
}

// readInstallFlags pulls every documented flag off cmd in one place.
func readInstallFlags(cmd *cobra.Command, _ bool) (installFlags, error) {
	kinds, _ := cmd.Flags().GetStringSlice("kind")
	force, _ := cmd.Flags().GetBool("force")
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	yes, _ := cmd.Flags().GetBool("yes")
	binDir, _ := cmd.Flags().GetString("bin-dir")
	policyModeStr, _ := cmd.Flags().GetString("policy-mode")
	pm, err := parsePolicyModeOverride(policyModeStr)
	if err != nil {
		return installFlags{}, err
	}
	return installFlags{
		kinds:      kinds,
		force:      force,
		dryRun:     dryRun,
		yes:        yes,
		binDir:     binDir,
		policyMode: pm,
	}, nil
}

// parseInstallKinds maps every CLI --kind entry to a domain.AgentKind.
// Returns an error on the first unknown entry.
func parseInstallKinds(raw []string) ([]domain.AgentKind, error) {
	out := make([]domain.AgentKind, 0, len(raw))
	for _, name := range raw {
		kind, ok := parseAgentKind(name)
		if !ok {
			return nil, fmt.Errorf("install: unknown kind %q", name)
		}
		out = append(out, kind)
	}
	return out, nil
}

// requireDetectorForAuto returns a descriptive error when --kind is
// empty and no AgentDetector is configured (the use case would fail
// with a less helpful message otherwise).
func requireDetectorForAuto(d domain.AgentDetector, kinds []domain.AgentKind, _ bool) error {
	if len(kinds) > 0 || d != nil {
		return nil
	}
	return fmt.Errorf("install: no agent detector configured; pass --kind explicitly")
}

// buildInstallResult projects the use case output into the CLI's JSON
// shape (string kinds, DryRun flag, Planned marker).
func buildInstallResult(out usecase.InstallAdapterOutput, dryRun bool) *InstallResult {
	res := &InstallResult{
		Time:    out.Time,
		Planned: out.Plan || dryRun,
	}
	for _, e := range out.Installed {
		res.Installed = append(res.Installed, InstallOutcome{
			Kind:   e.Kind.String(),
			Name:   e.Name,
			DryRun: e.DryRun || dryRun,
		})
	}
	res.Skipped = out.Skipped
	for _, f := range out.Failed {
		res.Failed = append(res.Failed, InstallError{Kind: f.Kind.String(), Error: f.Error})
	}
	return res
}

// installDeps is the dependency bag for the install/uninstall command.
// Tests can construct it directly without touching the global App.
type installDeps struct {
	UseCase *usecase.InstallAdapterUseCase
	Fmt     Formatter
	Now     func() string
}

// parsePolicyModeOverride maps a CLI flag value to domain.PolicyMode.
// Empty strings mean "no override".
func parsePolicyModeOverride(s string) (domain.PolicyMode, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "":
		return domain.PolicyModeUnknown, nil
	case "blocklist":
		return domain.PolicyModeBlocklist, nil
	case "allowlist":
		return domain.PolicyModeAllowlist, nil
	case "audit":
		return domain.PolicyModeAudit, nil
	default:
		return domain.PolicyModeUnknown, fmt.Errorf(
			"install: invalid --policy-mode %q (want blocklist|allowlist|audit)", s)
	}
}

// buildConfirm returns a Confirm callback that prints msg to w and
// reads a y/N answer from r. When yes is true it returns (true, nil)
// immediately without touching r. If reading from r fails (e.g. EOF
// on a non-TTY pipe) it returns (false, err) so the caller can decide.
//
// When w is nil the prompt is silently suppressed.
func buildConfirm(w io.Writer, r io.Reader, yes bool) func(string) (bool, error) {
	return func(msg string) (bool, error) {
		if yes {
			return true, nil
		}
		if w != nil {
			_, _ = io.WriteString(w, msg)
		}
		scanner := bufio.NewScanner(r)
		if !scanner.Scan() {
			if err := scanner.Err(); err != nil {
				return false, err
			}
			// EOF: no answer.
			return false, nil
		}
		ans := strings.TrimSpace(scanner.Text())
		return strings.EqualFold(ans, "y") || strings.EqualFold(ans, "yes"), nil
	}
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
