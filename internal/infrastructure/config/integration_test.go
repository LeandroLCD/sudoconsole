//go:build integration

package config

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/LeandroLCD/sudoconsole/internal/domain"
)

// These tests are gated behind the `integration` build tag and read
// the fixtures under ../../../testdata/config. They are meant for the
// full test matrix run; fast unit tests live in loader_test.go.

func TestFixture_Valid(t *testing.T) {
	repoRoot, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	store := NewAt(filepath.Join(repoRoot, "testdata", "config", "valid.toml"))
	cfg, err := store.Load(context.Background())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Policy.Mode != domain.PolicyModeAllowlist {
		t.Errorf("Policy.Mode = %v, want allowlist", cfg.Policy.Mode)
	}
}

func TestFixture_InvalidCache(t *testing.T) {
	repoRoot, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	store := NewAt(filepath.Join(repoRoot, "testdata", "config", "invalid_cache.toml"))
	_, err = store.Load(context.Background())
	if !errors.Is(err, domain.ErrConfigInvalid) {
		t.Fatalf("expected ErrConfigInvalid, got %v", err)
	}
}

func TestFixture_Malformed(t *testing.T) {
	repoRoot, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	store := NewAt(filepath.Join(repoRoot, "testdata", "config", "malformed.toml"))
	_, err = store.Load(context.Background())
	if err == nil {
		t.Fatal("expected error for malformed TOML")
	}
}

func TestRoundtrip_OnDisk(t *testing.T) {
	dir, err := os.MkdirTemp("", "sudoconsole-cfg-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	path := filepath.Join(dir, "config.toml")
	s := NewAt(path)
	in := domain.DefaultConfig()
	in.Cache.TimeoutSeconds = 777
	if err := s.Save(context.Background(), in); err != nil {
		t.Fatalf("Save: %v", err)
	}
	out, err := s.Load(context.Background())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if out.Cache.TimeoutSeconds != 777 {
		t.Errorf("roundtrip Cache.TimeoutSeconds = %d, want 777", out.Cache.TimeoutSeconds)
	}
}
