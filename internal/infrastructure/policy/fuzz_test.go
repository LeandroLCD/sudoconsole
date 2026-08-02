package policy

import (
	"testing"
)

// FuzzMatcher exercises Compile + Match across random pattern +
// input combinations. The matcher should:
//
//   - never panic;
//   - return without error from Compile on patterns that are
//     either valid glob or valid regex (with optional ReDoS
//     rejection);
//   - return a non-empty match (or no match) from Match without
//     hanging or exploding memory.
//
// Patterns starting with "re:" are treated as regex and routed
// through the ReDoS heuristic. Anything else is treated as a glob.
func FuzzMatcher(f *testing.F) {
	seeds := []struct{ pattern, input string }{
		{"*", "anything"},
		{"*.go", "main.go"},
		{"re:ssh.*-R", "ssh user@host -R 8080"},
		{"re:[", "any"},
		{"", "x"},
		{"   ", "x"},
		{"re:", "x"},
		{"re:.*", "abc"},
		{"?", ""},
	}
	for _, s := range seeds {
		f.Add(s.pattern, s.input)
	}
	f.Fuzz(func(t *testing.T, pattern, input string) {
		// Bound pattern length so the fuzzer can't OOM us with a
		// 1MB regex string.
		if len(pattern) > 256 {
			return
		}
		if len(input) > 4096 {
			return
		}
		_, err := Compile([]string{pattern})
		if err != nil {
			return // invalid patterns are expected; the matcher must
			// just refuse to compile, not panic.
		}
		m, err := Compile([]string{pattern})
		if err != nil {
			t.Fatalf("Compile succeeded, then failed on identical input: %v", err)
		}
		// Match must never panic or hang. We don't assert anything
		// about the result because every input is valid by
		// construction.
		_ = m.Match(input)
		_ = m.MatchAll(input)
	})
}

// FuzzValidateEach exercises the per-pattern validator. The
// validator must:
//
//   - never panic;
//   - return []PatternError without reading uninitialised memory
//     or crashing the regex engine;
//   - classify "re:(a+)+" as ReDoS (the canonical attack vector).
func FuzzValidateEach(f *testing.F) {
	f.Add("re:(a+)+")
	f.Add("re:.*")
	f.Add("[bad")
	f.Add("ok-pattern")
	f.Add("")
	f.Add("   ")
	f.Add("re:")
	f.Add("re:[")
	f.Fuzz(func(t *testing.T, pattern string) {
		if len(pattern) > 256 {
			return
		}
		// The validator must never panic, regardless of input.
		_ = ValidateEach([]string{pattern})
	})
}
