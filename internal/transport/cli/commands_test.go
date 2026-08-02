package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/LeandroLCD/sudoconsole/internal/domain"
	"github.com/LeandroLCD/sudoconsole/internal/infrastructure/audit"
)

// --- formatters ---------------------------------------------------------

func TestParseFormat(t *testing.T) {
	for _, c := range []struct {
		in   string
		want Format
	}{
		{"", FormatHuman},
		{"human", FormatHuman},
		{"json", FormatJSON},
	} {
		got, err := ParseFormat(c.in)
		if err != nil {
			t.Errorf("ParseFormat(%q) error: %v", c.in, err)
		}
		if got != c.want {
			t.Errorf("ParseFormat(%q) = %q, want %q", c.in, got, c.want)
		}
	}
	if _, err := ParseFormat("yaml"); err == nil {
		t.Error("yaml should fail")
	}
}

func TestHumanFormatter_AuthResult(t *testing.T) {
	f, _ := NewFormatter(FormatHuman, false)
	var buf bytes.Buffer
	if err := f.Print(&buf, &AuthResult{OK: true, Time: "now"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "authentication cached") {
		t.Errorf("unexpected output: %q", buf.String())
	}
}

func TestHumanFormatter_CheckResult(t *testing.T) {
	f, _ := NewFormatter(FormatHuman, false)
	var buf bytes.Buffer
	if err := f.Print(&buf, &CheckResult{Active: true, RemainingSeconds: 100, Time: "now"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "active") {
		t.Errorf("unexpected output: %q", buf.String())
	}
}

func TestHumanFormatter_ExecResult_Block(t *testing.T) {
	f, _ := NewFormatter(FormatHuman, false)
	var buf bytes.Buffer
	if err := f.Print(&buf, &ExecResult{Decision: "block", Reason: "ssh", ExitCode: 64, Time: "now"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "blocked by policy") {
		t.Errorf("unexpected output: %q", buf.String())
	}
}

func TestJSONFormatter_RoundTrip(t *testing.T) {
	f, _ := NewFormatter(FormatJSON, false)
	var buf bytes.Buffer
	if err := f.Print(&buf, &CheckResult{Active: true, RemainingSeconds: 12.5, Time: "now"}); err != nil {
		t.Fatal(err)
	}
	var got CheckResult
	if err := json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &got); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if got.Active != true || got.RemainingSeconds != 12.5 {
		t.Errorf("roundtrip mismatch: %+v", got)
	}
}

func TestHumanFormatter_UnsupportedValue(t *testing.T) {
	f, _ := NewFormatter(FormatHuman, false)
	var buf bytes.Buffer
	if err := f.Print(&buf, "garbage"); err == nil {
		t.Fatal("expected error for unsupported type")
	}
}

// --- logger -------------------------------------------------------------

func TestParseLogLevel(t *testing.T) {
	cases := map[string]LogLevel{
		"":       LogInfo,
		"silent": LogSilent,
		"error":  LogError,
		"warn":   LogWarn,
		"info":   LogInfo,
		"debug":  LogDebug,
	}
	for in, want := range cases {
		got, err := ParseLogLevel(in)
		if err != nil {
			t.Errorf("ParseLogLevel(%q) err = %v", in, err)
			continue
		}
		if got != want {
			t.Errorf("ParseLogLevel(%q) = %v, want %v", in, got, want)
		}
	}
	if _, err := ParseLogLevel("trace"); err == nil {
		t.Error("trace should fail")
	}
}

func TestNewLogger_SilentProducesNothing(t *testing.T) {
	var buf bytes.Buffer
	l := NewLogger(&buf, LogSilent)
	l.Info("hello", "key", "val")
	if buf.Len() != 0 {
		t.Errorf("silent logger wrote %q", buf.String())
	}
}

func TestNewLogger_InfoEmits(t *testing.T) {
	var buf bytes.Buffer
	l := NewLogger(&buf, LogInfo)
	l.Info("hello")
	if !strings.Contains(buf.String(), "hello") {
		t.Errorf("expected log line; got %q", buf.String())
	}
}

// --- ExitCode -----------------------------------------------------------

func TestExitCode(t *testing.T) {
	if ExitCode(nil) != 0 {
		t.Errorf("nil err: got %d", ExitCode(nil))
	}
	if ExitCode(domain.ErrAuthFailed) != 2 {
		t.Errorf("ErrAuthFailed: got %d", ExitCode(domain.ErrAuthFailed))
	}
	if ExitCode(domain.ErrCacheMiss) != 1 {
		t.Errorf("ErrCacheMiss: got %d", ExitCode(domain.ErrCacheMiss))
	}
	pve := &domain.PolicyViolationError{Result: domain.MatchResult{Decision: domain.DecisionBlock}}
	if ExitCode(pve) != 64 {
		t.Errorf("block: got %d", ExitCode(pve))
	}
	pve.Result.Decision = domain.DecisionWarn
	if ExitCode(pve) != 65 {
		t.Errorf("warn: got %d", ExitCode(pve))
	}
}

// --- CLI command smoke tests ---------------------------------------------

// cmdFromArgs builds a fresh cobra command from the App and runs it with
// the provided args. stdout/stderr are captured.
func cmdFromArgs(app *App, args []string) (stdout, stderr string, err error) {
	cmd := NewRootCmd(app, "test", "abc", "now")
	var out, errBuf bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errBuf)
	cmd.SetArgs(args)
	err = cmd.Execute()
	return out.String(), errBuf.String(), err
}

func TestCheckCmd_PrintsActiveOrExpired(t *testing.T) {
	app := stubApp(t)
	app.Repository = &fakeRepo{status: domain.CacheActive, remain: 200 * time.Second}
	stdout, _, err := cmdFromArgs(app, []string{"check"})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !strings.Contains(stdout, "active") {
		t.Errorf("expected active message; got %q", stdout)
	}
}

func TestCheckCmd_JSONOutput(t *testing.T) {
	app := stubApp(t)
	app.Repository = &fakeRepo{status: domain.CacheActive, remain: 200 * time.Second}
	fmtImpl, _ := NewFormatter(FormatJSON, false)
	app.Formatter = fmtImpl
	stdout, _, err := cmdFromArgs(app, []string{"check"})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	var got CheckResult
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("json: %v body=%q", err, stdout)
	}
	if !got.Active {
		t.Errorf("expected active=true; got %+v", got)
	}
}

func TestExecCmd_PolicyBlockExit64(t *testing.T) {
	app := stubApp(t)
	app.Evaluator = &fakeEvaluator{blocked: []string{"ssh"}}
	app.Gateway = &fakeGateway{}
	app.Repository = &fakeRepo{status: domain.CacheActive}
	app.Audit = audit.NoopLogger{}
	stdout, _, err := cmdFromArgs(app, []string{"exec", "ssh", "user@host"})
	if err == nil {
		t.Fatal("expected error")
	}
	if ExitCode(err) != 64 {
		t.Errorf("ExitCode = %d, want 64", ExitCode(err))
	}
	if !strings.Contains(stdout, "blocked by policy") {
		t.Errorf("stdout should mention block; got %q", stdout)
	}
}

func TestExecCmd_PolicyOverrideRunsCommand(t *testing.T) {
	app := stubApp(t)
	app.Evaluator = &fakeEvaluator{blocked: []string{"ssh"}}
	gw := &fakeGateway{}
	app.Gateway = gw
	app.Repository = &fakeRepo{status: domain.CacheActive}
	app.Audit = audit.NoopLogger{}
	_, _, err := cmdFromArgs(app, []string{"exec", "--policy-override", "debugging", "--yes", "ssh", "user@host"})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if gw.lastCmd.Path != "ssh" {
		t.Errorf("expected override to reach gateway; got %+v", gw.lastCmd)
	}
}

func TestAuthCmd_NoTTYRequiresEnv(t *testing.T) {
	app := stubApp(t)
	app.Gateway = &fakeGateway{}
	_, _, err := cmdFromArgs(app, []string{"auth", "--no-tty"})
	if err == nil {
		t.Fatal("expected error when --no-tty without $SUDOCONSOLE_PASSWORD")
	}
}

func TestAuthCmd_NoTTYAuthenticates(t *testing.T) {
	app := stubApp(t)
	gw := &fakeGateway{}
	app.Gateway = gw
	t.Setenv("SUDOCONSOLE_PASSWORD", "s3cr3t")
	stdout, _, err := cmdFromArgs(app, []string{"auth", "--no-tty"})
	if err != nil {
		t.Fatalf("err: %v; stdout=%q", err, stdout)
	}
	if string(gw.lastAuth) != "s3cr3t" {
		t.Errorf("secret not consumed; got %q", gw.lastAuth)
	}
	if !strings.Contains(stdout, "authentication cached") {
		t.Errorf("stdout should mention auth ok; got %q", stdout)
	}
}

func TestRootCmd_UnknownSubcommand(t *testing.T) {
	app := stubApp(t)
	_, stderr, err := cmdFromArgs(app, []string{"bogus"})
	if err == nil {
		t.Fatalf("expected error; stderr=%q", stderr)
	}
	if !strings.Contains(err.Error(), "unknown command") {
		t.Errorf("err = %v", err)
	}
}

// --- fakes used by these tests (duplicated to avoid touching the policy file) ---

type fakeRepo struct {
	status domain.CacheStatus
	err    error
	remain time.Duration
}

func (f *fakeRepo) IsActive(_ context.Context, _ domain.CacheConfig) (domain.CacheStatus, error) {
	return f.status, f.err
}
func (f *fakeRepo) TimeRemaining(_ context.Context, _ domain.CacheConfig) (time.Duration, error) {
	return f.remain, nil
}
func (f *fakeRepo) Refresh(_ context.Context, _ domain.CacheConfig) error { return nil }

type fakeGateway struct {
	authErr  error
	execErr  error
	lastAuth []byte
	lastCmd  domain.Command
}

func (f *fakeGateway) Authenticate(_ context.Context, secret []byte) error {
	f.lastAuth = append([]byte(nil), secret...)
	return f.authErr
}
func (f *fakeGateway) Execute(_ context.Context, cmd domain.Command) (domain.SudoResult, error) {
	f.lastCmd = cmd
	if f.execErr != nil {
		return domain.SudoResult{}, f.execErr
	}
	return domain.SudoResult{Stdout: "ok\n", ExitCode: 0}, nil
}
func (f *fakeGateway) ExecuteWithAuth(_ context.Context, secret []byte, cmd domain.Command) (domain.SudoResult, error) {
	f.lastAuth = append([]byte(nil), secret...)
	f.lastCmd = cmd
	if f.execErr != nil {
		return domain.SudoResult{}, f.execErr
	}
	return domain.SudoResult{Stdout: "ok\n", ExitCode: 0}, nil
}

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

// silence unused import warnings
var _ = errors.New
var _ = context.Background
var _ cobra.Command
