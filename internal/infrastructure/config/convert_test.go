package config

import "testing"

// covers parsePolicyMode and parseAgentKind indirectly through the
// schema validation pipeline.
func TestSchema_ValidModesAndKinds(t *testing.T) {
	for _, m := range []string{"blocklist", "allowlist", "audit"} {
		mode, ok := parsePolicyMode(m)
		if !ok {
			t.Errorf("parsePolicyMode(%q) not ok", m)
		}
		if mode.String() != m {
			t.Errorf("parsePolicyMode(%q) roundtrip = %q", m, mode.String())
		}
	}
	if _, ok := parsePolicyMode("nope"); ok {
		t.Error("unknown mode should fail")
	}
	for _, k := range []string{"kilo", "claude", "gemini", "aider", "codex", "copilot", "generic"} {
		ak, ok := parseAgentKind(k)
		if !ok {
			t.Errorf("parseAgentKind(%q) not ok", k)
		}
		if ak.String() != k {
			t.Errorf("parseAgentKind(%q) roundtrip = %q", k, ak.String())
		}
	}
	if _, ok := parseAgentKind("nope"); ok {
		t.Error("unknown agent should fail")
	}
}
