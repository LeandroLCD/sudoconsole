package cli

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/LeandroLCD/sudoconsole/internal/domain"
)

// stubListCategoriesEvaluator implements domain.PolicyEvaluator with
// both Evaluate (blocklist-by-string) and a fixed category table so
// policy list / test / show can be exercised end-to-end without the
// real evaluator.
type stubListCategoriesEvaluator struct {
	blocked []string
	cats    []domain.CategoryEntry
}

func (s *stubListCategoriesEvaluator) Evaluate(_ context.Context, _ domain.Policy, cmd domain.Command) (domain.MatchResult, error) {
	dec := domain.DecisionAllow
	var reasons []string
	for _, b := range s.blocked {
		if strings.Contains(cmd.String(), b) {
			dec = domain.DecisionBlock
			reasons = append(reasons, "blocked: "+b)
		}
	}
	return domain.MatchResult{Decision: dec, Reasons: reasons, EvaluatedAt: time.Now()}, nil
}
func (s *stubListCategoriesEvaluator) ListCategories(_ context.Context) ([]domain.CategoryEntry, error) {
	out := make([]domain.CategoryEntry, len(s.cats))
	copy(out, s.cats)
	return out, nil
}

func TestPolicyList_Human(t *testing.T) {
	app := stubApp(t)
	app.Evaluator = &stubListCategoriesEvaluator{
		cats: []domain.CategoryEntry{
			{Binary: "ssh", Category: domain.CategoryRemoteAccess, Risk: domain.RiskCritical, Notes: "remote shell", BuiltIn: true},
			{Binary: "apt", Category: domain.CategoryPackageManager, Risk: domain.RiskMedium, Notes: "package manager", BuiltIn: true},
		},
	}
	stdout, _, err := cmdFromArgs(app, []string{"policy", "list"})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	for _, want := range []string{"ssh", "apt", "remote_access", "package_manager", "builtin"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("output missing %q; got %q", want, stdout)
		}
	}
}

func TestPolicyList_JSON(t *testing.T) {
	app := stubApp(t)
	fmtImpl, _ := NewFormatter(FormatJSON, false)
	app.Formatter = fmtImpl
	app.Evaluator = &stubListCategoriesEvaluator{
		cats: []domain.CategoryEntry{
			{Binary: "ssh", Category: domain.CategoryRemoteAccess, Risk: domain.RiskCritical, BuiltIn: true},
		},
	}
	stdout, _, err := cmdFromArgs(app, []string{"policy", "list"})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	var got PolicyListResult
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("json: %v body=%q", err, stdout)
	}
	if len(got.Categories) != 1 || got.Categories[0].Binary != "ssh" {
		t.Errorf("unexpected categories: %+v", got.Categories)
	}
}

func TestPolicyTest_BlocksSsh(t *testing.T) {
	app := stubApp(t)
	fmtImpl, _ := NewFormatter(FormatJSON, false)
	app.Formatter = fmtImpl
	app.Evaluator = &stubListCategoriesEvaluator{blocked: []string{"ssh"}}

	stdout, _, err := cmdFromArgs(app, []string{"policy", "test", "ssh user@host"})
	if err == nil {
		t.Fatal("expected non-nil error for blocked command (exit code = 64)")
	}
	var got PolicyTestResult
	if jerr := json.Unmarshal([]byte(stdout), &got); jerr != nil {
		t.Fatalf("json: %v body=%q", jerr, stdout)
	}
	if got.Decision != "block" {
		t.Errorf("expected decision=block; got %q", got.Decision)
	}
	if !strings.Contains(got.Reasons[0], "blocked: ssh") {
		t.Errorf("expected reason to mention ssh; got %v", got.Reasons)
	}
}

func TestPolicyTest_AllowsApt(t *testing.T) {
	app := stubApp(t)
	fmtImpl, _ := NewFormatter(FormatJSON, false)
	app.Formatter = fmtImpl
	app.Evaluator = &stubListCategoriesEvaluator{blocked: []string{"ssh"}}

	stdout, _, err := cmdFromArgs(app, []string{"policy", "test", "apt", "update"})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	var got PolicyTestResult
	if jerr := json.Unmarshal([]byte(stdout), &got); jerr != nil {
		t.Fatalf("json: %v body=%q", jerr, stdout)
	}
	if got.Decision != "allow" {
		t.Errorf("expected decision=allow; got %q", got.Decision)
	}
}

func TestPolicyShow_Human(t *testing.T) {
	app := stubApp(t)
	stdout, _, err := cmdFromArgs(app, []string{"policy", "show"})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	for _, want := range []string{"mode", "blocked.categories", "remote_access", "credential_exposure"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("output missing %q; got %q", want, stdout)
		}
	}
}

func TestPolicyShow_JSON(t *testing.T) {
	app := stubApp(t)
	fmtImpl, _ := NewFormatter(FormatJSON, false)
	app.Formatter = fmtImpl
	stdout, _, err := cmdFromArgs(app, []string{"policy", "show"})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	var got PolicyShowResult
	if jerr := json.Unmarshal([]byte(stdout), &got); jerr != nil {
		t.Fatalf("json: %v body=%q", jerr, stdout)
	}
	if got.Mode != "blocklist" {
		t.Errorf("expected mode=blocklist (default); got %q", got.Mode)
	}
	if len(got.BlockedCategories) == 0 {
		t.Errorf("expected default blocked categories; got %+v", got)
	}
}

func TestPolicyValidate_DefaultClean(t *testing.T) {
	app := stubApp(t)
	fmtImpl, _ := NewFormatter(FormatHuman, false)
	app.Formatter = fmtImpl
	stdout, _, err := cmdFromArgs(app, []string{"policy", "validate"})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !strings.Contains(stdout, "all patterns compile cleanly") {
		t.Errorf("expected success message; got %q", stdout)
	}
}

func TestPolicyValidate_RejectsReDoSAndBadGlob(t *testing.T) {
	app := stubApp(t)
	dir := t.TempDir()
	// Write a config with bad patterns so validate has work to do.
	cfgPath := dir + "/bad.toml"
	toml := `
[policy]
extra_patterns = ["re:(a+)+", "[bad", "ok-pattern", "  "]

[policy.blocked]
patterns = ["re:ssh\\s+-R\\s+"]
`
	if err := writeFile(cfgPath, toml); err != nil {
		t.Fatal(err)
	}
	app.Config.Policy.ExtraPatterns = []string{"re:(a+)+", "[bad", "ok-pattern", "  "}
	app.Config.Policy.Blocked.Patterns = []string{`re:ssh\s+-R\s+`}

	fmtImpl, _ := NewFormatter(FormatJSON, false)
	app.Formatter = fmtImpl
	stdout, _, err := cmdFromArgs(app, []string{"policy", "validate"})
	if err == nil {
		t.Fatal("expected non-nil exit on invalid patterns")
	}
	var got PolicyValidateResult
	if jerr := json.Unmarshal([]byte(stdout), &got); jerr != nil {
		t.Fatalf("json: %v body=%q", jerr, stdout)
	}
	if got.Valid {
		t.Errorf("expected Valid=false; got %+v", got)
	}
	if len(got.Extras) < 3 {
		t.Errorf("expected >=3 extras errors; got %d (%+v)", len(got.Extras), got.Extras)
	}
	// First error should be the ReDoS vector.
	if !strings.Contains(got.Extras[0].Pattern, "(a+)+") {
		t.Errorf("expected ReDoS pattern in first extra error; got %+v", got.Extras[0])
	}
}

// writeFile is a tiny helper used by the validate-fixture test.
func writeFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0o600)
}

// silence unused import when build flags strip the helper.
var _ = context.Background
