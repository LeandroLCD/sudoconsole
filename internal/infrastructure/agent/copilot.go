package agent

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/LeandroLCD/sudoconsole/internal/domain"
)

// CopilotAdapter wires sudoconsole into GitHub Copilot via the `gh`
// CLI's alias mechanism: `gh alias set sudoconsole 'sudoconsole'`.
//
// The adapter does not touch any config file directly; instead it
// delegates to the host's `gh` binary so the alias lands in the same
// place gh itself would write it (typically
// `~/.config/gh/aliases.yml`).
type CopilotAdapter struct {
	base   baseAdapter
	ghLook func(string) (string, error)
	runCmd func(ctx context.Context, name string, args ...string) (string, error)
}

// NewCopilotAdapter returns a CopilotAdapter that delegates to the
// real `gh` binary.
func NewCopilotAdapter(cfg domain.Config) (*CopilotAdapter, error) {
	base, err := newBase(domain.AgentCopilot, cfg)
	if err != nil {
		return nil, err
	}
	return &CopilotAdapter{
		base:   base,
		ghLook: exec.LookPath,
		runCmd: runGhAlias,
	}, nil
}

// Name implements domain.AgentInstaller.
func (c *CopilotAdapter) Name() domain.AgentKind { return domain.AgentCopilot }

// Detect reports whether `gh` is on PATH or the Copilot config dir
// exists.
func (c *CopilotAdapter) Detect(_ context.Context) (bool, error) {
	if _, err := c.ghLook("gh"); err == nil {
		return true, nil
	}
	dir := c.base.expandHome(".config/gh")
	return c.base.fileExists(dir), nil
}

// Install runs `gh alias set sudoconsole 'sudoconsole'`.
func (c *CopilotAdapter) Install(ctx context.Context, opts domain.InstallOptions) error {
	if opts.DryRun {
		return nil
	}
	if _, err := c.ghLook("gh"); err != nil {
		return fmt.Errorf("%w: gh CLI not found in PATH", domain.ErrAgentInstallFailed)
	}
	out, err := c.runCmd(ctx, "gh", "alias", "set", "sudoconsole", adapterCmd())
	if err != nil {
		return fmt.Errorf("copilot install: %w (%s)", err, strings.TrimSpace(out))
	}
	return nil
}

// Uninstall runs `gh alias delete sudoconsole`.
func (c *CopilotAdapter) Uninstall(ctx context.Context) error {
	if _, err := c.ghLook("gh"); err != nil {
		return nil
	}
	if _, err := c.runCmd(ctx, "gh", "alias", "delete", "sudoconsole"); err != nil {
		return fmt.Errorf("copilot uninstall: %w", err)
	}
	return nil
}

// AdapterCommand returns the command line for invoking sudoconsole.
func (c *CopilotAdapter) AdapterCommand() string { return adapterCmd() }

// runGhAlias is the default exec backend for CopilotAdapter. It is a
// variable so tests can replace it with a stub.
var runGhAlias = func(ctx context.Context, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...) // #nosec G204 -- name was resolved by exec.LookPath.
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// silence unused warnings.
var (
	_ = filepath.Join
	_ = errors.New
)

// Compile-time check.
var _ domain.AgentInstaller = (*CopilotAdapter)(nil)
