package agent

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/LeandroLCD/sudoconsole/internal/domain"
)

// GenericAdapter registers sudoconsole as a shell alias in the
// user's `~/.bashrc` and `~/.zshrc`. Use it as a fallback when none
// of the dedicated adapters apply.
type GenericAdapter struct {
	base baseAdapter
}

// NewGenericAdapter returns a GenericAdapter backed by the real OS fs.
func NewGenericAdapter(cfg domain.Config) (*GenericAdapter, error) {
	base, err := newBase(domain.AgentGeneric, cfg)
	if err != nil {
		return nil, err
	}
	return &GenericAdapter{base: base}, nil
}

// Name implements domain.AgentInstaller.
func (g *GenericAdapter) Name() domain.AgentKind { return domain.AgentGeneric }

// Detect always returns true so the user can install it manually via
// `sudoconsole install --include generic`.
func (g *GenericAdapter) Detect(_ context.Context) (bool, error) {
	return true, nil
}

const genericAliasName = "sudosafe"

func genericAliasBlock() string {
	return fmt.Sprintf("\n%s\nalias:\n  %s: %s\n",
		marker(domain.AgentGeneric),
		genericAliasName,
		adapterCmd(),
	)
}

// Install appends the alias block to every existing rc file.
func (g *GenericAdapter) Install(ctx context.Context, opts domain.InstallOptions) error {
	if opts.DryRun {
		return nil
	}
	for _, rc := range g.rcFiles() {
		if !g.base.fileExists(rc) {
			continue
		}
		existing := g.base.readFileOrEmpty(rc)
		if containsMarker(existing, marker(domain.AgentGeneric)) {
			if !opts.Force {
				continue
			}
			existing = removeAliasBlock(existing, domain.AgentGeneric)
		}
		out := existing
		if out != "" && !strings.HasSuffix(out, "\n") {
			out += "\n"
		}
		out += genericAliasBlock()
		if err := g.base.writeAtomic(rc, []byte(out), 0o600); err != nil {
			return fmt.Errorf("generic install %s: %w", rc, err)
		}
	}
	return nil
}

// Uninstall removes the alias block from every rc file.
func (g *GenericAdapter) Uninstall(_ context.Context) error {
	for _, rc := range g.rcFiles() {
		if !g.base.fileExists(rc) {
			continue
		}
		existing := g.base.readFileOrEmpty(rc)
		if existing == "" {
			continue
		}
		cleaned := removeAliasBlock(existing, domain.AgentGeneric)
		if cleaned == existing {
			continue
		}
		if cleaned == "" {
			if err := g.base.removeIfPresent(rc); err != nil {
				return fmt.Errorf("generic uninstall %s: %w", rc, err)
			}
			continue
		}
		if err := g.base.writeAtomic(rc, []byte(cleaned), 0o600); err != nil {
			return fmt.Errorf("generic uninstall %s: %w", rc, err)
		}
	}
	return nil
}

// AdapterCommand returns the command line for invoking sudoconsole.
func (g *GenericAdapter) AdapterCommand() string { return genericAliasName }

// rcFiles returns the paths to inspect on the current OS. macOS and
// Linux both keep bashrc/zshrc in $HOME; Windows is unsupported.
func (g *GenericAdapter) rcFiles() []string {
	if os.PathSeparator == '\\' {
		return nil
	}
	home := g.base.resolveHome()
	return []string{
		filepath.Join(home, ".bashrc"),
		filepath.Join(home, ".zshrc"),
	}
}

// Compile-time check.
var _ domain.AgentInstaller = (*GenericAdapter)(nil)
