package agent

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"testing/fstest"
	"time"

	"github.com/LeandroLCD/sudoconsole/internal/domain"
)

func newDetectorForTest(home string, paths []string, fsys fs.FS) *Detector {
	return &Detector{
		LookupEnv:        func(string) string { return "" },
		UserHomeDir:      func() (string, error) { return home, nil },
		FS:               fsys,
		ExecutableLookup: nil,
		Timeout:          500 * time.Millisecond,
		PathLayout:       func() []string { return paths },
	}
}

func TestDetector_NoAgents(t *testing.T) {
	fsys := fstest.MapFS{}
	d := newDetectorForTest("/home/u", nil, fsys)
	out, err := d.Detect(context.Background())
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(out) != 0 {
		t.Errorf("expected 0 agents, got %d: %+v", len(out), out)
	}
}

func TestDetector_BinaryOnPath(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "kilo")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\necho kilo 0.1.0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	// make it executable even on platforms that ignore mode bits.
	if runtime.GOOS != "windows" {
		_ = os.Chmod(bin, 0o755)
	}
	fsys := fstest.MapFS{}
	d := newDetectorForTest("/home/u", []string{dir}, fsys)
	out, err := d.Detect(context.Background())
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("want 1 agent, got %d: %+v", len(out), out)
	}
	if out[0].Kind != domain.AgentKilo {
		t.Errorf("Kind = %v", out[0].Kind)
	}
	if out[0].BinaryPath == "" {
		t.Errorf("BinaryPath empty")
	}
}

func TestDetector_ConfigDirOnly(t *testing.T) {
	home := "/home/u"
	fsys := fstest.MapFS{
		"home/u/.claude":        &fstest.MapFile{Mode: fs.ModeDir | 0o755},
		"home/u/.aider":         &fstest.MapFile{Mode: fs.ModeDir | 0o755},
		"home/u/.config/kilo":   &fstest.MapFile{Mode: fs.ModeDir | 0o755},
		"home/u/.config/gemini": &fstest.MapFile{Mode: fs.ModeDir | 0o755},
	}
	d := newDetectorForTest(home, nil, fsys)
	out, err := d.Detect(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	got := kinds(out)
	want := []domain.AgentKind{domain.AgentKilo, domain.AgentClaude, domain.AgentGemini, domain.AgentAider}
	if !equalKinds(got, want) {
		t.Errorf("kinds = %v, want %v", got, want)
	}
}

func TestDetector_DetectOne(t *testing.T) {
	fsys := fstest.MapFS{}
	d := newDetectorForTest("/home/u", nil, fsys)
	if _, err := d.DetectOne(context.Background(), domain.AgentClaude); !errIs(err, domain.ErrAgentNotDetected) {
		t.Errorf("expected ErrAgentNotDetected; got %v", err)
	}
	if _, err := d.DetectOne(context.Background(), domain.AgentUnknown); err == nil {
		t.Errorf("expected error for unknown kind")
	}
}

func TestDetector_TimeoutIsEnforced(t *testing.T) {
	// Use a zero-timeout detector: Detect should still return
	// without hanging (it returns whatever was probed before the
	// deadline expired).
	d := &Detector{
		LookupEnv:        func(string) string { return "" },
		UserHomeDir:      func() (string, error) { return "/nonexistent", nil },
		FS:               fstest.MapFS{},
		ExecutableLookup: nil,
		Timeout:          1 * time.Millisecond,
		PathLayout:       func() []string { return []string{"/nonexistent"} },
	}
	start := time.Now()
	if _, err := d.Detect(context.Background()); err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if elapsed := time.Since(start); elapsed > 500*time.Millisecond {
		t.Errorf("Detect took %v, expected <500ms", elapsed)
	}
}

func TestDetector_Parallel(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"kilo", "claude", "gemini"} {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	fsys := fstest.MapFS{}
	d := newDetectorForTest("/home/u", []string{dir}, fsys)
	out, err := d.Detect(context.Background())
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(out) != 3 {
		t.Errorf("want 3, got %d", len(out))
	}
}

func TestDetector_GenericNeverDetected(t *testing.T) {
	fsys := fstest.MapFS{}
	d := newDetectorForTest("/home/u", nil, fsys)
	out, err := d.Detect(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range out {
		if a.Kind == domain.AgentGeneric {
			t.Errorf("generic should never be reported: %+v", a)
		}
	}
}

func TestDetector_ImplementsAgentDetector(t *testing.T) {
	var _ domain.AgentDetector = (*Detector)(nil)
}

func TestSpecs_AreUnique(t *testing.T) {
	seen := map[domain.AgentKind]bool{}
	for _, s := range Specs {
		if seen[s.Kind] {
			t.Errorf("duplicate spec for kind %v", s.Kind)
		}
		seen[s.Kind] = true
	}
}

// --- helpers ----------------------------------------------------------

func kinds(d []domain.AgentDescriptor) []domain.AgentKind {
	out := make([]domain.AgentKind, 0, len(d))
	for _, x := range d {
		out = append(out, x.Kind)
	}
	return out
}

func equalKinds(a, b []domain.AgentKind) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func errIs(err, target error) bool {
	return errors.Is(err, target)
}
