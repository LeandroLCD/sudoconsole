package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/LeandroLCD/sudoconsole/internal/domain"
)

// ClaudeAdapter installs sudoconsole as a Claude Code slash command.
//
// Claude Code picks up command files from `~/.claude/commands/` and
// reads `~/.claude/settings.json` for project-level configuration. We
// drop a slash command file and add (or refresh) a single key in the
// settings JSON without touching any pre-existing keys.
type ClaudeAdapter struct {
	base baseAdapter
}

// NewClaudeAdapter returns a ClaudeAdapter backed by the real OS fs.
func NewClaudeAdapter(cfg domain.Config) (*ClaudeAdapter, error) {
	base, err := newBase(domain.AgentClaude, cfg)
	if err != nil {
		return nil, err
	}
	return &ClaudeAdapter{base: base}, nil
}

// Name implements domain.AgentInstaller.
func (c *ClaudeAdapter) Name() domain.AgentKind { return domain.AgentClaude }

// Detect reports whether the Claude Code config dir exists.
func (c *ClaudeAdapter) Detect(_ context.Context) (bool, error) {
	dir := c.base.expandHome(".claude")
	return c.base.fileExists(dir), nil
}

// Install writes the slash command file and patches settings.json.
func (c *ClaudeAdapter) Install(ctx context.Context, opts domain.InstallOptions) error {
	if opts.DryRun {
		return nil
	}
	cmdTarget := filepath.Join(c.base.expandHome(".claude"), "commands", "sudoconsole.md")
	if err := c.base.guardedInstall(cmdTarget, commandsMarkdown(domain.AgentClaude), opts.Force); err != nil {
		return fmt.Errorf("claude install: %w", err)
	}
	if err := c.patchSettings(opts.Force); err != nil {
		return fmt.Errorf("claude settings: %w", err)
	}
	return nil
}

// Uninstall removes the slash command file and the settings.json key.
func (c *ClaudeAdapter) Uninstall(_ context.Context) error {
	cmdTarget := filepath.Join(c.base.expandHome(".claude"), "commands", "sudoconsole.md")
	if err := c.base.removeIfPresent(cmdTarget); err != nil {
		return fmt.Errorf("claude uninstall: %w", err)
	}
	if err := c.unpatchSettings(); err != nil {
		return fmt.Errorf("claude settings uninstall: %w", err)
	}
	return nil
}

// AdapterCommand returns the command line for invoking sudoconsole.
func (c *ClaudeAdapter) AdapterCommand() string { return adapterCmd() }

// patchSettings adds (or refreshes) the `sudoconsole` key in
// settings.json, preserving every other key.
func (c *ClaudeAdapter) patchSettings(force bool) error {
	path := filepath.Join(c.base.expandHome(".claude"), "settings.json")
	settings := map[string]json.RawMessage{}
	if raw := c.base.readFileOrEmpty(path); raw != "" {
		if err := json.Unmarshal([]byte(raw), &settings); err != nil {
			return fmt.Errorf("settings.json: %w", err)
		}
		if _, exists := settings["sudoconsole"]; exists && !force {
			return nil
		}
	}
	payload := map[string]any{
		"command":   "sudoconsole",
		"managed":   "sudoconsole",
		"installed": "true",
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	settings["sudoconsole"] = b
	out, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	return c.base.writeAtomic(path, out, 0o600)
}

// unpatchSettings removes the `sudoconsole` key from settings.json.
func (c *ClaudeAdapter) unpatchSettings() error {
	path := filepath.Join(c.base.expandHome(".claude"), "settings.json")
	raw := c.base.readFileOrEmpty(path)
	if raw == "" {
		return nil
	}
	settings := map[string]json.RawMessage{}
	if err := json.Unmarshal([]byte(raw), &settings); err != nil {
		return nil // tolerate corruption; nothing to clean up
	}
	if _, ok := settings["sudoconsole"]; !ok {
		return nil
	}
	delete(settings, "sudoconsole")
	if len(settings) == 0 {
		return c.base.removeIfPresent(path)
	}
	out, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	return c.base.writeAtomic(path, out, 0o600)
}

// Compile-time check.
var _ domain.AgentInstaller = (*ClaudeAdapter)(nil)

// silence unused warnings.
var _ = strings.TrimSpace
