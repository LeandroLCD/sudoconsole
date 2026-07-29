package domain

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"
)

// -- mock implementations used to verify interface contracts compile and work.

type fakePty struct{}

func (fakePty) Allocate(context.Context, []string, []string) (PtySession, error) {
	return nil, errors.New("not implemented")
}
func (fakePty) ReadSecret(context.Context, string) ([]byte, error) { return nil, nil }

type fakeSudo struct{}

func (fakeSudo) Authenticate(context.Context, []byte) error { return nil }
func (fakeSudo) Execute(context.Context, Command) (SudoResult, error) {
	return SudoResult{ExitCode: 0}, nil
}
func (fakeSudo) ExecuteWithAuth(context.Context, []byte, Command) (SudoResult, error) {
	return SudoResult{ExitCode: 0}, nil
}

type fakeCache struct{}

func (fakeCache) IsActive(context.Context, CacheConfig) (CacheStatus, error) { return CacheActive, nil }
func (fakeCache) TimeRemaining(context.Context, CacheConfig) (time.Duration, error) {
	return 5 * time.Minute, nil
}
func (fakeCache) Refresh(context.Context, CacheConfig) error { return nil }

type fakePolicy struct{}

func (fakePolicy) Evaluate(context.Context, Policy, Command) (MatchResult, error) {
	return MatchResult{Decision: DecisionAllow, EvaluatedAt: time.Now()}, nil
}
func (fakePolicy) ListCategories(context.Context) ([]CategoryEntry, error) {
	return []CategoryEntry{{Binary: "ssh", Category: CategoryRemoteAccess, Risk: RiskCritical}}, nil
}

type fakeAudit struct{ buf bytes.Buffer }

func (a *fakeAudit) Log(context.Context, AuditEvent) error { return nil }
func (a *fakeAudit) Close() error                          { return nil }

type fakeConfig struct{}

func (fakeConfig) Load(context.Context) (Config, error) { return DefaultConfig(), nil }
func (fakeConfig) Save(context.Context, Config) error   { return nil }
func (fakeConfig) DefaultPath() string                  { return "/tmp/sudoconsole.toml" }

type fakeDetector struct{}

func (fakeDetector) Detect(context.Context) ([]AgentDescriptor, error) {
	return []AgentDescriptor{{Kind: AgentKilo, Available: true}}, nil
}
func (fakeDetector) DetectOne(context.Context, AgentKind) (AgentDescriptor, error) {
	return AgentDescriptor{Kind: AgentKilo, Available: true}, nil
}

type fakeInstaller struct{}

func (fakeInstaller) Name() AgentKind                               { return AgentKilo }
func (fakeInstaller) Detect(context.Context) (bool, error)          { return true, nil }
func (fakeInstaller) Install(context.Context, InstallOptions) error { return nil }
func (fakeInstaller) Uninstall(context.Context) error               { return nil }
func (fakeInstaller) AdapterCommand() string                        { return "sudoconsole" }

// -- compile-time interface compliance tests.

func TestPorts_CompileTimeCompliance(t *testing.T) {
	var (
		_ PtyGateway      = fakePty{}
		_ SudoGateway     = fakeSudo{}
		_ CacheRepository = fakeCache{}
		_ PolicyEvaluator = fakePolicy{}
		_ AuditLogger     = &fakeAudit{}
		_ ConfigStore     = fakeConfig{}
		_ AgentDetector   = fakeDetector{}
		_ AgentInstaller  = fakeInstaller{}
	)
}

func TestDefaultConfig(t *testing.T) {
	c := DefaultConfig()
	if c.Cache.TimeoutSeconds != 900 {
		t.Fatalf("Cache.TimeoutSeconds = %d", c.Cache.TimeoutSeconds)
	}
	if !c.Security.PurgeMemory {
		t.Fatal("PurgeMemory should default to true")
	}
	if c.Policy.Mode != PolicyModeBlocklist {
		t.Fatalf("Policy.Mode = %v", c.Policy.Mode)
	}
	if c.Output.Format != "human" {
		t.Fatalf("Output.Format = %q", c.Output.Format)
	}
	if c.Agent.BinDir != "~/bin" {
		t.Fatalf("Agent.BinDir = %q", c.Agent.BinDir)
	}
}

func TestDefaultAgentConfig(t *testing.T) {
	c := DefaultAgentConfig()
	if !c.AutoDetect {
		t.Fatal("AutoDetect should default to true")
	}
	if c.Install != nil {
		t.Fatalf("Install should be nil (all), got %v", c.Install)
	}
}

func TestDefaultSecurityConfig(t *testing.T) {
	c := DefaultSecurityConfig()
	if !c.PurgeMemory || !c.DisableCoreDumps || !c.RequireTTY {
		t.Fatalf("security defaults should all be true: %+v", c)
	}
}

func TestDefaultOutputConfig(t *testing.T) {
	c := DefaultOutputConfig()
	if c.Format != "human" || c.LogLevel != "info" {
		t.Fatalf("output defaults wrong: %+v", c)
	}
}

func TestCategoryEntry(t *testing.T) {
	e := CategoryEntry{Binary: "ssh", Category: CategoryRemoteAccess, Risk: RiskCritical, BuiltIn: true}
	if e.Binary != "ssh" || e.Category != CategoryRemoteAccess || e.Risk != RiskCritical {
		t.Fatal("CategoryEntry fields not preserved")
	}
}

func TestAuditEvent_Fields(t *testing.T) {
	c, _ := NewCommand("apt", []string{"update"})
	e := AuditEvent{
		Timestamp: time.Now(),
		User:      "develop",
		Hostname:  "host",
		Command:   c,
		Decision:  DecisionAllow,
		ExitCode:  0,
	}
	if e.User != "develop" || e.Hostname != "host" || e.ExitCode != 0 {
		t.Fatal("AuditEvent fields not preserved")
	}
}

func TestInstallOptions(t *testing.T) {
	opts := InstallOptions{
		Force:  true,
		BinDir: "/usr/local/bin",
		DryRun: false,
		Config: DefaultConfig(),
	}
	if !opts.Force || opts.BinDir != "/usr/local/bin" || opts.Config.Cache.TimeoutSeconds != 900 {
		t.Fatal("InstallOptions fields not preserved")
	}
}

// fakeInstaller should satisfy AgentInstaller (verified by compile-time check above).
// This runtime test exercises each method once.
func TestFakeInstaller_Runtime(t *testing.T) {
	var f AgentInstaller = fakeInstaller{}
	if f.Name() != AgentKilo {
		t.Fatal("Name wrong")
	}
	ok, err := f.Detect(context.Background())
	if err != nil || !ok {
		t.Fatal("Detect wrong")
	}
	if err := f.Install(context.Background(), InstallOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := f.Uninstall(context.Background()); err != nil {
		t.Fatal(err)
	}
	if f.AdapterCommand() != "sudoconsole" {
		t.Fatal("AdapterCommand wrong")
	}
}

func TestFakeDetector_Runtime(t *testing.T) {
	var d AgentDetector = fakeDetector{}
	got, err := d.Detect(context.Background())
	if err != nil || len(got) != 1 || got[0].Kind != AgentKilo {
		t.Fatal("Detect wrong")
	}
	one, err := d.DetectOne(context.Background(), AgentKilo)
	if err != nil || one.Kind != AgentKilo {
		t.Fatal("DetectOne wrong")
	}
}

func TestFakePolicy_Runtime(t *testing.T) {
	var p PolicyEvaluator = fakePolicy{}
	c, _ := NewCommand("apt", nil)
	res, err := p.Evaluate(context.Background(), DefaultPolicy(), c)
	if err != nil || res.Decision != DecisionAllow {
		t.Fatal("Evaluate wrong")
	}
	cats, err := p.ListCategories(context.Background())
	if err != nil || len(cats) == 0 {
		t.Fatal("ListCategories wrong")
	}
}

func TestFakeCache_Runtime(t *testing.T) {
	var c CacheRepository = fakeCache{}
	status, err := c.IsActive(context.Background(), DefaultCacheConfig())
	if err != nil || status != CacheActive {
		t.Fatal("IsActive wrong")
	}
	d, err := c.TimeRemaining(context.Background(), DefaultCacheConfig())
	if err != nil || d <= 0 {
		t.Fatal("TimeRemaining wrong")
	}
	if err := c.Refresh(context.Background(), DefaultCacheConfig()); err != nil {
		t.Fatal(err)
	}
}

func TestFakeSudo_Runtime(t *testing.T) {
	var s SudoGateway = fakeSudo{}
	c, _ := NewCommand("apt", nil)
	if err := s.Authenticate(context.Background(), []byte("x")); err != nil {
		t.Fatal(err)
	}
	r, err := s.Execute(context.Background(), c)
	if err != nil || r.ExitCode != 0 {
		t.Fatal("Execute wrong")
	}
	r, err = s.ExecuteWithAuth(context.Background(), []byte("x"), c)
	if err != nil || r.ExitCode != 0 {
		t.Fatal("ExecuteWithAuth wrong")
	}
}

func TestFakeConfig_Runtime(t *testing.T) {
	var c ConfigStore = fakeConfig{}
	cfg, err := c.Load(context.Background())
	if err != nil || cfg.Cache.TimeoutSeconds != 900 {
		t.Fatal("Load wrong")
	}
	if err := c.Save(context.Background(), cfg); err != nil {
		t.Fatal(err)
	}
	if c.DefaultPath() == "" {
		t.Fatal("DefaultPath empty")
	}
}
