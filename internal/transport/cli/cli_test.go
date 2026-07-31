package cli

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LeandroLCD/sudoconsole/internal/domain"
)

func TestRootCmd_Help(t *testing.T) {
	cmd := NewRootCmd(stubApp(t), "test", "abc1234", "now")
	cmd.SetArgs([]string{"--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	s := out.String()
	for _, want := range []string{"sudoconsole", "--config", "--cache-timeout", "--format", "--log-level"} {
		if !strings.Contains(s, want) {
			t.Errorf("help missing %q", want)
		}
	}
}

func TestRootCmd_Version(t *testing.T) {
	cmd := NewRootCmd(stubApp(t), "9.9.9", "deadbeef", "1970-01-01")
	cmd.SetArgs([]string{"version"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	s := out.String()
	if !strings.Contains(s, "9.9.9") || !strings.Contains(s, "deadbeef") {
		t.Errorf("version output unexpected: %q", s)
	}
}

func TestConfigCmd_Path(t *testing.T) {
	dir := t.TempDir()
	cmd := NewRootCmd(stubApp(t), "v", "c", "d")
	cmd.SetArgs([]string{"--config", filepath.Join(dir, "missing.toml"), "config", "path"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(out.String(), filepath.Join(dir, "missing.toml")) {
		t.Errorf("config path missing: %q", out.String())
	}
}

func TestConfigCmd_Show_Defaults(t *testing.T) {
	dir := t.TempDir()
	cmd := NewRootCmd(stubApp(t), "v", "c", "d")
	cmd.SetArgs([]string{"--config", filepath.Join(dir, "missing.toml"), "config", "show"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(out.String(), "[cache]") {
		t.Errorf("missing [cache] section: %q", out.String())
	}
	if !strings.Contains(out.String(), "timeout_seconds") {
		t.Errorf("missing cache.timeout_seconds: %q", out.String())
	}
}

func TestConfigCmd_Show_Overrides(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "c.toml")
	if err := os.WriteFile(cfgPath, []byte(partialTOMLForTest()), 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := NewRootCmd(stubApp(t), "v", "c", "d")
	cmd.SetArgs([]string{
		"--config", cfgPath,
		"--cache-timeout", "1200",
		"--format", "json",
		"config", "show",
	})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(out.String(), "timeout_seconds        = 1200") {
		t.Errorf("flag override not applied: %q", out.String())
	}
	if !strings.Contains(out.String(), "format    = json") {
		t.Errorf("format override not applied: %q", out.String())
	}
}

func TestConfigCmd_Show_InvalidTOML(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "c.toml")
	if err := os.WriteFile(cfgPath, []byte("not = valid = toml =="), 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := NewRootCmd(stubApp(t), "v", "c", "d")
	cmd.SetArgs([]string{"--config", cfgPath, "config", "show"})
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	err := cmd.Execute()
	if err == nil {
		t.Fatalf("expected error, got nil; out=%q", out.String())
	}
	if !strings.Contains(err.Error(), "config:") {
		t.Errorf("error should be tagged with config: %v", err)
	}
}

func TestConfigCmd_Show_InvalidCacheValue(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "c.toml")
	if err := os.WriteFile(cfgPath, []byte("[cache]\ntimeout_seconds = 10\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := NewRootCmd(stubApp(t), "v", "c", "d")
	cmd.SetArgs([]string{"--config", cfgPath, "config", "show"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	err := cmd.Execute()
	if err == nil {
		t.Fatalf("expected validation error; out=%q", out.String())
	}
	if !errors.Is(err, domain.ErrConfigInvalid) {
		t.Errorf("expected ErrConfigInvalid wrapped, got %v", err)
	}
}

func TestConfigCmd_Validate_OK(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "c.toml")
	if err := os.WriteFile(cfgPath, []byte("[cache]\ntimeout_seconds = 600\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := NewRootCmd(stubApp(t), "v", "c", "d")
	cmd.SetArgs([]string{"--config", cfgPath, "config", "validate"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("validate should pass: %v", err)
	}
	if !strings.Contains(out.String(), "ok:") {
		t.Errorf("missing ok: %q", out.String())
	}
}

func TestLoadEffectiveConfig_MissingFileFallsBackToDefaults(t *testing.T) {
	opts := &Options{ConfigPath: filepath.Join(t.TempDir(), "absent.toml")}
	cfg, err := loadEffectiveConfig(context.Background(), opts)
	if err != nil {
		t.Fatalf("missing file should be OK: %v", err)
	}
	def := domain.DefaultConfig()
	if cfg.Cache.TimeoutSeconds != def.Cache.TimeoutSeconds {
		t.Errorf("got %d want default %d", cfg.Cache.TimeoutSeconds, def.Cache.TimeoutSeconds)
	}
}

func TestApplyOverrides_OnlySet(t *testing.T) {
	cfg := domain.DefaultConfig()
	original := cfg.Cache.TimeoutSeconds
	opts := &Options{CacheTimeoutSeconds: 0, Format: ""}
	applyOverrides(&cfg, opts)
	if cfg.Cache.TimeoutSeconds != original {
		t.Errorf("zero flag should not change value: got %d", cfg.Cache.TimeoutSeconds)
	}
	applyOverrides(&cfg, &Options{CacheTimeoutSeconds: 300})
	if cfg.Cache.TimeoutSeconds != 300 {
		t.Errorf("flag override not applied: %d", cfg.Cache.TimeoutSeconds)
	}
}

func partialTOMLForTest() string {
	return `
[cache]
timeout_seconds = 900

[output]
format = "human"
`
}
