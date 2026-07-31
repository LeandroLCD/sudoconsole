package usecase

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/LeandroLCD/sudoconsole/internal/domain"
)

// --- fakes --------------------------------------------------------------

type fakeGateway struct {
	authErr  error
	execErr  error
	lastAuth []byte
	lastCmd  domain.Command
	mu       sync.Mutex
}

func (f *fakeGateway) Authenticate(_ context.Context, secret []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.lastAuth = append([]byte(nil), secret...)
	return f.authErr
}

func (f *fakeGateway) Execute(_ context.Context, cmd domain.Command) (domain.SudoResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.lastCmd = cmd
	if f.execErr != nil {
		return domain.SudoResult{}, f.execErr
	}
	return domain.SudoResult{Stdout: "ok\n", ExitCode: 0, Duration: 10 * time.Millisecond}, nil
}

func (f *fakeGateway) ExecuteWithAuth(_ context.Context, secret []byte, cmd domain.Command) (domain.SudoResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.lastAuth = append([]byte(nil), secret...)
	f.lastCmd = cmd
	if f.execErr != nil {
		return domain.SudoResult{}, f.execErr
	}
	return domain.SudoResult{Stdout: "ok\n", ExitCode: 0}, nil
}

type fakeAudit struct {
	mu     sync.Mutex
	events []domain.AuditEvent
}

func (f *fakeAudit) Log(_ context.Context, e domain.AuditEvent) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.events = append(f.events, e)
	return nil
}
func (f *fakeAudit) Close() error { return nil }

type fakeCache struct {
	status domain.CacheStatus
	err    error
	remain time.Duration
}

func (f *fakeCache) IsActive(_ context.Context, _ domain.CacheConfig) (domain.CacheStatus, error) {
	return f.status, f.err
}
func (f *fakeCache) TimeRemaining(_ context.Context, _ domain.CacheConfig) (time.Duration, error) {
	return f.remain, nil
}
func (f *fakeCache) Refresh(_ context.Context, _ domain.CacheConfig) error { return nil }

// fakeEvaluator is a minimal PolicyEvaluator stub: it returns Allow for
// non-blocked commands and Block for anything matching a substring in
// `blocked`.
type fakeEvaluator struct {
	blocked []string
}

func (f *fakeEvaluator) Evaluate(_ context.Context, _ domain.Policy, cmd domain.Command) (domain.MatchResult, error) {
	dec := domain.DecisionAllow
	var reasons []string
	for _, b := range f.blocked {
		if strings.Contains(cmd.String(), b) {
			dec = domain.DecisionBlock
			reasons = append(reasons, "blocked: "+b)
		}
	}
	return domain.MatchResult{Decision: dec, Reasons: reasons, EvaluatedAt: time.Now()}, nil
}
func (f *fakeEvaluator) ListCategories(_ context.Context) ([]domain.CategoryEntry, error) {
	return nil, nil
}

// --- AuthUseCase --------------------------------------------------------

func TestAuthUseCase_OK(t *testing.T) {
	gw := &fakeGateway{}
	au := NewAuthUseCase(gw, &fakeAudit{})
	if err := au.Execute(context.Background(), AuthInput{Secret: []byte("hunter2")}); err != nil {
		t.Fatalf("Execute: %v", err)
	}
}

func TestAuthUseCase_FailsAndAudits(t *testing.T) {
	gw := &fakeGateway{authErr: domain.ErrAuthFailed}
	audit := &fakeAudit{}
	au := NewAuthUseCase(gw, audit)
	err := au.Execute(context.Background(), AuthInput{Secret: []byte("x")})
	if !errors.Is(err, domain.ErrAuthFailed) {
		t.Fatalf("err = %v", err)
	}
	if len(audit.events) != 1 || audit.events[0].Decision != domain.DecisionBlock {
		t.Fatalf("audit events = %+v", audit.events)
	}
}

func TestAuthUseCase_NilGateway(t *testing.T) {
	au := &AuthUseCase{Gateway: nil, Audit: nilAudit{}, Now: defaultNow}
	if err := au.Execute(context.Background(), AuthInput{}); err == nil {
		t.Fatal("expected error on nil gateway")
	}
}

// --- CheckUseCase -------------------------------------------------------

func TestCheckUseCase_CacheActive(t *testing.T) {
	repo := &fakeCache{status: domain.CacheActive, remain: 300 * time.Second}
	cu := NewCheckUseCase(repo)
	out, err := cu.Execute(context.Background(), CheckInput{Config: domain.DefaultConfig()})
	if err != nil {
		t.Fatal(err)
	}
	if out.Status != domain.CacheActive {
		t.Errorf("Status = %v, want CacheActive", out.Status)
	}
	if out.Remaining != 300*time.Second {
		t.Errorf("Remaining = %v, want 300s", out.Remaining)
	}
}

func TestCheckUseCase_DisabledCacheAlwaysExpired(t *testing.T) {
	repo := &fakeCache{status: domain.CacheActive, remain: 100 * time.Second}
	cu := NewCheckUseCase(repo)
	cfg := domain.DefaultConfig()
	cfg.Cache.Disabled = true
	out, err := cu.Execute(context.Background(), CheckInput{Config: cfg})
	if err != nil {
		t.Fatal(err)
	}
	if out.Status != domain.CacheExpired {
		t.Errorf("Status = %v, want CacheExpired", out.Status)
	}
}

func TestCheckUseCase_NilRepository(t *testing.T) {
	cu := &CheckUseCase{Repository: nil}
	if _, err := cu.Execute(context.Background(), CheckInput{}); err == nil {
		t.Fatal("expected error on nil repo")
	}
}

// --- ExecUseCase --------------------------------------------------------

func TestExecUseCase_PolicyAllow_RunsWithCache(t *testing.T) {
	gw := &fakeGateway{}
	ev := &fakeEvaluator{}
	repo := &fakeCache{status: domain.CacheActive}
	audit := &fakeAudit{}
	u := NewExecUseCase(gw, ev, repo, domain.DefaultPolicy(), audit)
	cmd, _ := domain.NewCommand("apt", []string{"update"})
	out, err := u.Execute(context.Background(), ExecInput{Command: cmd, CacheActive: true})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out.Decision.Decision != domain.DecisionAllow {
		t.Errorf("Decision = %v", out.Decision.Decision)
	}
	if !out.CacheUsed {
		t.Error("expected CacheUsed=true")
	}
	if gw.lastCmd.Path != "apt" {
		t.Errorf("lastCmd.Path = %q", gw.lastCmd.Path)
	}
}

func TestExecUseCase_PolicyBlock_StopsCommand(t *testing.T) {
	gw := &fakeGateway{}
	ev := &fakeEvaluator{blocked: []string{"ssh"}}
	repo := &fakeCache{status: domain.CacheActive}
	audit := &fakeAudit{}
	u := NewExecUseCase(gw, ev, repo, domain.DefaultPolicy(), audit)
	cmd, _ := domain.NewCommand("ssh", []string{"user@host"})
	_, err := u.Execute(context.Background(), ExecInput{Command: cmd, CacheActive: true})
	var pve *domain.PolicyViolationError
	if !errors.As(err, &pve) {
		t.Fatalf("err = %v, want PolicyViolationError", err)
	}
	if gw.lastCmd.Path != "" {
		t.Errorf("blocked command should not reach gateway; got %q", gw.lastCmd.Path)
	}
	if len(audit.events) != 1 || audit.events[0].Decision != domain.DecisionBlock {
		t.Errorf("audit events = %+v", audit.events)
	}
}

func TestExecUseCase_PolicyBlock_WithOverride_RunsAndAudits(t *testing.T) {
	gw := &fakeGateway{}
	ev := &fakeEvaluator{blocked: []string{"ssh"}}
	repo := &fakeCache{status: domain.CacheActive}
	audit := &fakeAudit{}
	u := NewExecUseCase(gw, ev, repo, domain.DefaultPolicy(), audit)
	cmd, _ := domain.NewCommand("ssh", []string{"user@host"})
	out, err := u.Execute(context.Background(), ExecInput{Command: cmd, CacheActive: true, Override: true})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !out.Overrode {
		t.Error("Overrode should be true")
	}
	if len(audit.events) != 1 {
		t.Fatalf("expected 1 audit event, got %d", len(audit.events))
	}
	if audit.events[0].OverrideBy == "" {
		t.Errorf("OverrideBy should be set, got %q", audit.events[0].OverrideBy)
	}
}

func TestExecUseCase_CacheMiss_FallsBackToSecret(t *testing.T) {
	gw := &fakeGateway{}
	ev := &fakeEvaluator{}
	repo := &fakeCache{status: domain.CacheExpired}
	audit := &fakeAudit{}
	u := NewExecUseCase(gw, ev, repo, domain.DefaultPolicy(), audit)
	cmd, _ := domain.NewCommand("apt", []string{"update"})
	_, err := u.Execute(context.Background(), ExecInput{Command: cmd, Secret: []byte("p")})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if string(gw.lastAuth) != "p" {
		t.Errorf("secret not consumed; got %q", gw.lastAuth)
	}
}

func TestExecUseCase_CacheMissNoSecret_ReturnsCacheMiss(t *testing.T) {
	gw := &fakeGateway{}
	ev := &fakeEvaluator{}
	repo := &fakeCache{status: domain.CacheExpired}
	u := NewExecUseCase(gw, ev, repo, domain.DefaultPolicy(), &fakeAudit{})
	cmd, _ := domain.NewCommand("apt", []string{"update"})
	_, err := u.Execute(context.Background(), ExecInput{Command: cmd})
	if !errors.Is(err, domain.ErrCacheMiss) {
		t.Fatalf("err = %v, want ErrCacheMiss", err)
	}
}

func TestExecUseCase_NilGateway(t *testing.T) {
	u := &ExecUseCase{Audit: nilAudit{}}
	if _, err := u.Execute(context.Background(), ExecInput{}); err == nil {
		t.Fatal("expected error on nil gateway")
	}
}

func TestExecUseCase_AuditsSuccessfulCommand(t *testing.T) {
	gw := &fakeGateway{}
	ev := &fakeEvaluator{}
	repo := &fakeCache{status: domain.CacheActive}
	audit := &fakeAudit{}
	u := NewExecUseCase(gw, ev, repo, domain.DefaultPolicy(), audit)
	cmd, _ := domain.NewCommand("apt", []string{"update"})
	if _, err := u.Execute(context.Background(), ExecInput{Command: cmd, CacheActive: true}); err != nil {
		t.Fatal(err)
	}
	if len(audit.events) != 1 || audit.events[0].Decision != domain.DecisionAllow {
		t.Errorf("audit events = %+v", audit.events)
	}
}
