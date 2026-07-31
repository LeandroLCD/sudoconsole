package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/LeandroLCD/sudoconsole/internal/domain"
)

// DetectResult is the JSON output of `sudoconsole detect`.
type DetectResult struct {
	Detected []AgentDetection `json:"detected"`
	Time     string           `json:"time,omitempty"`
}

// AgentDetection is one detected agent.
type AgentDetection struct {
	Kind       string `json:"kind"`
	Name       string `json:"name"`
	BinaryPath string `json:"binary_path,omitempty"`
	ConfigDir  string `json:"config_dir,omitempty"`
	Version    string `json:"version,omitempty"`
	Available  bool   `json:"available"`
}

// newDetectCmd builds `sudoconsole detect`.
func newDetectCmd(app *App) *cobra.Command {
	deps := &detectDeps{Fmt: app.Formatter, Now: app.Now}
	return &cobra.Command{
		Use:   "detect",
		Short: "Detect installed CLI agents on the host",
		Long: `detect scans the host for supported CLI agents (Kilo, Claude Code,
Gemini, Aider, Codex, Copilot). Each probe runs in parallel with a
2-second budget so a slow PATH lookup never blocks the call. Use
--json to feed the result into scripts.`,
		Example: "  sudoconsole detect\n  sudoconsole --format json detect",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runDetect(cmd, deps, app)
		},
	}
}

type detectDeps struct {
	Fmt Formatter
	Now func() string
}

func runDetect(cmd *cobra.Command, d *detectDeps, app *App) error {
	desc, err := app.AgentDetector.Detect(cmd.Context())
	if err != nil {
		return fmt.Errorf("detect: %w", err)
	}
	out := &DetectResult{Time: d.Now()}
	for _, a := range desc {
		out.Detected = append(out.Detected, AgentDetection{
			Kind:       a.Kind.String(),
			Name:       a.Kind.DisplayName(),
			BinaryPath: a.BinaryPath,
			ConfigDir:  a.ConfigDir,
			Version:    a.Version,
			Available:  a.Available,
		})
	}
	return d.Fmt.Print(cmd.OutOrStdout(), out)
}

// silence unused import warnings if time/domain become unused.
var (
	_ = time.Second
	_ domain.AgentKind
)
