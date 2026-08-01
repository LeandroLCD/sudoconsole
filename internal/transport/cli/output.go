package cli

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/fatih/color"

	"github.com/LeandroLCD/sudoconsole/internal/domain"
)

// Format selects the output style of the CLI.
type Format string

const (
	FormatHuman Format = "human"
	FormatJSON  Format = "json"
)

// ParseFormat normalises a CLI flag value. Empty strings map to
// FormatHuman so the user can rely on the documented default.
func ParseFormat(s string) (Format, error) {
	switch s {
	case "", string(FormatHuman):
		return FormatHuman, nil
	case string(FormatJSON):
		return FormatJSON, nil
	default:
		return "", fmt.Errorf("invalid format %q (want human or json)", s)
	}
}

// Formatter is the contract used by CLI commands to print results.
// Implementations must be safe for concurrent use.
type Formatter interface {
	Print(w io.Writer, v any) error
}

// NewFormatter returns a formatter for the given style. colour is
// enabled for human output only when wantColor is true (typically when
// stdout is a TTY).
func NewFormatter(f Format, wantColor bool) (Formatter, error) {
	switch f {
	case FormatHuman, "":
		return &humanFormatter{color: wantColor}, nil
	case FormatJSON:
		return &jsonFormatter{}, nil
	default:
		return nil, fmt.Errorf("formatter: unknown format %q", f)
	}
}

// --- human -------------------------------------------------------------

type humanFormatter struct {
	color bool
	mu    sync.Mutex
}

func (h *humanFormatter) Print(w io.Writer, v any) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	bw := bufio.NewWriter(w)
	defer func() { _ = bw.Flush() }()
	switch m := v.(type) {
	case nil:
		return nil
	case *AuthResult:
		return printAuth(bw, m, h.color)
	case *CheckResult:
		return printCheck(bw, m, h.color)
	case *ExecResult:
		return printExec(bw, m, h.color)
	case *VersionInfo:
		return printVersion(bw, m, h.color)
	case *AuditTailResult:
		return printAuditTail(bw, m, h.color)
	case *DetectResult:
		return printDetect(bw, m, h.color)
	case *InstallResult:
		return printInstall(bw, m, h.color)
	case *PolicyListResult:
		return printPolicyList(bw, m, h.color)
	case *PolicyTestResult:
		return printPolicyTest(bw, m, h.color)
	case *PolicyShowResult:
		return printPolicyShow(bw, m, h.color)
	case *PolicyValidateResult:
		return printPolicyValidate(bw, m, h.color)
	default:
		return fmt.Errorf("human formatter: unsupported value %T", v)
	}
}

func printAuth(w *bufio.Writer, r *AuthResult, c bool) error {
	if r.OK {
		g := color.New(color.FgGreen)
		if c {
			g.EnableColor()
		}
		_, _ = fmt.Fprintln(w, g.Sprint("✓ authentication cached"))
	} else {
		r2 := color.New(color.FgRed)
		if c {
			r2.EnableColor()
		}
		_, _ = fmt.Fprintln(w, r2.Sprint("✗ authentication failed"))
	}
	if r.Message != "" {
		_, _ = fmt.Fprintf(w, "  %s\n", r.Message)
	}
	return nil
}

func printCheck(w *bufio.Writer, r *CheckResult, c bool) error {
	if r.Active {
		g := color.New(color.FgGreen)
		if c {
			g.EnableColor()
		}
		_, _ = fmt.Fprintf(w, "%s sudo cache active (%.0fs remaining)\n",
			g.Sprint("✓"), r.RemainingSeconds)
	} else {
		y := color.New(color.FgYellow)
		if c {
			y.EnableColor()
		}
		_, _ = fmt.Fprintf(w, "%s sudo cache expired (requires authentication)\n",
			y.Sprint("!"))
	}
	return nil
}

func printExec(w *bufio.Writer, r *ExecResult, c bool) error {
	if r.Stdout != "" {
		_, _ = fmt.Fprint(w, r.Stdout)
	}
	if r.Stderr != "" {
		r2 := color.New(color.FgRed)
		if c {
			r2.EnableColor()
		}
		_, _ = fmt.Fprint(w, r2.Sprint(r.Stderr))
	}
	switch r.Decision {
	case "block":
		r2 := color.New(color.FgRed)
		if c {
			r2.EnableColor()
		}
		_, _ = fmt.Fprintf(w, "%s blocked by policy: %s\n",
			r2.Sprint("✗"), r.Reason)
	case "warn":
		y := color.New(color.FgYellow)
		if c {
			y.EnableColor()
		}
		_, _ = fmt.Fprintf(w, "%s warning: %s\n",
			y.Sprint("!"), r.Reason)
	}
	return nil
}

func printVersion(w *bufio.Writer, v *VersionInfo, c bool) error {
	cy := color.New(color.FgCyan)
	if c {
		cy.EnableColor()
	}
	_, _ = fmt.Fprintf(w, "%s %s\n", cy.Sprint("sudoconsole"), v.Version)
	_, _ = fmt.Fprintf(w, "  commit:  %s\n", v.Commit)
	_, _ = fmt.Fprintf(w, "  built:   %s\n", v.BuildDate)
	_, _ = fmt.Fprintf(w, "  go:      %s\n", v.GoVersion)
	_, _ = fmt.Fprintf(w, "  os/arch: %s/%s\n", v.GOOS, v.GOARCH)
	return nil
}

func printAuditTail(w *bufio.Writer, r *AuditTailResult, c bool) error {
	cy := color.New(color.FgCyan)
	if c {
		cy.EnableColor()
	}
	_, _ = fmt.Fprintf(w, "%s audit log: %s (%d records)\n", cy.Sprint("•"), r.Path, r.Count)
	for _, e := range r.Events {
		dc := color.New(color.FgWhite)
		switch e.Decision {
		case "block":
			dc = color.New(color.FgRed)
		case "warn":
			dc = color.New(color.FgYellow)
		case "audit", "allow":
			dc = color.New(color.FgGreen)
		}
		if c {
			dc.EnableColor()
		}
		_, _ = fmt.Fprintf(w, "  %s  %s  %s\n", e.Time, dc.Sprint(e.Decision), e.Command)
		if e.Redacted {
			_, _ = fmt.Fprintln(w, "       [redacted]")
		}
	}
	return nil
}

func printDetect(w *bufio.Writer, r *DetectResult, c bool) error {
	g := color.New(color.FgGreen)
	if c {
		g.EnableColor()
	}
	if len(r.Detected) == 0 {
		_, _ = fmt.Fprintln(w, "no supported agents found")
		return nil
	}
	_, _ = fmt.Fprintf(w, "%s %d agent(s) detected:\n", g.Sprint("•"), len(r.Detected))
	for _, a := range r.Detected {
		_, _ = fmt.Fprintf(w, "  %-8s  %s\n", a.Kind, a.Name)
		if a.BinaryPath != "" {
			_, _ = fmt.Fprintf(w, "    binary:   %s\n", a.BinaryPath)
		}
		if a.ConfigDir != "" {
			_, _ = fmt.Fprintf(w, "    config:   %s\n", a.ConfigDir)
		}
		if a.Version != "" {
			_, _ = fmt.Fprintf(w, "    version:  %s\n", a.Version)
		}
	}
	return nil
}

func printInstall(w *bufio.Writer, r *InstallResult, c bool) error {
	g := color.New(color.FgGreen)
	r2 := color.New(color.FgRed)
	y := color.New(color.FgYellow)
	if c {
		g.EnableColor()
		r2.EnableColor()
		y.EnableColor()
	}
	verb := "installed"
	if r.Planned {
		verb = "would install"
	}
	if len(r.Installed) > 0 {
		mark := g.Sprint("✓")
		if r.Planned {
			mark = y.Sprint("?")
		}
		_, _ = fmt.Fprintf(w, "%s %d %s\n", mark, len(r.Installed), verb)
		for _, o := range r.Installed {
			tag := ""
			if o.DryRun {
				tag = " (dry-run)"
			}
			_, _ = fmt.Fprintf(w, "  %s%s\n", o.Name, tag)
		}
	}
	if len(r.Failed) > 0 {
		_, _ = fmt.Fprintf(w, "%s %d failed\n", r2.Sprint("✗"), len(r.Failed))
		for _, e := range r.Failed {
			_, _ = fmt.Fprintf(w, "  %s: %s\n", e.Kind, e.Error)
		}
	}
	if len(r.Installed) == 0 && len(r.Failed) == 0 {
		_, _ = fmt.Fprintln(w, "no changes")
	}
	return nil
}

// --- json --------------------------------------------------------------

type jsonFormatter struct {
	mu sync.Mutex
}

func (j *jsonFormatter) Print(w io.Writer, v any) error {
	j.mu.Lock()
	defer j.mu.Unlock()
	if v == nil {
		return nil
	}
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return fmt.Errorf("json formatter: %w", err)
	}
	return nil
}

// --- result structs ----------------------------------------------------

// AuthResult is the outcome of the `auth` command.
type AuthResult struct {
	OK      bool   `json:"ok"`
	Message string `json:"message,omitempty"`
	Time    string `json:"time,omitempty"`
}

// CheckResult is the outcome of the `check` command.
type CheckResult struct {
	Active           bool    `json:"active"`
	RemainingSeconds float64 `json:"remaining_seconds"`
	Time             string  `json:"time,omitempty"`
}

// ExecResult is the outcome of the `exec` command.
type ExecResult struct {
	Decision   string `json:"decision"`
	Reason     string `json:"reason,omitempty"`
	Stdout     string `json:"stdout,omitempty"`
	Stderr     string `json:"stderr,omitempty"`
	ExitCode   int    `json:"exit_code"`
	DurationMs int64  `json:"duration_ms"`
	Cached     bool   `json:"cached"`
	Time       string `json:"time,omitempty"`
}

// VersionInfo is the build metadata.
type VersionInfo struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildDate string `json:"build_date"`
	GoVersion string `json:"go_version"`
	GOOS      string `json:"goos"`
	GOARCH    string `json:"goarch"`
	Time      string `json:"time,omitempty"`
}

// ErrUnsupported is returned by formatters when they cannot encode the
// supplied value.
var ErrUnsupported = errors.New("formatter: unsupported value")

// --- policy ------------------------------------------------------------

func printPolicyList(w *bufio.Writer, r *PolicyListResult, c bool) error {
	if len(r.Categories) == 0 {
		_, _ = fmt.Fprintln(w, "no categories registered")
		return nil
	}
	cy := color.New(color.FgCyan)
	if c {
		cy.EnableColor()
	}
	_, _ = fmt.Fprintf(w, "%s %d entries\n", cy.Sprint("•"), len(r.Categories))
	for _, e := range r.Categories {
		tag := "builtin"
		if !e.BuiltIn {
			tag = "custom"
		}
		_, _ = fmt.Fprintf(w, "  %-12s %-22s %-10s %s [%s]\n",
			e.Binary, e.Category, e.Risk, e.Notes, tag)
	}
	return nil
}

func printPolicyTest(w *bufio.Writer, r *PolicyTestResult, c bool) error {
	dc := color.New(color.FgWhite)
	switch r.Decision {
	case "block":
		dc = color.New(color.FgRed)
	case "warn":
		dc = color.New(color.FgYellow)
	case "audit", "allow":
		dc = color.New(color.FgGreen)
	}
	if c {
		dc.EnableColor()
	}
	_, _ = fmt.Fprintf(w, "%s  %s\n", dc.Sprint(r.Decision), r.Command)
	if len(r.Reasons) > 0 {
		for _, rs := range r.Reasons {
			_, _ = fmt.Fprintf(w, "    reason: %s\n", rs)
		}
	}
	if len(r.Categories) > 0 {
		_, _ = fmt.Fprintf(w, "    categories: %s\n", joinStrings(r.Categories))
	}
	if r.Risk != "" {
		_, _ = fmt.Fprintf(w, "    risk: %s\n", r.Risk)
	}
	return nil
}

func printPolicyShow(w *bufio.Writer, r *PolicyShowResult, c bool) error {
	cy := color.New(color.FgCyan)
	if c {
		cy.EnableColor()
	}
	_, _ = fmt.Fprintf(w, "%s policy\n", cy.Sprint("•"))
	_, _ = fmt.Fprintf(w, "  mode               = %s\n", r.Mode)
	if len(r.BlockedCategories) > 0 {
		_, _ = fmt.Fprintf(w, "  blocked.categories = %s\n", joinStrings(r.BlockedCategories))
	}
	if len(r.AllowedCategories) > 0 {
		_, _ = fmt.Fprintf(w, "  allowed.categories = %s\n", joinStrings(r.AllowedCategories))
	}
	if len(r.BlockedCommands) > 0 {
		_, _ = fmt.Fprintf(w, "  blocked.commands   = %v\n", r.BlockedCommands)
	}
	if len(r.AllowedCommands) > 0 {
		_, _ = fmt.Fprintf(w, "  allowed.commands   = %v\n", r.AllowedCommands)
	}
	if len(r.BlockedPatterns) > 0 {
		_, _ = fmt.Fprintf(w, "  blocked.patterns   = %v\n", r.BlockedPatterns)
	}
	if len(r.AllowedPatterns) > 0 {
		_, _ = fmt.Fprintf(w, "  allowed.patterns   = %v\n", r.AllowedPatterns)
	}
	if len(r.ExtraPatterns) > 0 {
		_, _ = fmt.Fprintf(w, "  extra_patterns     = %v\n", r.ExtraPatterns)
	}
	_, _ = fmt.Fprintf(w, "  remote_access      = %+v\n", r.RemoteAccess)
	_, _ = fmt.Fprintf(w, "  credential_exposure= %+v\n", r.CredentialExposure)
	_, _ = fmt.Fprintf(w, "  audit              = %+v\n", r.Audit)
	return nil
}

func printPolicyValidate(w *bufio.Writer, r *PolicyValidateResult, c bool) error {
	g := color.New(color.FgGreen)
	r2 := color.New(color.FgRed)
	if c {
		g.EnableColor()
		r2.EnableColor()
	}
	if r.Valid {
		_, _ = fmt.Fprintf(w, "%s all patterns compile cleanly\n", g.Sprint("✓"))
		return nil
	}
	_, _ = fmt.Fprintf(w, "%s invalid patterns:\n", r2.Sprint("✗"))
	writeSection := func(label string, errs []domain.PatternError) {
		for _, e := range errs {
			_, _ = fmt.Fprintf(w, "  %s: %s\n  %s\n", label, e.Pattern, e.Reason)
		}
	}
	writeSection("blocked", r.Blocked)
	writeSection("allowed", r.Allowed)
	writeSection("extras ", r.Extras)
	return nil
}

// joinStrings renders a slice of stringy values as a comma-separated list.
func joinStrings[T ~string](s []T) string {
	if len(s) == 0 {
		return ""
	}
	out := make([]string, len(s))
	for i, v := range s {
		out[i] = string(v)
	}
	return strings.Join(out, ", ")
}
