package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/LeandroLCD/sudoconsole/internal/domain"
	"github.com/LeandroLCD/sudoconsole/internal/infrastructure/audit"
)

// AuditEventView is the JSON-friendly projection of a single audit
// record used by `sudoconsole audit tail`.
type AuditEventView struct {
	Time       string   `json:"time"`
	User       string   `json:"user"`
	Hostname   string   `json:"hostname"`
	SessionID  string   `json:"session_id"`
	Decision   string   `json:"decision"`
	PolicyHash string   `json:"policy_hash"`
	Command    string   `json:"command"`
	Categories []string `json:"categories,omitempty"`
	ExitCode   int      `json:"exit_code"`
	Redacted   bool     `json:"redacted"`
	OverrideBy string   `json:"override_by,omitempty"`
	Notes      string   `json:"notes,omitempty"`
}

// AuditTailResult is the JSON wrapper returned by `audit tail`.
type AuditTailResult struct {
	Path   string           `json:"path"`
	Count  int              `json:"count"`
	Events []AuditEventView `json:"events"`
	Time   string           `json:"time,omitempty"`
}

// newAuditCmd builds `sudoconsole audit`.
func newAuditCmd(app *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "audit",
		Short: "Inspect the audit log",
	}
	cmd.AddCommand(newAuditTailCmd(app))
	return cmd
}

func newAuditTailCmd(app *App) *cobra.Command {
	deps := &auditDeps{
		Fmt: app.Formatter,
		Now: app.Now,
	}
	cmd := &cobra.Command{
		Use:   "tail",
		Short: "Print the last N entries from the audit log",
		Long: `tail prints the last N records of the JSONL audit log written
by sudoconsole. The path is taken from the loaded configuration
(policy.audit.log_file). Defaults to 20 records.`,
		Example: "  sudoconsole audit tail\n  sudoconsole audit tail --count 5\n  sudoconsole --format json audit tail --count 50",
		RunE: func(cmd *cobra.Command, _ []string) error {
			n, _ := cmd.Flags().GetInt("count")
			return runAuditTail(cmd, deps, app, n)
		},
	}
	cmd.Flags().Int("count", 20, "number of recent records to display")
	return cmd
}

type auditDeps struct {
	Fmt Formatter
	Now func() string
}

func runAuditTail(cmd *cobra.Command, d *auditDeps, app *App, n int) error {
	path := app.Config.Policy.Audit.LogFile
	if path == "" {
		return fmt.Errorf("audit: policy.audit.log_file is not set")
	}
	events, err := audit.Tail(audit.TailOptions{Path: path, N: n})
	if err != nil {
		return fmt.Errorf("audit: %w", err)
	}
	views := make([]AuditEventView, 0, len(events))
	for _, e := range events {
		views = append(views, AuditEventView{
			Time:       e.Timestamp.UTC().Format("2006-01-02T15:04:05Z"),
			User:       e.User,
			Hostname:   e.Hostname,
			SessionID:  e.SessionID,
			Decision:   e.Decision.String(),
			PolicyHash: e.PolicyHash,
			Command:    e.Command.String(),
			Categories: categoriesToStrings(e.Result.Categories),
			ExitCode:   e.ExitCode,
			Redacted:   e.Redacted,
			OverrideBy: e.OverrideBy,
			Notes:      e.Notes,
		})
	}
	return d.Fmt.Print(cmd.OutOrStdout(), &AuditTailResult{
		Path:   path,
		Count:  len(views),
		Events: views,
		Time:   d.Now(),
	})
}

func categoriesToStrings(cats []domain.Category) []string {
	out := make([]string, 0, len(cats))
	for _, c := range cats {
		if s := string(c); s != "" {
			out = append(out, s)
		}
	}
	return out
}

// silence unused import warning when building without json formatter.
var _ = json.Marshal
