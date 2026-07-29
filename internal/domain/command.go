package domain

import (
	"errors"
	"fmt"
	"strings"
)

// Command represents a process invocation to be executed (possibly with sudo).
//
// A Command is immutable; construct via NewCommand. The Raw field preserves
// the original user-typed string for display and policy matching; Path and
// Args are the parsed form used by the executor.
//
// All fields are exported for serialization; do NOT embed a Credential here.
type Command struct {
	// Path is the resolved executable path (may be a basename).
	Path string
	// Args is the full argument vector (excluding Path itself).
	Args []string
	// Env holds optional environment variables for the child process.
	// Each entry is in the form "KEY=VALUE".
	Env []string
	// Dir is the working directory ("" = inherit current).
	Dir string
	// Raw is the original unparsed command line as typed by the user.
	// Used for policy matching and audit display.
	Raw string
}

// NewCommand constructs a Command from an argument vector.
//
// path must be non-empty. The Args slice is defensively copied.
func NewCommand(path string, args []string) (Command, error) {
	if strings.TrimSpace(path) == "" {
		return Command{}, fmt.Errorf("%w: path is empty", ErrInvalidCommand)
	}
	argsCopy := make([]string, len(args))
	copy(argsCopy, args)
	return Command{
		Path: path,
		Args: argsCopy,
		Raw:  buildRaw(path, args),
	}, nil
}

// NewCommandFromRaw parses a single command line into a Command.
//
// The parser is intentionally minimal: it splits on whitespace and does
// NOT handle quoting/escaping. Callers that need shell-like parsing
// should pre-process the input. This is by design — sudoconsole's policy
// matcher operates on the unparsed Raw form so that dangerous patterns
// (e.g. `ssh -R 8080:localhost:80 user@host`) are preserved verbatim.
func NewCommandFromRaw(raw string) (Command, error) {
	if strings.TrimSpace(raw) == "" {
		return Command{}, fmt.Errorf("%w: raw is empty", ErrInvalidCommand)
	}
	fields := strings.Fields(raw)
	// strings.Fields never returns empty if TrimSpace is non-empty,
	// so no length check is needed.
	return NewCommand(fields[0], fields[1:])
}

// buildRaw reconstructs a raw command line from path and args. Used only
// when the caller does not supply Raw explicitly (e.g. via NewCommand).
func buildRaw(path string, args []string) string {
	if len(args) == 0 {
		return path
	}
	var b strings.Builder
	b.WriteString(path)
	for _, a := range args {
		b.WriteByte(' ')
		if strings.ContainsAny(a, " \t\"'\\") {
			fmt.Fprintf(&b, "%q", a)
		} else {
			b.WriteString(a)
		}
	}
	return b.String()
}

// String returns the human-readable form, which is Raw when present or
// the reconstructed form otherwise.
func (c Command) String() string {
	if c.Raw != "" {
		return c.Raw
	}
	return buildRaw(c.Path, c.Args)
}

// CommandLine returns the single-line form suitable for logging.
func (c Command) CommandLine() string {
	return c.String()
}

// Basename returns the executable name without directory components.
func (c Command) Basename() string {
	i := strings.LastIndex(c.Path, "/")
	if i < 0 {
		return c.Path
	}
	return c.Path[i+1:]
}

// Equals reports whether two commands have identical Path and Args.
func (c Command) Equals(other Command) bool {
	if c.Path != other.Path {
		return false
	}
	if len(c.Args) != len(other.Args) {
		return false
	}
	for i := range c.Args {
		if c.Args[i] != other.Args[i] {
			return false
		}
	}
	return true
}

// Is checks whether the command's basename matches the given binary name.
// Used by the policy engine to short-circuit category lookups.
func (c Command) Is(binaryName string) bool {
	return c.Basename() == binaryName
}

// ContainsAny returns true if any of the command's arguments (or the
// executable path) contains any of the given substrings. Used by policy
// pattern matching (e.g. detecting `ssh -R` flags).
func (c Command) ContainsAny(needles []string) bool {
	if c.ContainsPath(needles) {
		return true
	}
	for _, a := range c.Args {
		for _, n := range needles {
			if strings.Contains(a, n) {
				return true
			}
		}
	}
	return false
}

// ContainsPath returns true if Path contains any of the substrings.
func (c Command) ContainsPath(needles []string) bool {
	for _, n := range needles {
		if strings.Contains(c.Path, n) {
			return true
		}
	}
	return false
}

// Validate returns nil if the command is well-formed (non-empty path).
func (c Command) Validate() error {
	if strings.TrimSpace(c.Path) == "" {
		return fmt.Errorf("%w: empty path", ErrInvalidCommand)
	}
	return nil
}

// ErrInvalidCommand is the sentinel for malformed commands.
// Concrete error wrapping happens at call sites.
var _ = errors.New
