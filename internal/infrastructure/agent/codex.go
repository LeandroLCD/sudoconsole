package agent

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/LeandroLCD/sudoconsole/internal/domain"
)

// CodexAdapter installs sudoconsole as a Codex tool entry.
//
// Codex reads tool definitions from `~/.codex/<name>.toml`. We drop a
// TOML file describing the sudoconsole wrapper.
type CodexAdapter struct {
	base baseAdapter
}

// NewCodexAdapter returns a CodexAdapter backed by the real OS fs.
func NewCodexAdapter(cfg domain.Config) (*CodexAdapter, error) {
	base, err := newBase(domain.AgentCodex, cfg)
	if err != nil {
		return nil, err
	}
	return &CodexAdapter{base: base}, nil
}

// Name implements domain.AgentInstaller.
func (c *CodexAdapter) Name() domain.AgentKind { return domain.AgentCodex }

// Detect reports whether the Codex config dir exists.
func (c *CodexAdapter) Detect(_ context.Context) (bool, error) {
	dir := c.base.expandHome(".codex")
	return c.base.fileExists(dir), nil
}

func codexToolToml() []byte {
	return []byte(`# ` + marker(domain.AgentCodex) + `
# managed by sudoconsole. do not edit by hand.

[tool]
name = "sudoconsole"
description = "Run a shell command under sudo with policy enforcement."
command = "sudoconsole"
`)
}

// Install drops the Codex tool file.
func (c *CodexAdapter) Install(ctx context.Context, opts domain.InstallOptions) error {
	if opts.DryRun {
		return nil
	}
	target := filepath.Join(c.base.expandHome(".codex"), "sudosafe.toml")
	if err := c.base.guardedInstall(target, codexToolToml(), opts.Force); err != nil {
		return fmt.Errorf("codex install: %w", err)
	}
	return nil
}

// Uninstall removes the Codex tool file.
func (c *CodexAdapter) Uninstall(_ context.Context) error {
	target := filepath.Join(c.base.expandHome(".codex"), "sudosafe.toml")
	if err := c.base.removeIfPresent(target); err != nil {
		return fmt.Errorf("codex uninstall: %w", err)
	}
	return nil
}

// AdapterCommand returns the command line for invoking sudoconsole.
func (c *CodexAdapter) AdapterCommand() string { return adapterCmd() }

// Compile-time check.
var _ domain.AgentInstaller = (*CodexAdapter)(nil)
