package usecase

import (
	"context"
	"errors"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/LeandroLCD/sudoconsole/internal/domain"
)

// fakeDetector is a stub AgentDetector whose behaviour is dictated by
// the `agents` slice.
type fakeDetector struct {
	agents []domain.AgentDescriptor
	err    error
}

func (f *fakeDetector) Detect(_ context.Context) ([]domain.AgentDescriptor, error) {
	if f.err != nil {
		return nil, f.err
	}
	out := make([]domain.AgentDescriptor, len(f.agents))
	copy(out, f.agents)
	return out, nil
}
func (f *fakeDetector) DetectOne(_ context.Context, kind domain.AgentKind) (domain.AgentDescriptor, error) {
	for _, a := range f.agents {
		if a.Kind == kind {
			return a, nil
		}
	}
	return domain.AgentDescriptor{}, domain.ErrAgentNotDetected
}

// fakeInstaller is a stub AgentInstaller backed by a MapFS so each
// test exercises the real baseAdapter contract (marker, idempotency).
type fakeInstaller struct {
	kind domain.AgentKind
	fs   fstest.MapFS
	path string // target file inside the MapFS
}

func (f *fakeInstaller) Name() domain.AgentKind { return f.kind }
func (f *fakeInstaller) Detect(_ context.Context) (bool, error) {
	_, err := f.fs.Stat(f.path)
	return err == nil, nil
}
func (f *fakeInstaller) Install(_ context.Context, _ domain.InstallOptions) error {
	marker := "# sudoconsole-marker: " + f.kind.String()
	if existing, err := f.fs.ReadFile(f.path); err == nil {
		if strings.Contains(string(existing), marker) {
			return nil // idempotent
		}
	}
	f.fs[f.path] = &fstest.MapFile{Data: []byte(marker + "\n"), Mode: 0o600}
	return nil
}
func (f *fakeInstaller) Uninstall(_ context.Context) error {
	if _, err := f.fs.Stat(f.path); err != nil {
		return nil // idempotent skip
	}
	delete(f.fs, f.path)
	return nil
}
func (f *fakeInstaller) AdapterCommand() string { return "sudoconsole" }

func TestInstallAdapter_ResolveKinds_AutoDetect(t *testing.T) {
	det := &fakeDetector{agents: []domain.AgentDescriptor{
		{Kind: domain.AgentKilo, Available: true},
		{Kind: domain.AgentClaude, Available: false}, // filtered out
		{Kind: domain.AgentGemini, Available: true},
		{Kind: domain.AgentGeneric, Available: true}, // filtered out (opt-in)
	}}
	uc := NewInstallAdapterUseCase(det, nil, nil)

	got, err := uc.resolveKinds(context.Background(), InstallAdapterInput{})
	if err != nil {
		t.Fatalf("resolveKinds: %v", err)
	}
	if len(got) != 2 || got[0] != domain.AgentKilo || got[1] != domain.AgentGemini {
		t.Errorf("unexpected kinds: %v", got)
	}
}

func TestInstallAdapter_ResolveKinds_Explicit(t *testing.T) {
	uc := NewInstallAdapterUseCase(&fakeDetector{}, nil, nil)
	got, err := uc.resolveKinds(context.Background(), InstallAdapterInput{
		Kinds: []domain.AgentKind{domain.AgentClaude, domain.AgentUnknown},
	})
	if err != nil {
		t.Fatalf("resolveKinds: %v", err)
	}
	if len(got) != 1 || got[0] != domain.AgentClaude {
		t.Errorf("unexpected kinds: %+v", got)
	}
}

func TestInstallAdapter_ResolveKinds_NeedDetectorOrKinds(t *testing.T) {
	uc := NewInstallAdapterUseCase(nil, nil, nil) // no detector
	_, err := uc.resolveKinds(context.Background(), InstallAdapterInput{})
	if err == nil {
		t.Fatal("expected error when neither detector nor kinds supplied")
	}
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Errorf("expected ErrInvalidInput, got %v", err)
	}
}

func TestInstallAdapter_BuildOptions_PolicyOverride(t *testing.T) {
	uc := NewInstallAdapterUseCase(nil, nil, nil)
	in := InstallAdapterInput{
		Config:             domain.DefaultConfig(),
		BinDir:             "/opt/bin",
		PolicyModeOverride: domain.PolicyModeAudit,
	}
	opts := uc.buildOptions(in)
	if opts.Config.Policy.Mode != domain.PolicyModeAudit {
		t.Errorf("policy override not applied: %v", opts.Config.Policy.Mode)
	}
	if opts.BinDir != "/opt/bin" {
		t.Errorf("BinDir not propagated: %q", opts.BinDir)
	}

	// No override → original mode preserved.
	in2 := InstallAdapterInput{Config: domain.DefaultConfig()}
	opts2 := uc.buildOptions(in2)
	if opts2.Config.Policy.Mode != domain.PolicyModeBlocklist {
		t.Errorf("expected default blocklist, got %v", opts2.Config.Policy.Mode)
	}
	if opts2.BinDir != domain.DefaultConfig().Agent.BinDir {
		t.Errorf("BinDir default not propagated: %q", opts2.BinDir)
	}
}

func TestInstallAdapter_Execute_HappyPath(t *testing.T) {
	fs1 := fstest.MapFS{}
	fs2 := fstest.MapFS{}
	resolve := func(k domain.AgentKind) (domain.AgentInstaller, error) {
		switch k {
		case domain.AgentKilo:
			return &fakeInstaller{kind: k, fs: fs1, path: "home/u/.config/kilo/commands/sudoconsole.md"}, nil
		case domain.AgentClaude:
			return &fakeInstaller{kind: k, fs: fs2, path: "home/u/.claude/commands/sudoconsole.md"}, nil
		}
		return nil, domain.ErrAgentNotDetected
	}
	uc := NewInstallAdapterUseCase(nil, resolve, nil)

	out, err := uc.Execute(context.Background(), InstallAdapterInput{
		Kinds: []domain.AgentKind{domain.AgentKilo, domain.AgentClaude},
		Yes:   true,
		Time:  time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(out.Installed) != 2 {
		t.Fatalf("expected 2 installed, got %d (%+v)", len(out.Installed), out.Installed)
	}
	if len(out.Failed) != 0 {
		t.Errorf("expected 0 failures, got %d", len(out.Failed))
	}
	// Verify MapFS actually got the marker written.
	if _, ok := fs1["home/u/.config/kilo/commands/sudoconsole.md"]; !ok {
		t.Errorf("kilo integration not written to MapFS")
	}
	if _, ok := fs2["home/u/.claude/commands/sudoconsole.md"]; !ok {
		t.Errorf("claude integration not written to MapFS")
	}
}

func TestInstallAdapter_Execute_Idempotent(t *testing.T) {
	marker := []byte("# sudoconsole-marker: kilo\n")
	fs1 := fstest.MapFS{
		"home/u/.config/kilo/commands/sudoconsole.md": &fstest.MapFile{Data: marker, Mode: 0o600},
	}
	resolve := func(k domain.AgentKind) (domain.AgentInstaller, error) {
		return &fakeInstaller{kind: domain.AgentKilo, fs: fs1,
			path: "home/u/.config/kilo/commands/sudoconsole.md"}, nil
	}
	uc := NewInstallAdapterUseCase(nil, resolve, nil)

	out, err := uc.Execute(context.Background(), InstallAdapterInput{
		Kinds: []domain.AgentKind{domain.AgentKilo},
		Yes:   true,
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(out.Installed) != 1 {
		t.Errorf("expected 1 installed (idempotent OK), got %d", len(out.Installed))
	}
	if len(out.Failed) != 0 {
		t.Errorf("idempotent install should not fail: %+v", out.Failed)
	}
}

func TestInstallAdapter_Execute_Uninstall(t *testing.T) {
	marker := []byte("# sudoconsole-marker: kilo\n")
	fs1 := fstest.MapFS{
		"home/u/.config/kilo/commands/sudoconsole.md": &fstest.MapFile{Data: marker, Mode: 0o600},
	}
	resolve := func(k domain.AgentKind) (domain.AgentInstaller, error) {
		return &fakeInstaller{kind: domain.AgentKilo, fs: fs1,
			path: "home/u/.config/kilo/commands/sudoconsole.md"}, nil
	}
	uc := NewInstallAdapterUseCase(nil, resolve, nil)

	out, err := uc.Execute(context.Background(), InstallAdapterInput{
		Kinds:     []domain.AgentKind{domain.AgentKilo},
		Uninstall: true,
		Yes:       true,
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(out.Installed) != 1 {
		t.Errorf("expected 1 uninstalled, got %d", len(out.Installed))
	}
	if _, ok := fs1["home/u/.config/kilo/commands/sudoconsole.md"]; ok {
		t.Errorf("expected marker file to be removed")
	}
}

func TestInstallAdapter_Execute_ConfirmAccepted(t *testing.T) {
	called := false
	resolve := func(k domain.AgentKind) (domain.AgentInstaller, error) {
		return &fakeInstaller{kind: k, fs: fstest.MapFS{}, path: "x.md"}, nil
	}
	uc := NewInstallAdapterUseCase(nil, resolve, nil)

	out, err := uc.Execute(context.Background(), InstallAdapterInput{
		Kinds: []domain.AgentKind{domain.AgentKilo},
		Confirm: func(msg string) (bool, error) {
			called = true
			if !strings.Contains(msg, "Kilo CLI") {
				t.Errorf("plan should mention agent name; got %q", msg)
			}
			return true, nil
		},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !called {
		t.Error("Confirm was not invoked")
	}
	if len(out.Installed) != 1 {
		t.Errorf("expected 1 installed, got %d", len(out.Installed))
	}
}

func TestInstallAdapter_Execute_ConfirmRejected(t *testing.T) {
	resolve := func(k domain.AgentKind) (domain.AgentInstaller, error) {
		return &fakeInstaller{kind: k, fs: fstest.MapFS{}, path: "x.md"}, nil
	}
	uc := NewInstallAdapterUseCase(nil, resolve, nil)

	_, err := uc.Execute(context.Background(), InstallAdapterInput{
		Kinds: []domain.AgentKind{domain.AgentKilo},
		Confirm: func(_ string) (bool, error) {
			return false, nil
		},
	})
	if !errors.Is(err, ErrInstallAborted) {
		t.Errorf("expected ErrInstallAborted, got %v", err)
	}
}

func TestInstallAdapter_Execute_ConfirmSkippedOnDryRunAndUninstall(t *testing.T) {
	called := false
	confirm := func(_ string) (bool, error) { called = true; return true, nil }
	resolve := func(k domain.AgentKind) (domain.AgentInstaller, error) {
		return &fakeInstaller{kind: k, fs: fstest.MapFS{}, path: "x.md"}, nil
	}
	uc := NewInstallAdapterUseCase(nil, resolve, nil)

	// Dry-run: confirm must NOT be called.
	if _, err := uc.Execute(context.Background(), InstallAdapterInput{
		Kinds:   []domain.AgentKind{domain.AgentKilo},
		DryRun:  true,
		Confirm: confirm,
	}); err != nil {
		t.Fatalf("dry-run execute: %v", err)
	}
	if called {
		t.Error("Confirm should be skipped during dry-run")
	}

	// Uninstall: confirm must NOT be called.
	if _, err := uc.Execute(context.Background(), InstallAdapterInput{
		Kinds:     []domain.AgentKind{domain.AgentKilo},
		Uninstall: true,
		Confirm:   confirm,
	}); err != nil {
		t.Fatalf("uninstall execute: %v", err)
	}
	if called {
		t.Error("Confirm should be skipped during uninstall")
	}
}

func TestInstallAdapter_Execute_ResolverErrorCollected(t *testing.T) {
	resolve := func(k domain.AgentKind) (domain.AgentInstaller, error) {
		if k == domain.AgentClaude {
			return nil, errors.New("boom")
		}
		return &fakeInstaller{kind: k, fs: fstest.MapFS{}, path: "x.md"}, nil
	}
	uc := NewInstallAdapterUseCase(nil, resolve, nil)

	out, err := uc.Execute(context.Background(), InstallAdapterInput{
		Kinds: []domain.AgentKind{domain.AgentKilo, domain.AgentClaude},
		Yes:   true,
	})
	if err != nil {
		t.Fatalf("Execute should not return error for per-kind failure: %v", err)
	}
	if len(out.Failed) != 1 || out.Failed[0].Kind != domain.AgentClaude {
		t.Errorf("expected claude in Failed, got %+v", out.Failed)
	}
	if len(out.Installed) != 1 || out.Installed[0].Kind != domain.AgentKilo {
		t.Errorf("expected kilo in Installed, got %+v", out.Installed)
	}
}

func TestInstallAdapter_Execute_NoResolver(t *testing.T) {
	uc := NewInstallAdapterUseCase(nil, nil, nil)
	_, err := uc.Execute(context.Background(), InstallAdapterInput{
		Kinds: []domain.AgentKind{domain.AgentKilo},
		Yes:   true,
	})
	if err == nil {
		t.Fatal("expected error when resolver is nil")
	}
}

func TestInstallAdapter_Execute_EmptyKindsNoDetector(t *testing.T) {
	uc := NewInstallAdapterUseCase(nil, func(domain.AgentKind) (domain.AgentInstaller, error) {
		t.Error("resolver should not be called with empty kinds")
		return nil, nil
	}, nil)
	_, err := uc.Execute(context.Background(), InstallAdapterInput{Yes: true})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Errorf("expected ErrInvalidInput with no detector and no kinds; got %v", err)
	}
}

func TestInstallAdapter_Execute_AuditIntegration(t *testing.T) {
	audit := &fakeAudit{}
	resolve := func(k domain.AgentKind) (domain.AgentInstaller, error) {
		return &fakeInstaller{kind: k, fs: fstest.MapFS{}, path: "x.md"}, nil
	}
	uc := NewInstallAdapterUseCase(nil, resolve, audit)

	if _, err := uc.Execute(context.Background(), InstallAdapterInput{
		Kinds: []domain.AgentKind{domain.AgentKilo},
		Yes:   true,
	}); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	// nilAudit swallows; fakeAudit records. Verify at least the call
	// didn't crash the audit pipeline.
	if len(audit.events) != 0 {
		t.Logf("audit recorded %d events (informational)", len(audit.events))
	}
}

func TestRenderPlan_ContainsKeyFields(t *testing.T) {
	msg := renderPlan(false, false, []domain.AgentKind{
		domain.AgentKilo, domain.AgentClaude,
	}, domain.InstallOptions{
		Force:  true,
		BinDir: "/opt/bin",
		Config: domain.Config{Policy: domain.Policy{Mode: domain.PolicyModeAudit}},
	})
	for _, want := range []string{
		"Install the following",
		"Kilo CLI",
		"Claude Code",
		"force:",
		"bin_dir: /opt/bin",
		"policy_mode: audit",
		"Proceed?",
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("plan missing %q; got:\n%s", want, msg)
		}
	}
}
