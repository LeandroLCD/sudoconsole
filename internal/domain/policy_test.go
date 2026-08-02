package domain

import (
	"errors"
	"testing"
	"time"
)

func TestDecision_String(t *testing.T) {
	for d, want := range map[Decision]string{
		DecisionAllow: "allow",
		DecisionWarn:  "warn",
		DecisionAudit: "audit",
		DecisionBlock: "block",
		Decision(99):  "unknown",
	} {
		if got := d.String(); got != want {
			t.Errorf("%d.String() = %q, want %q", d, got, want)
		}
	}
}

func TestDecision_ExitCode(t *testing.T) {
	cases := map[Decision]int{
		DecisionAllow: 0,
		DecisionAudit: 0,
		DecisionWarn:  65,
		DecisionBlock: 64,
		Decision(99):  2,
	}
	for d, want := range cases {
		if got := d.ExitCode(); got != want {
			t.Errorf("%d.ExitCode() = %d, want %d", d, got, want)
		}
	}
}

func TestRisk_String(t *testing.T) {
	for r, want := range map[Risk]string{
		RiskLow:      "low",
		RiskMedium:   "medium",
		RiskHigh:     "high",
		RiskCritical: "critical",
		Risk(99):     "unknown",
	} {
		if got := r.String(); got != want {
			t.Errorf("%d.String() = %q, want %q", r, got, want)
		}
	}
}

func TestPolicyMode_String(t *testing.T) {
	for m, want := range map[PolicyMode]string{
		PolicyModeBlocklist: "blocklist",
		PolicyModeAllowlist: "allowlist",
		PolicyModeAudit:     "audit",
		PolicyMode(99):      "unknown",
	} {
		if got := m.String(); got != want {
			t.Errorf("%d.String() = %q, want %q", m, got, want)
		}
	}
}

func TestDefaultPolicy(t *testing.T) {
	p := DefaultPolicy()
	if p.Mode != PolicyModeBlocklist {
		t.Fatalf("default mode = %v, want blocklist", p.Mode)
	}
	if len(p.Blocked.Categories) == 0 {
		t.Fatal("default should block some categories")
	}
	if !p.RemoteAccess.BlockReverseTunnels {
		t.Fatal("default should block reverse tunnels")
	}
	if !p.CredentialExposure.BlockPasswdChange {
		t.Fatal("default should block passwd change")
	}
}

func TestMatchResult_Reason(t *testing.T) {
	r := MatchResult{}
	if r.Reason() != "no reason" {
		t.Fatalf("empty reason = %q", r.Reason())
	}
	r.Reasons = []string{"a", "b"}
	if got := r.Reason(); got != "a; b" {
		t.Fatalf("Reason = %q", got)
	}
}

func TestMatchResult_IsTerminal(t *testing.T) {
	for _, d := range []Decision{DecisionAllow, DecisionBlock} {
		if !(MatchResult{Decision: d}).IsTerminal() {
			t.Errorf("%v should be terminal", d)
		}
	}
	for _, d := range []Decision{DecisionWarn, DecisionAudit, DecisionUnknown} {
		if (MatchResult{Decision: d}).IsTerminal() {
			t.Errorf("%v should NOT be terminal", d)
		}
	}
}

func TestCategory_KnownValues(t *testing.T) {
	// Ensure all named categories compile and are distinct.
	cats := []Category{
		CategoryPackageManager,
		CategoryServiceControl,
		CategoryFilesystem,
		CategoryNetworkConfig,
		CategoryUserManagement,
		CategoryRemoteAccess,
		CategoryCredentialExposure,
		CategoryShellSpawn,
		CategoryPersistence,
		CategoryUnknown,
	}
	seen := map[Category]bool{}
	for _, c := range cats {
		if seen[c] {
			t.Fatalf("duplicate category: %q", c)
		}
		seen[c] = true
	}
}

func TestPolicy_ZeroValueIsSafe(t *testing.T) {
	var p Policy
	// Should not panic; Mode is PolicyModeUnknown which is the zero value.
	if p.Mode.String() != "unknown" {
		t.Fatal("zero Policy mode should be unknown")
	}
}

// Compile-time checks for the time import (so the file stays self-contained
// in case we drop the time ref later).
var _ = errors.New
var _ = time.Second
