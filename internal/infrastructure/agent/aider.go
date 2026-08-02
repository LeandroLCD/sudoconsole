package agent

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"github.com/LeandroLCD/sudoconsole/internal/domain"
)

const aiderAliasName = "sudoconsole"

// AiderAdapter wires sudoconsole into Aider via an alias entry in
// `~/.aider.conf.yml`. Aider re-reads the YAML on every invocation so
// we append (and on uninstall remove) a single top-level `alias` key
// without touching any pre-existing keys.
type AiderAdapter struct {
	base baseAdapter
}

// NewAiderAdapter returns an AiderAdapter backed by the real OS fs.
func NewAiderAdapter(cfg domain.Config) (*AiderAdapter, error) {
	base, err := newBase(domain.AgentAider, cfg)
	if err != nil {
		return nil, err
	}
	return &AiderAdapter{base: base}, nil
}

// Name implements domain.AgentInstaller.
func (a *AiderAdapter) Name() domain.AgentKind { return domain.AgentAider }

// Detect reports whether the Aider config dir or file exists.
func (a *AiderAdapter) Detect(_ context.Context) (bool, error) {
	dir := a.base.expandHome(".aider")
	file := a.base.expandHome(".aider.conf.yml")
	return a.base.fileExists(dir) || a.base.fileExists(file), nil
}

// Install appends the alias block to ~/.aider.conf.yml.
func (a *AiderAdapter) Install(ctx context.Context, opts domain.InstallOptions) error {
	if opts.DryRun {
		return nil
	}
	target := a.base.expandHome(".aider.conf.yml")
	existing := a.base.readFileOrEmpty(target)
	if containsMarker(existing, marker(domain.AgentAider)) {
		if !opts.Force {
			return nil
		}
		// Strip the previous block before rewriting.
		existing = removeAliasBlock(existing, domain.AgentAider)
	}
	block := aiderAliasBlock()
	out := existing
	if out != "" && !strings.HasSuffix(out, "\n") {
		out += "\n"
	}
	out += block
	if err := a.base.writeAtomic(target, []byte(out), 0o600); err != nil {
		return fmt.Errorf("aider install: %w", err)
	}
	return nil
}

// Uninstall removes the alias block.
func (a *AiderAdapter) Uninstall(_ context.Context) error {
	target := a.base.expandHome(".aider.conf.yml")
	existing := a.base.readFileOrEmpty(target)
	if existing == "" {
		return nil
	}
	cleaned := removeAliasBlock(existing, domain.AgentAider)
	if cleaned == existing {
		return nil
	}
	if cleaned == "" {
		return a.base.removeIfPresent(target)
	}
	if err := a.base.writeAtomic(target, []byte(cleaned), 0o600); err != nil {
		return fmt.Errorf("aider uninstall: %w", err)
	}
	return nil
}

// AdapterCommand returns the command line for invoking sudoconsole.
func (a *AiderAdapter) AdapterCommand() string { return adapterCmd() }

// aiderAliasBlock returns the YAML fragment that registers the alias.
// We use a literal block style for readability.
func aiderAliasBlock() string {
	return fmt.Sprintf("\n%s\nalias:\n  %s: %s\n",
		marker(domain.AgentAider),
		aiderAliasName,
		adapterCmd(),
	)
}

// removeAliasBlock removes the section starting at the marker for the
// supplied kind and the next "alias:" block. It is intentionally
// conservative: it operates on text only, never on YAML ASTs, so a
// malformed file is preserved.
func removeAliasBlock(text string, kind domain.AgentKind) string {
	lines := bytes.Split([]byte(text), []byte{'\n'})
	out := make([][]byte, 0, len(lines))
	skipUntilDedent := false
	for _, line := range lines {
		s := string(line)
		trim := strings.TrimLeft(s, " \t")
		if strings.HasPrefix(trim, marker(kind)) {
			skipUntilDedent = true
			continue
		}
		if skipUntilDedent {
			if strings.HasPrefix(trim, "alias:") {
				continue
			}
			if strings.HasPrefix(s, "  ") || strings.HasPrefix(s, "\t") || trim == "" {
				continue
			}
			skipUntilDedent = false
		}
		out = append(out, []byte(s))
	}
	cleaned := strings.TrimRight(string(bytes.Join(out, []byte{'\n'})), "\n") + "\n"
	if cleaned == "\n" {
		return ""
	}
	return cleaned
}

// Compile-time check.
var _ domain.AgentInstaller = (*AiderAdapter)(nil)
