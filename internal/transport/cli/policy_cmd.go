package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/LeandroLCD/sudoconsole/internal/domain"
	"github.com/LeandroLCD/sudoconsole/internal/infrastructure/policy"
	"github.com/LeandroLCD/sudoconsole/internal/usecase"
)

// PolicyListResult is the JSON shape of `sudoconsole policy list`.
type PolicyListResult struct {
	Categories []domain.CategoryEntry `json:"categories"`
	Time       string                 `json:"time,omitempty"`
}

// PolicyTestResult is the JSON shape of `sudoconsole policy test "<cmd>"`.
type PolicyTestResult struct {
	Decision   string            `json:"decision"`
	Reasons    []string          `json:"reasons,omitempty"`
	Categories []domain.Category `json:"categories,omitempty"`
	Risk       string            `json:"risk,omitempty"`
	Command    string            `json:"command"`
	Overrode   bool              `json:"overrode,omitempty"`
	Time       string            `json:"time,omitempty"`
}

// PolicyShowResult is the JSON shape of `sudoconsole policy show`.
type PolicyShowResult struct {
	Mode               string                          `json:"mode"`
	BlockedCategories  []domain.Category               `json:"blocked_categories,omitempty"`
	AllowedCategories  []domain.Category               `json:"allowed_categories,omitempty"`
	BlockedCommands    []string                        `json:"blocked_commands,omitempty"`
	AllowedCommands    []string                        `json:"allowed_commands,omitempty"`
	BlockedPatterns    []string                        `json:"blocked_patterns,omitempty"`
	AllowedPatterns    []string                        `json:"allowed_patterns,omitempty"`
	ExtraPatterns      []string                        `json:"extra_patterns,omitempty"`
	RemoteAccess       domain.RemoteAccessConfig       `json:"remote_access"`
	CredentialExposure domain.CredentialExposureConfig `json:"credential_exposure"`
	Audit              domain.AuditConfig              `json:"audit"`
	Time               string                          `json:"time,omitempty"`
}

// PolicyValidateResult is the JSON shape of `sudoconsole policy validate`.
type PolicyValidateResult struct {
	Valid   bool                  `json:"valid"`
	Blocked []domain.PatternError `json:"blocked,omitempty"`
	Allowed []domain.PatternError `json:"allowed,omitempty"`
	Extras  []domain.PatternError `json:"extras,omitempty"`
	Time    string                `json:"time,omitempty"`
}

// newPolicyCmd builds `sudoconsole policy` and every subcommand.
func newPolicyCmd(app *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "policy",
		Short: "Inspect and validate the active policy",
		Long: `policy lets you preview and verify the rules sudoconsole applies
before any sudo command runs. Use it to understand which binaries are
blocked, to dry-run a single command against the active configuration,
and to lint custom patterns before they take effect.`,
	}
	cmd.AddCommand(newPolicyListCmd(app))
	cmd.AddCommand(newPolicyTestCmd(app))
	cmd.AddCommand(newPolicyShowCmd(app))
	cmd.AddCommand(newPolicyValidateCmd(app))
	return cmd
}

func newPolicyListCmd(app *App) *cobra.Command {
	deps := &policyDeps{Fmt: app.Formatter, Now: app.Now}
	return &cobra.Command{
		Use:   "list",
		Short: "List every binary classified by the policy engine",
		Long: `list dumps the (binary → category → risk) table used by the
evaluator. Built-in entries ship with sudoconsole; entries added via
[policy.custom] are flagged with builtin=false.`,
		Example: "  sudoconsole policy list\n  sudoconsole --format json policy list",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runPolicyList(cmd, deps, app)
		},
	}
}

func newPolicyTestCmd(app *App) *cobra.Command {
	deps := &policyDeps{Fmt: app.Formatter, Now: app.Now}
	return &cobra.Command{
		Use:   `test "<cmd>"`,
		Short: "Dry-run a command against the active policy",
		Long: `test evaluates a command line against the effective policy and
prints the resulting decision (allow / warn / block / audit) together
with the matched categories, risk level and reasons. Nothing is
executed.`,
		Example: `  sudoconsole policy test "ssh user@host"
  sudoconsole --format json policy test "apt update"`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPolicyTest(cmd, deps, app, args)
		},
	}
}

func newPolicyShowCmd(app *App) *cobra.Command {
	deps := &policyDeps{Fmt: app.Formatter, Now: app.Now}
	return &cobra.Command{
		Use:   "show",
		Short: "Dump the effective policy configuration",
		Long: `show prints the resolved policy (defaults merged with the user
config file and any --policy-mode override from this invocation).
Useful for debugging which rules are actually active.`,
		Example: "  sudoconsole policy show\n  sudoconsole --format json policy show",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runPolicyShow(cmd, deps, app)
		},
	}
}

func newPolicyValidateCmd(app *App) *cobra.Command {
	deps := &policyDeps{
		Fmt:       app.Formatter,
		Now:       app.Now,
		Validator: usecase.NewValidatePolicyUseCase(policy.NewPatternValidator()),
	}
	return &cobra.Command{
		Use:   "validate",
		Short: "Lint every custom pattern for syntax errors and ReDoS vectors",
		Long: `validate compiles each user-defined pattern (glob or re:regex) and
reports syntax errors plus ReDoS heuristics. Exits non-zero when at
least one pattern is rejected.`,
		Example: "  sudoconsole policy validate\n  sudoconsole --format json policy validate",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runPolicyValidate(cmd, deps, app)
		},
	}
}

// policyDeps bundles the dependencies shared by every policy subcommand.
type policyDeps struct {
	Fmt         Formatter
	Now         func() string
	EvaluatorUC *usecase.EvaluatePolicyUseCase
	Validator   *usecase.ValidatePolicyUseCase
}

func runPolicyList(cmd *cobra.Command, d *policyDeps, app *App) error {
	cats, err := app.Evaluator.ListCategories(cmd.Context())
	if err != nil {
		return fmt.Errorf("policy list: %w", err)
	}
	out := &PolicyListResult{Categories: cats, Time: d.Now()}
	return d.Fmt.Print(cmd.OutOrStdout(), out)
}

func runPolicyTest(cmd *cobra.Command, d *policyDeps, app *App, args []string) error {
	if app.Evaluator == nil {
		return fmt.Errorf("policy test: evaluator not configured")
	}
	raw := strings.Join(args, " ")
	command, err := domain.NewCommandFromRaw(raw)
	if err != nil {
		return fmt.Errorf("policy test: %w", err)
	}
	override, _ := cmd.Flags().GetBool("override")
	uc := usecase.NewEvaluatePolicyUseCase(app.Evaluator)

	out, err := uc.Execute(cmd.Context(), usecase.EvaluatePolicyInput{
		Policy:   app.Config.Policy,
		Command:  command,
		Override: override,
	})
	if err != nil {
		var pve *domain.PolicyViolationError
		if errors.As(err, &pve) {
			// For test we still want to surface the decision; render
			// the structured result and signal the block via exit code.
			pcats := make([]domain.Category, len(pve.Result.Categories))
			copy(pcats, pve.Result.Categories)
			res := &PolicyTestResult{
				Decision:   pve.Result.Decision.String(),
				Reasons:    pve.Result.Reasons,
				Categories: pcats,
				Risk:       pve.Result.Risk.String(),
				Command:    command.String(),
				Overrode:   out.Override,
				Time:       d.Now(),
			}
			if perr := d.Fmt.Print(cmd.OutOrStdout(), res); perr != nil {
				return perr
			}
			return err
		}
		return fmt.Errorf("policy test: %w", err)
	}
	cats := make([]domain.Category, len(out.Result.Categories))
	copy(cats, out.Result.Categories)
	res := &PolicyTestResult{
		Decision:   out.Result.Decision.String(),
		Reasons:    out.Result.Reasons,
		Categories: cats,
		Risk:       out.Result.Risk.String(),
		Command:    command.String(),
		Overrode:   out.Override,
		Time:       d.Now(),
	}
	return d.Fmt.Print(cmd.OutOrStdout(), res)
}

func runPolicyShow(cmd *cobra.Command, d *policyDeps, app *App) error {
	p := app.Config.Policy
	res := &PolicyShowResult{
		Mode:               p.Mode.String(),
		BlockedCategories:  p.Blocked.Categories,
		AllowedCategories:  p.Allowed.Categories,
		BlockedCommands:    p.Blocked.Commands,
		AllowedCommands:    p.Allowed.Commands,
		BlockedPatterns:    p.Blocked.Patterns,
		AllowedPatterns:    p.Allowed.Patterns,
		ExtraPatterns:      p.ExtraPatterns,
		RemoteAccess:       p.RemoteAccess,
		CredentialExposure: p.CredentialExposure,
		Audit:              p.Audit,
		Time:               d.Now(),
	}
	return d.Fmt.Print(cmd.OutOrStdout(), res)
}

func runPolicyValidate(cmd *cobra.Command, d *policyDeps, app *App) error {
	out := d.Validator.Execute(cmd.Context(), usecase.ValidatePolicyInput{
		Policy: app.Config.Policy,
	})
	res := &PolicyValidateResult{
		Valid:   out.Valid,
		Blocked: out.Blocked,
		Allowed: out.Allowed,
		Extras:  out.Extras,
		Time:    d.Now(),
	}
	if perr := d.Fmt.Print(cmd.OutOrStdout(), res); perr != nil {
		return perr
	}
	if !out.Valid {
		return fmt.Errorf("policy validate: %d pattern(s) rejected", len(out.AllErrors()))
	}
	return nil
}

// silence unused import warning when context is not otherwise consumed.
var _ = context.Background
