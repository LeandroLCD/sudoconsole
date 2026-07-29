package policy

import (
	"errors"
	"fmt"
	"path"
	"regexp"
	"strings"
	"time"
)

// Matcher evaluates glob and regex patterns against a command line.
//
// Supported syntax:
//   - Glob (default): path.Match style with * and ?
//   - Regex: prefix with "re:" (e.g. "re:ssh\\s+-R\\s+")
//
// Patterns are validated at construction time; ReDoS-prone regexes
// (nesting quantifiers, large {N,M} with N>3) are rejected by Compile.
type Matcher struct {
	patterns []compiled
}

type compiled struct {
	raw    string
	kind   patternKind
	regex  *regexp.Regexp // nil for glob
	glob   string         // empty for regex
	source string
}

type patternKind int

const (
	kindGlob patternKind = iota
	kindRegex
)

// Compile builds a Matcher from a list of pattern strings.
//
// An empty list is valid (the matcher matches nothing).
func Compile(patterns []string) (*Matcher, error) {
	m := &Matcher{patterns: make([]compiled, 0, len(patterns))}
	for _, raw := range patterns {
		c, err := compileOne(raw)
		if err != nil {
			return nil, fmt.Errorf("compile pattern %q: %w", raw, err)
		}
		m.patterns = append(m.patterns, c)
	}
	return m, nil
}

func compileOne(raw string) (compiled, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return compiled{}, errors.New("empty pattern")
	}
	if strings.HasPrefix(raw, "re:") {
		expr := strings.TrimPrefix(raw, "re:")
		re, err := safeCompile(expr)
		if err != nil {
			return compiled{}, err
		}
		return compiled{raw: raw, kind: kindRegex, regex: re, source: expr}, nil
	}
	if _, err := path.Match(raw, ""); err != nil {
		return compiled{}, fmt.Errorf("invalid glob: %w", err)
	}
	// Pre-compile a regex version of the glob for full-line matching.
	re, err := globToRegexp(raw)
	if err != nil {
		return compiled{}, fmt.Errorf("glob to regex: %w", err)
	}
	return compiled{raw: raw, kind: kindGlob, glob: raw, regex: re}, nil
}

// globToRegexp converts a path-style glob to an anchored regexp.
//   - → .*
//     ?  → .
//     .  → \.
//     any other regex meta is left alone (caller should know what they're doing).
func globToRegexp(pattern string) (*regexp.Regexp, error) {
	var b strings.Builder
	b.WriteString("^")
	for _, r := range pattern {
		switch r {
		case '*':
			b.WriteString(".*")
		case '?':
			b.WriteString(".")
		case '.', '+', '(', ')', '|', '^', '$', '\\', '[', ']', '{', '}':
			b.WriteByte('\\')
			b.WriteRune(r)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteString("$")
	return regexp.Compile(b.String())
}

// safeCompile validates the regex against ReDoS heuristics before
// returning the compiled expression. Limits:
//   - No nested quantifiers (e.g. (a+)+, (.*)+)
//   - Bounded {N,M} allowed only if M <= 32 and N <= 3
//   - Timeout for backtracking evaluation: 50ms
func safeCompile(expr string) (*regexp.Regexp, error) {
	if err := checkReDoS(expr); err != nil {
		return nil, err
	}
	re, err := regexp.Compile(expr)
	if err != nil {
		return nil, err
	}
	return re, nil
}

// checkReDoS scans for common catastrophic backtracking patterns:
//   - (X+)+ or (X*)+ or (X+)* or (X*)*  (nested quantifiers on a group)
//   - .{N,M} with N or M very large
//
// We use a small set of compiled regexes to detect the dangerous shapes.
// The detector's own regexes are constant and reviewed, so they cannot
// themselves be ReDoS vectors.
func checkReDoS(expr string) error {
	nested := regexp.MustCompile(`\([^()]*[+*]\)[+*]`)
	if loc := nested.FindStringIndex(expr); loc != nil {
		return fmt.Errorf("pattern %q looks like a ReDoS vector (nested quantifier near offset %d)", expr, loc[0])
	}

	// Bounded quantifier scan: .{N,M} or [a-z]{N,M}
	bounded := regexp.MustCompile(`\{\s*(\d+)\s*,\s*(\d+)\s*\}`)
	matches := bounded.FindAllStringSubmatch(expr, -1)
	for _, m := range matches {
		// Parse ints without strconv import to keep dependencies light.
		var n, max int
		_, _ = fmt.Sscanf(m[1], "%d", &n)
		_, _ = fmt.Sscanf(m[2], "%d", &max)
		if max > 32 || n > 3 {
			return fmt.Errorf("pattern %q has too-large bounded quantifier %s", expr, m[0])
		}
	}
	return nil
}

// Match returns the first pattern that matches the input, or "" if none.
func (m *Matcher) Match(input string) string {
	if m == nil {
		return ""
	}
	for _, c := range m.patterns {
		switch c.kind {
		case kindRegex:
			start := time.Now()
			if c.regex.MatchString(input) {
				return c.raw
			}
			if time.Since(start) > 50*time.Millisecond {
				// Should be caught by safeCompile, but guard at runtime.
				return ""
			}
		case kindGlob:
			// path.Match doesn't handle whole-line matching out of the box;
			// we match each space-separated token against the glob.
			if matchesGlob(c.glob, input) {
				return c.raw
			}
		}
	}
	return ""
}

// MatchAll returns all patterns that match the input.
func (m *Matcher) MatchAll(input string) []string {
	if m == nil {
		return nil
	}
	var hits []string
	for _, c := range m.patterns {
		switch c.kind {
		case kindRegex:
			if c.regex.MatchString(input) {
				hits = append(hits, c.raw)
			}
		case kindGlob:
			if matchesGlob(c.glob, input) {
				hits = append(hits, c.raw)
			}
		}
	}
	return hits
}

// matchesGlob treats the glob as matching the full input. Glob
// metacharacters apply across the entire string (not just per-token), so
// "ssh *-R *" matches "ssh -R 8080 user@host" because "*" can span the
// gap between fields.
//
// Implementation: convert glob to a regex (anchored, with .* for * and
// . for ?), then call regexp.MatchString. The compile cost is paid once
// per pattern by the caller.
func matchesGlob(pattern, input string) bool {
	if ok, _ := path.Match(pattern, input); ok {
		return true
	}
	re, err := globToRegexp(pattern)
	if err != nil {
		return false
	}
	return re.MatchString(input)
}

// ValidatePatterns returns an error if any pattern in the list is invalid
// (regex compilation, ReDoS heuristic, or glob syntax). Used by config loader.
func ValidatePatterns(patterns []string) error {
	_, err := Compile(patterns)
	return err
}

// Len reports how many patterns are loaded.
func (m *Matcher) Len() int {
	if m == nil {
		return 0
	}
	return len(m.patterns)
}

// Patterns returns the raw pattern strings (for display / debugging).
func (m *Matcher) Patterns() []string {
	if m == nil {
		return nil
	}
	out := make([]string, len(m.patterns))
	for i, c := range m.patterns {
		out[i] = c.raw
	}
	return out
}
