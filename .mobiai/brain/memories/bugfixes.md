# Bugfixes

<!--
Bugfixes and workarounds worth remembering for this project.
Append entries with: mobiai brain save bugfix (coming in Phase 2).
Mark temporary workarounds as status: temporary so the agent does not
treat them as permanent decisions.
-->

## M4: CLI prints must ignore fmt.Fprint* errors

- id: m4-cli-prints-must-ignore-fmt-fprint-errors-20260731-123835
- type: bug_fix
- status: active
- platform: shared
- area: linting
- date: 2026-07-31

## Problem
`golangci-lint` (errcheck) flags every `fmt.Fprintf`/`fmt.Fprintln` call in the CLI package because the error return is ignored. The same issue affects any CLI command that writes to `cmd.OutOrStdout()`.

## Solution
Use the `_, _ = fmt.Fprintf(...)` pattern at every CLI output site. Rationale: the writer is owned by cobra (typically `os.Stdout`), and a write failure at this stage means the user pipe is gone — the OS will surface it on the next syscall. Returning the error would just spam the user with a confusing "write to closed pipe" message.

### Files
- internal/transport/cli/config.go
- internal/transport/cli/version.go
