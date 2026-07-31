package agent

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/LeandroLCD/sudoconsole/internal/domain"
)

// GeminiAdapter installs sudoconsole as a Gemini tool entry.
//
// Gemini reads tool definitions from `~/.gemini/tools/<name>.toml`.
// We drop a TOML file describing a thin shell wrapper around
// sudoconsole.
type GeminiAdapter struct {
	base baseAdapter
}

// NewGeminiAdapter returns a GeminiAdapter backed by the real OS fs.
func NewGeminiAdapter(cfg domain.Config) (*GeminiAdapter, error) {
	base, err := newBase(domain.AgentGemini, cfg)
	if err != nil {
		return nil, err
	}
	return &GeminiAdapter{base: base}, nil
}

// Name implements domain.AgentInstaller.
func (g *GeminiAdapter) Name() domain.AgentKind { return domain.AgentGemini }

// Detect reports whether the Gemini config dir exists.
func (g *GeminiAdapter) Detect(_ context.Context) (bool, error) {
	dir := g.base.expandHome(".gemini")
	return g.base.fileExists(dir), nil
}

func geminiToolToml() []byte {
	return []byte(`# ` + marker(domain.AgentGemini) + `
# managed by sudoconsole. do not edit by hand.

[tool]
name = "sudoconsole"
description = "Run a shell command under sudo with policy enforcement."
command = "sudoconsole"
`)
}

// Install drops the Gemini tool definition file.
func (g *GeminiAdapter) Install(ctx context.Context, opts domain.InstallOptions) error {
	if opts.DryRun {
		return nil
	}
	target := filepath.Join(g.base.expandHome(".gemini"), "tools", "sudoconsole.toml")
	if err := g.base.guardedInstall(target, geminiToolToml(), opts.Force); err != nil {
		return fmt.Errorf("gemini install: %w", err)
	}
	return nil
}

// Uninstall removes the Gemini tool file.
func (g *GeminiAdapter) Uninstall(_ context.Context) error {
	target := filepath.Join(g.base.expandHome(".gemini"), "tools", "sudoconsole.toml")
	if err := g.base.removeIfPresent(target); err != nil {
		return fmt.Errorf("gemini uninstall: %w", err)
	}
	return nil
}

// AdapterCommand returns the command line for invoking sudoconsole.
func (g *GeminiAdapter) AdapterCommand() string { return adapterCmd() }

// Compile-time check.
var _ domain.AgentInstaller = (*GeminiAdapter)(nil)
