package agent

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/LeandroLCD/sudoconsole/internal/domain"
)

func TestContract_Kilo(t *testing.T) {
	fsys := newMemFS()
	home := "/h"
	contract(t, fsys, home, domain.AgentKilo, domain.DefaultConfig(),
		filepath.Join(home, ".config/kilo", "commands", "sudoconsole.md"))
}

func TestContract_Claude(t *testing.T) {
	fsys := newMemFS()
	home := "/h"
	a := installInto(t, fsys, home, domain.AgentClaude, domain.DefaultConfig())
	if err := a.Install(context.Background(), domain.InstallOptions{}); err != nil {
		t.Fatal(err)
	}
	// Both command file and settings.json must exist.
	if !fsys.exists(filepath.Join(home, ".claude", "commands", "sudoconsole.md")) {
		t.Error("command file missing")
	}
	if !fsys.exists(filepath.Join(home, ".claude", "settings.json")) {
		t.Error("settings.json missing")
	}
	// settings.json should contain a sudoconsole key.
	if !contains(fsys.get(filepath.Join(home, ".claude", "settings.json")), `"sudoconsole"`) {
		t.Error("settings.json missing sudoconsole key")
	}
	if err := a.Uninstall(context.Background()); err != nil {
		t.Fatal(err)
	}
	if fsys.exists(filepath.Join(home, ".claude", "commands", "sudoconsole.md")) {
		t.Error("command file should be gone")
	}
	// Settings file should either be gone or no longer contain the
	// sudoconsole key.
	if fsys.exists(filepath.Join(home, ".claude", "settings.json")) {
		if contains(fsys.get(filepath.Join(home, ".claude", "settings.json")), `"sudoconsole"`) {
			t.Error("settings.json still contains sudoconsole key")
		}
	}
}

func TestContract_Gemini(t *testing.T) {
	fsys := newMemFS()
	home := "/h"
	contract(t, fsys, home, domain.AgentGemini, domain.DefaultConfig(),
		filepath.Join(home, ".gemini", "tools", "sudoconsole.toml"))
}

func TestContract_Aider(t *testing.T) {
	fsys := newMemFS()
	home := "/h"
	a := installInto(t, fsys, home, domain.AgentAider, domain.DefaultConfig())
	if err := a.Install(context.Background(), domain.InstallOptions{}); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(home, ".aider.conf.yml")
	if !fsys.exists(target) {
		t.Fatal("aider config missing")
	}
	if !contains(fsys.get(target), "alias:") {
		t.Error("aider config missing alias block")
	}
	// Idempotent install.
	if err := a.Install(context.Background(), domain.InstallOptions{}); err != nil {
		t.Fatal(err)
	}
	count := containsCount(fsys.get(target), "alias:")
	if count != 1 {
		t.Errorf("alias appears %d times; want 1", count)
	}
	// Uninstall.
	if err := a.Uninstall(context.Background()); err != nil {
		t.Fatal(err)
	}
	if contains(fsys.get(target), "alias:") {
		t.Errorf("alias still present after uninstall: %q", fsys.get(target))
	}
}

func TestContract_Codex(t *testing.T) {
	fsys := newMemFS()
	home := "/h"
	contract(t, fsys, home, domain.AgentCodex, domain.DefaultConfig(),
		filepath.Join(home, ".codex", "sudosafe.toml"))
}

func TestContract_Generic(t *testing.T) {
	fsys := newMemFS()
	home := "/h"
	// Pre-create bashrc and zshrc.
	fsys.files[filepath.Join(home, ".bashrc")] = []byte("# existing\n")
	fsys.files[filepath.Join(home, ".zshrc")] = []byte("# existing\n")
	a := installInto(t, fsys, home, domain.AgentGeneric, domain.DefaultConfig())
	if err := a.Install(context.Background(), domain.InstallOptions{}); err != nil {
		t.Fatal(err)
	}
	for _, rc := range []string{".bashrc", ".zshrc"} {
		path := filepath.Join(home, rc)
		body := fsys.get(path)
		if !contains(body, marker(domain.AgentGeneric)) {
			t.Errorf("%s missing marker; got %q", rc, body)
		}
	}
	// Idempotent.
	if err := a.Install(context.Background(), domain.InstallOptions{}); err != nil {
		t.Fatal(err)
	}
	for _, rc := range []string{".bashrc", ".zshrc"} {
		path := filepath.Join(home, rc)
		if containsCount(fsys.get(path), marker(domain.AgentGeneric)) != 1 {
			t.Errorf("%s marker duplicated", rc)
		}
	}
	// Uninstall.
	if err := a.Uninstall(context.Background()); err != nil {
		t.Fatal(err)
	}
	for _, rc := range []string{".bashrc", ".zshrc"} {
		path := filepath.Join(home, rc)
		if contains(fsys.get(path), marker(domain.AgentGeneric)) {
			t.Errorf("%s marker still present after uninstall", rc)
		}
	}
}

func TestContract_Copilot_NoGH(t *testing.T) {
	fsys := newMemFS()
	home := "/h"
	a := installInto(t, fsys, home, domain.AgentCopilot, domain.DefaultConfig())
	// Override gh look so it always fails.
	if c, ok := a.(*CopilotAdapter); ok {
		c.ghLook = func(string) (string, error) { return "", context.Canceled }
		c.runCmd = func(context.Context, string, ...string) (string, error) {
			return "", nil
		}
	}
	if err := a.Install(context.Background(), domain.InstallOptions{}); err == nil {
		t.Error("Install should fail when gh is missing")
	}
}

func TestContract_DryRun(t *testing.T) {
	fsys := newMemFS()
	home := "/h"
	a := installInto(t, fsys, home, domain.AgentKilo, domain.DefaultConfig())
	if err := a.Install(context.Background(), domain.InstallOptions{DryRun: true}); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(home, ".config/kilo", "commands", "sudoconsole.md")
	if fsys.exists(target) {
		t.Error("DryRun should not touch the filesystem")
	}
}

func TestRegistry_All(t *testing.T) {
	all, err := All(domain.DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	// 6 dedicated adapters (Generic is opt-in).
	if len(all) != 6 {
		t.Errorf("want 6 installers; got %d", len(all))
	}
}

func contains(haystack, needle string) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}

func containsCount(haystack, needle string) int {
	if needle == "" {
		return 0
	}
	n := 0
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			n++
		}
	}
	return n
}
