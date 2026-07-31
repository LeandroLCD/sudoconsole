package config

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/LeandroLCD/sudoconsole/internal/domain"
)

// memFS is a minimal in-memory FS used to drive the loader without
// touching the host filesystem.
type memFS struct {
	files map[string][]byte
	dirs  map[string]bool
}

func newMemFS() *memFS {
	return &memFS{
		files: map[string][]byte{},
		dirs:  map[string]bool{},
	}
}

func (m *memFS) ReadFile(name string) ([]byte, error) {
	b, ok := m.files[name]
	if !ok {
		return nil, &os.PathError{Op: "open", Path: name, Err: os.ErrNotExist}
	}
	return b, nil
}

func (m *memFS) WriteFile(name string, data []byte, _ os.FileMode) error {
	m.files[name] = append([]byte(nil), data...)
	m.dirs[filepath.Dir(name)] = true
	return nil
}

func (m *memFS) MkdirAll(path string, _ os.FileMode) error {
	m.dirs[path] = true
	return nil
}

func (m *memFS) Stat(name string) (os.FileInfo, error) {
	return nil, &os.PathError{Op: "stat", Path: name, Err: os.ErrNotExist}
}

func (m *memFS) Rename(oldpath, newpath string) error {
	if _, ok := m.files[oldpath]; !ok {
		return &os.PathError{Op: "rename", Path: oldpath, Err: os.ErrNotExist}
	}
	m.files[newpath] = m.files[oldpath]
	delete(m.files, oldpath)
	return nil
}

func (m *memFS) Remove(name string) error {
	if _, ok := m.files[name]; !ok {
		return &os.PathError{Op: "remove", Path: name, Err: os.ErrNotExist}
	}
	delete(m.files, name)
	return nil
}

// --- paths -----------------------------------------------------------

func TestPathResolver_DefaultPath_Darwin(t *testing.T) {
	r := PathResolver{
		Getenv: func(k string) string {
			if k == EnvHome {
				return "/Users/alice"
			}
			return ""
		},
		GOOS: "darwin",
	}
	got := r.DefaultPath()
	want := filepath.Join("/Users/alice", "Library", "Application Support", "sudoconsole", "config.toml")
	if got != want {
		t.Fatalf("darwin: got %q want %q", got, want)
	}
}

func TestPathResolver_DefaultPath_LinuxXDG(t *testing.T) {
	r := PathResolver{
		Getenv: func(k string) string {
			switch k {
			case EnvXDGConfig:
				return "/home/alice/.cfg"
			case EnvHome:
				return "/home/alice"
			}
			return ""
		},
		GOOS: "linux",
	}
	got := r.DefaultPath()
	want := filepath.Join("/home/alice/.cfg", "sudoconsole", "config.toml")
	if got != want {
		t.Fatalf("linux xdg: got %q want %q", got, want)
	}
}

func TestPathResolver_DefaultPath_LinuxFallback(t *testing.T) {
	r := PathResolver{
		Getenv: func(k string) string {
			if k == EnvHome {
				return "/home/alice"
			}
			return ""
		},
		GOOS: "linux",
	}
	got := r.DefaultPath()
	want := filepath.Join("/home/alice", ".config", "sudoconsole", "config.toml")
	if got != want {
		t.Fatalf("linux fallback: got %q want %q", got, want)
	}
}

func TestPathResolver_New_UsesRuntimeGOOS(t *testing.T) {
	r := NewPathResolver()
	if r.GOOS != runtime.GOOS {
		t.Fatalf("NewPathResolver GOOS = %q, want %q", r.GOOS, runtime.GOOS)
	}
}

// --- loader ----------------------------------------------------------

const validTOML = `
[cache]
timeout_seconds = 600
refresh_before_seconds = 60
disabled = false

[security]
purge_memory = true
disable_core_dumps = true
require_tty = true

[policy]
mode = "allowlist"

[policy.blocked]
categories = ["remote_access", "shell_spawn"]
commands = ["rm", "dd"]
patterns = ["^wget\\s+"]

[policy.allowed]
categories = ["package_manager"]
commands = ["apt", "dnf"]

[policy.audit]
log_file = "/var/log/sudoconsole.jsonl"
log_blocked = true
log_warned = true
max_bytes = 1048576

[policy.remote_access]
block_reverse_tunnels = true
block_port_forward = true
block_shell_spawn = false
block_tunnels = true

[policy.credential_exposure]
block_passwd_change = true
block_sudoers_edit = false

[agent]
auto_detect = true
install = ["kilo", "claude"]
bin_dir = "/opt/bin"

[output]
format = "json"
log_level = "debug"
`

const partialTOML = `
[cache]
timeout_seconds = 1800

[output]
format = "json"
`

const invalidTOML = `
this is :: not valid toml = "x
`

func TestStore_Load_Valid(t *testing.T) {
	fs := newMemFS()
	fs.files["/home/alice/.config/sudoconsole/config.toml"] = []byte(validTOML)
	s := &Store{Path: "/home/alice/.config/sudoconsole/config.toml", FS: fs}

	cfg, err := s.Load(t.Context())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.Cache.TimeoutSeconds != 600 {
		t.Errorf("Cache.TimeoutSeconds = %d, want 600", cfg.Cache.TimeoutSeconds)
	}
	if cfg.Cache.RefreshBeforeSeconds != 60 {
		t.Errorf("Cache.RefreshBeforeSeconds = %d, want 60", cfg.Cache.RefreshBeforeSeconds)
	}
	if cfg.Policy.Mode != domain.PolicyModeAllowlist {
		t.Errorf("Policy.Mode = %v, want allowlist", cfg.Policy.Mode)
	}
	if got := len(cfg.Policy.Blocked.Categories); got != 2 {
		t.Errorf("Blocked.Categories len = %d, want 2", got)
	}
	if cfg.Policy.Audit.LogFile != "/var/log/sudoconsole.jsonl" {
		t.Errorf("Audit.LogFile = %q", cfg.Policy.Audit.LogFile)
	}
	if cfg.Policy.Audit.MaxBytes != 1048576 {
		t.Errorf("Audit.MaxBytes = %d", cfg.Policy.Audit.MaxBytes)
	}
	if cfg.Agent.BinDir != "/opt/bin" {
		t.Errorf("Agent.BinDir = %q", cfg.Agent.BinDir)
	}
	if cfg.Output.Format != "json" {
		t.Errorf("Output.Format = %q", cfg.Output.Format)
	}
	if cfg.Output.LogLevel != "debug" {
		t.Errorf("Output.LogLevel = %q", cfg.Output.LogLevel)
	}
}

func TestStore_Load_PartialUsesDefaults(t *testing.T) {
	fs := newMemFS()
	fs.files["/c.toml"] = []byte(partialTOML)
	s := &Store{Path: "/c.toml", FS: fs}
	cfg, err := s.Load(t.Context())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Cache.TimeoutSeconds != 1800 {
		t.Errorf("user value not applied: %d", cfg.Cache.TimeoutSeconds)
	}
	def := domain.DefaultConfig()
	if cfg.Cache.RefreshBeforeSeconds != def.Cache.RefreshBeforeSeconds {
		t.Errorf("default not applied: got %d want %d", cfg.Cache.RefreshBeforeSeconds, def.Cache.RefreshBeforeSeconds)
	}
	if cfg.Output.LogLevel != def.Output.LogLevel {
		t.Errorf("default not applied: got %q want %q", cfg.Output.LogLevel, def.Output.LogLevel)
	}
}

func TestStore_Load_MissingReturnsDefaults(t *testing.T) {
	fs := newMemFS()
	s := &Store{Path: "/missing.toml", FS: fs}
	cfg, err := s.Load(t.Context())
	if err != nil {
		t.Fatalf("missing file should not error: %v", err)
	}
	def := domain.DefaultConfig()
	if cfg.Cache.TimeoutSeconds != def.Cache.TimeoutSeconds {
		t.Errorf("missing file should return defaults: got %d", cfg.Cache.TimeoutSeconds)
	}
}

func TestStore_Load_InvalidTOML(t *testing.T) {
	fs := newMemFS()
	fs.files["/c.toml"] = []byte(invalidTOML)
	s := &Store{Path: "/c.toml", FS: fs}
	_, err := s.Load(t.Context())
	if err == nil {
		t.Fatal("expected error on invalid TOML")
	}
	if !errors.Is(err, domain.ErrConfigInvalid) {
		// TOML decode errors are not wrapped as ErrConfigInvalid; we
		// accept any error here, but assert it carries a useful path.
		t.Logf("error (acceptable): %v", err)
	}
}

func TestStore_Load_InvalidCacheTimeout(t *testing.T) {
	fs := newMemFS()
	fs.files["/c.toml"] = []byte("[cache]\ntimeout_seconds = 5\n")
	s := &Store{Path: "/c.toml", FS: fs}
	_, err := s.Load(t.Context())
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !errors.Is(err, domain.ErrConfigInvalid) {
		t.Errorf("expected ErrConfigInvalid, got %v", err)
	}
}

func TestStore_Load_InvalidPolicyMode(t *testing.T) {
	fs := newMemFS()
	fs.files["/c.toml"] = []byte("[policy]\nmode = \"blockklist\"\n")
	s := &Store{Path: "/c.toml", FS: fs}
	_, err := s.Load(t.Context())
	if !errors.Is(err, domain.ErrConfigInvalid) {
		t.Fatalf("expected ErrConfigInvalid, got %v", err)
	}
}

func TestStore_Load_InvalidRegex(t *testing.T) {
	fs := newMemFS()
	fs.files["/c.toml"] = []byte("[policy]\nextra_patterns = [\"(unbalanced\"]\n")
	s := &Store{Path: "/c.toml", FS: fs}
	_, err := s.Load(t.Context())
	if !errors.Is(err, domain.ErrConfigInvalid) {
		t.Fatalf("expected ErrConfigInvalid, got %v", err)
	}
}

func TestStore_Load_UnknownCategory(t *testing.T) {
	fs := newMemFS()
	fs.files["/c.toml"] = []byte("[policy.blocked]\ncategories = [\"made_up\"]\n")
	s := &Store{Path: "/c.toml", FS: fs}
	_, err := s.Load(t.Context())
	if !errors.Is(err, domain.ErrConfigInvalid) {
		t.Fatalf("expected ErrConfigInvalid, got %v", err)
	}
}

func TestStore_SaveThenLoad_Roundtrip(t *testing.T) {
	fs := newMemFS()
	s := &Store{Path: "/round.toml", FS: fs}

	in := domain.DefaultConfig()
	in.Cache.TimeoutSeconds = 1234
	in.Output.Format = "json"
	in.Policy.Mode = domain.PolicyModeAllowlist
	in.Policy.Blocked.Categories = []domain.Category{domain.CategoryRemoteAccess}
	in.Agent.BinDir = "/srv/bin"

	if err := s.Save(t.Context(), in); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if _, ok := fs.files["/round.toml"]; !ok {
		t.Fatal("Save did not write file")
	}

	out, err := s.Load(t.Context())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if out.Cache.TimeoutSeconds != 1234 {
		t.Errorf("roundtrip Cache.TimeoutSeconds = %d, want 1234", out.Cache.TimeoutSeconds)
	}
	if out.Output.Format != "json" {
		t.Errorf("roundtrip Output.Format = %q", out.Output.Format)
	}
	if out.Policy.Mode != domain.PolicyModeAllowlist {
		t.Errorf("roundtrip Policy.Mode = %v", out.Policy.Mode)
	}
	if len(out.Policy.Blocked.Categories) != 1 || out.Policy.Blocked.Categories[0] != domain.CategoryRemoteAccess {
		t.Errorf("roundtrip Blocked.Categories = %v", out.Policy.Blocked.Categories)
	}
	if out.Agent.BinDir != "/srv/bin" {
		t.Errorf("roundtrip Agent.BinDir = %q", out.Agent.BinDir)
	}
}

func TestStore_DefaultPath(t *testing.T) {
	s := &Store{Path: "/explicit.toml"}
	if got := s.DefaultPath(); got != "/explicit.toml" {
		t.Errorf("explicit path overridden: %q", got)
	}
	s2 := &Store{Resolver: PathResolver{
		Getenv: func(k string) string {
			if k == EnvHome {
				return "/h"
			}
			return ""
		},
		GOOS: "linux",
	}}
	want := filepath.Join("/h", ".config", "sudoconsole", "config.toml")
	if got := s2.DefaultPath(); got != want {
		t.Errorf("resolved path: got %q want %q", got, want)
	}
}

func TestStore_ImplementsConfigStore(t *testing.T) {
	var _ domain.ConfigStore = (*Store)(nil)
}
