# Decisions

<!--
Architecture decisions specific to this project.
Append entries with: mobiai brain save decision (coming in Phase 2).
Each entry should record: title, status (active|deprecated), platform,
area, date, decision, reason, files.
-->

## sudoconsole is a Go CLI tool, not a mobile project

- id: sudoconsole-is-a-go-cli-tool-not-a-mobile-project-20260731-123822
- type: architecture_decision
- status: active
- platform: shared
- area: project_type
- date: 2026-07-31

sudoconsole is a Go-based CLI that wraps sudo for AI coding agents (Kilo, Claude Code, Gemini, etc.). It uses Clean Architecture: domain → usecase → infrastructure → transport/cli. The mobiai brain scanner returns `project_type: unknown` because it is calibrated for mobile projects (Android/iOS/KMP/Flutter/RN). Do not be misled by the scanner — when proposing tooling or library choices, treat this as a server/CLI Go project, not mobile.

### Files
- README.md
- go.mod
- plans/00-master.md

## M4: config package uses pelletier/go-toml/v2 + injectable FS

- id: m4-config-package-uses-pelletier-go-toml-v2-injectable-fs-20260731-123826
- type: architecture_decision
- status: active
- platform: shared
- area: config_loader
- date: 2026-07-31

## Decision
M4 chose **pelletier/go-toml/v2** for TOML decoding and exposes an `FS` interface (`ReadFile/WriteFile/MkdirAll/Rename/Remove`) so the loader can be driven by an in-memory filesystem in tests.

## Reason
- `pelletier/go-toml/v2` is the smallest, fastest pure-Go TOML encoder/decoder with no reflection-heavy APIs.
- The injectable FS keeps tests hermetic — no `os.Chdir`, no temp dirs leaking into the real `$HOME`.
- `domain.ConfigStore` is the only contract the use-case layer sees; the loader is the single infrastructure adapter.

## Files
- `internal/infrastructure/config/loader.go` (Store + FS interface)
- `internal/infrastructure/config/schema.go` (TOML schema)
- `internal/infrastructure/config/validate.go` (validation, wraps errors as `domain.ErrConfigInvalid`)
- `internal/infrastructure/config/paths.go` (XDG on Linux, Application Support on macOS)

### Files
- internal/infrastructure/config/loader.go
- internal/infrastructure/config/schema.go
- internal/infrastructure/config/validate.go

## M4: error wrapping contract for config layer

- id: m4-error-wrapping-contract-for-config-layer-20260731-123845
- type: architecture_decision
- status: active
- platform: shared
- area: config_loader
- date: 2026-07-31

## Contract
- All validation errors from the config layer MUST wrap `domain.ErrConfigInvalid` (see `internal/domain/errors.go`) so callers can `errors.Is(err, domain.ErrConfigInvalid)`.
- Validation errors carry a field path (e.g. `policy.blocked.categories`) so the CLI can print `config: <field>: <message>`.
- The CLI tags every load error with `config: ` prefix before returning; `sudoconsole` exits 1.

### Files
- internal/domain/errors.go
- internal/infrastructure/config/validate.go
- internal/transport/cli/config.go

## M5: CLI uses composition root (App struct) wired in main.go

- id: m5-cli-uses-composition-root-app-struct-wired-in-main-go-20260731-125119
- type: architecture_decision
- status: active
- platform: shared
- area: cli
- date: 2026-07-31

## Decision
M5 introduced an `App` struct in `internal/transport/cli` that holds every
dependency (Gateway, Repository, Evaluator, Audit, Logger, Formatter,
Now, Config). `cmd/sudoconsole/main.go` builds it once at process start
and passes it to every cobra command.

## Reason
- Single composition root → no globals in the CLI package.
- Tests can construct an `App` with fakes (`stubApp` in
  `testhelpers_test.go`) and exercise individual commands without
  touching the real filesystem or PTY.
- Cobra commands are pure functions of their `App` — easy to compose
  and to extend in M6+.

## Files
- `internal/transport/cli/auth_cmd.go` (`authDeps`, `runAuth`)
- `internal/transport/cli/check_cmd.go` (`checkDeps`, `runCheck`)
- `internal/transport/cli/exec_cmd.go` (`execDeps`, `runExec`)
- `internal/transport/cli/testhelpers_test.go` (`stubApp`)
- `cmd/sudoconsole/main.go` (builds App, wires CLI)

### Files
- internal/transport/cli/auth_cmd.go
- internal/transport/cli/check_cmd.go
- internal/transport/cli/exec_cmd.go
- cmd/sudoconsole/main.go

## M5 carry-over: Evaluator must satisfy domain.PolicyEvaluator

- id: m5-carry-over-evaluator-must-satisfy-domain-policyevaluator-20260731-125124
- type: architecture_decision
- status: active
- platform: shared
- area: policy_evaluator
- date: 2026-07-31

## Bug fixed during M5
`domain.PolicyEvaluator.Evaluate(ctx, policy, cmd)` requires 3 args; the
M2 Evaluator implemented `Evaluate(ctx, cmd)` only. The mismatch was
hidden until M5 tried to wire the Evaluator into the App.

## Fix
- Added `Evaluate(ctx, policy, cmd)` that temporarily swaps `e.policy`
  for the call.
- Kept `EvaluateStored(ctx, cmd)` for tests that rely on the struct-stored
  policy.
- Updated the test suite to use `EvaluateStored` (sed replacement).

## Files
- `internal/infrastructure/policy/evaluator.go`
- `internal/infrastructure/policy/evaluator_test.go` (~37 calls updated)

### Files
- internal/infrastructure/policy/evaluator.go
- internal/domain/ports.go

## M6: audit log enriched with host/user/session/policy_hash

- id: m6-audit-log-enriched-with-host-user-session-policy-hash-20260731-130413
- type: architecture_decision
- status: active
- platform: shared
- area: audit
- date: 2026-07-31

## Decision
The audit `FileLogger` auto-populates `Timestamp`, `User`, `Hostname`,
`SessionID` and `PolicyHash` on every event via a private `enrich()`
helper. The `domain.AuditEvent` struct gained two new fields:
`PolicyHash` and `Redacted`.

## Reason
- Callers (usecases) only need to populate Command / Decision /
  Result / ExitCode / OverrideBy; the audit logger fills the rest.
- Single source of truth: if any field is omitted it falls back to
  the logger-detected value, never an empty string.
- `PolicyHash` makes audit events correlatable with the policy version
  that produced the decision.

## Files
- `internal/infrastructure/audit/audit.go` (`enrich`, `HashPolicy`,
  `SetPolicyHash`, `meta` struct)
- `internal/domain/ports.go` (`AuditEvent.PolicyHash`, `AuditEvent.Redacted`)

### Files
- internal/infrastructure/audit/audit.go
- internal/domain/ports.go

## M6: secrets redacted via RedactCommand heuristic

- id: m6-secrets-redacted-via-redactcommand-heuristic-20260731-130413
- type: architecture_decision
- status: active
- platform: shared
- area: audit
- date: 2026-07-31

## Decision
`RedactCommand(c)` mutates a copy of the command, replacing
secret-looking arguments with `<REDACTED>` and setting
`event.Redacted = true`. The function handles four patterns:

1. Long flags: `--password=`, `--passwd=`, `--token=`, `--secret=`.
2. Embedded markers: `password=`, `passwd=`, `token=`, `secret=`
   inside any argument (e.g. curl --data-urlencode).
3. KEY=VALUE environment-style assignments where KEY is sensitive.
4. MySQL-style `-p<password>` flags.

`sudo -S` is NOT redacted because the password is read from stdin,
not from argv.

## Files
- `internal/infrastructure/audit/audit.go` (`RedactCommand`, helpers
  `redactFlag`, `redactEmbedded`, `redactEnvVar`, `redactShortPassword`)

### Files
- internal/infrastructure/audit/audit.go

## M7: agent detector uses parallel probes with 2s budget

- id: m7-agent-detector-uses-parallel-probes-with-2s-budget-20260731-134524
- type: architecture_decision
- status: active
- platform: shared
- area: agent_detector
- date: 2026-07-31

## Decision
`agent.Detector.Detect` uses `golang.org/x/sync/errgroup` to probe every
supported CLI agent in parallel with a hard 2-second deadline. Each
per-agent probe has its own 500 ms version-flag timeout. A single
failing probe (binary missing, version flag unsupported) does not abort
the scan; the result simply omits that kind.

The detector matches an agent if EITHER its binary is on PATH OR its
config dir exists under `$HOME`. This handles npm/pip installs that
land on non-standard PATHs but still drop a `.config/<name>` dir.

## Files
- `internal/infrastructure/agent/detector.go` (Specs, Detector,
  lookupBinary, lookupConfigDir, lookupVersion)
- `internal/infrastructure/agent/detector_test.go` (8 tests with
  `testing/fstest.MapFS` + injectable PathLayout)

### Files
- internal/infrastructure/agent/detector.go
