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
