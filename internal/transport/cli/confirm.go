package cli

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"github.com/fatih/color"
)

// confirmPrompt writes prompt to w and reads a single y/N answer from r.
//
// Returns:
//   - (true, nil) when the user typed "y" or "yes" (case-insensitive).
//   - (false, nil) on EOF (treated as "no") or any non-y input.
//   - (false, err) on a genuine read failure (network error, etc).
//
// When w is nil the prompt is silently dropped (useful for tests that
// don't care about the rendered text).
func confirmPrompt(w io.Writer, r io.Reader, prompt string) (bool, error) {
	if w != nil {
		if _, err := io.WriteString(w, prompt); err != nil {
			return false, err
		}
	}
	scanner := bufio.NewScanner(r)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return false, err
		}
		return false, nil
	}
	ans := strings.TrimSpace(scanner.Text())
	return strings.EqualFold(ans, "y") || strings.EqualFold(ans, "yes"), nil
}

// renderOverridePrompt formats the interactive confirmation shown by
// `sudoconsole exec --policy-override <reason>`.
func renderOverridePrompt(command, reason string) string {
	bold := color.New(color.FgRed, color.Bold)
	return fmt.Sprintf("\n%s Override policy block for: %s\n  reason: %s\n  Proceed? [y/N]: ",
		bold.Sprint("[override]"), command, reason)
}
