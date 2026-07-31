package agent

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/LeandroLCD/sudoconsole/internal/domain"
)

// KiloAdapter installs sudoconsole as a Kilo CLI slash command.
//
// Kilo picks up command files from
// `~/.config/kilo/commands/<name>.md`. We drop one called
// `sudoconsole.md` so the user can invoke `/sudoconsole` from the
// agent chat.
type KiloAdapter struct {
	base baseAdapter
}

// NewKiloAdapter returns a KiloAdapter backed by the real OS fs.
func NewKiloAdapter(cfg domain.Config) (*KiloAdapter, error) {
	base, err := newBase(domain.AgentKilo, cfg)
	if err != nil {
		return nil, err
	}
	return &KiloAdapter{base: base}, nil
}

// Name implements domain.AgentInstaller.
func (k *KiloAdapter) Name() domain.AgentKind { return domain.AgentKilo }

// Detect reports whether the Kilo config dir exists.
func (k *KiloAdapter) Detect(_ context.Context) (bool, error) {
	dir := k.base.expandHome(".config/kilo")
	return k.base.fileExists(dir), nil
}

// Install drops the slash command file in ~/.config/kilo/commands/.
func (k *KiloAdapter) Install(ctx context.Context, opts domain.InstallOptions) error {
	if opts.DryRun {
		return nil
	}
	target := filepath.Join(k.base.expandHome(".config/kilo"), "commands", "sudoconsole.md")
	if err := k.base.guardedInstall(target, commandsMarkdown(domain.AgentKilo), opts.Force); err != nil {
		return fmt.Errorf("kilo install: %w", err)
	}
	return nil
}

// Uninstall removes the slash command file.
func (k *KiloAdapter) Uninstall(_ context.Context) error {
	target := filepath.Join(k.base.expandHome(".config/kilo"), "commands", "sudoconsole.md")
	if err := k.base.removeIfPresent(target); err != nil {
		return fmt.Errorf("kilo uninstall: %w", err)
	}
	return nil
}

// AdapterCommand returns the command line for invoking sudoconsole.
func (k *KiloAdapter) AdapterCommand() string { return adapterCmd() }

// Compile-time check.
var _ domain.AgentInstaller = (*KiloAdapter)(nil)

// silence unused warnings when ctx is not consumed.
var _ = context.Background

// silence unused warnings when errors is not consumed.
var _ = errors.New
