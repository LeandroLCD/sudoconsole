# Architecture

## Overview

`sudoconsole` follows **Clean Architecture** (a.k.a. Hexagonal / Ports & Adapters) with four concentric layers. The dependency rule is strict: **outer layers depend on inner layers, never the reverse**.

```
┌──────────────────────────────────────────────────────────────┐
│  cmd/sudoconsole              ← composition root              │
│  internal/transport/cli       ← boundary adapters (cobra)    │
│  internal/infrastructure      ← adapters (PTY, sudo, etc.)   │
│  internal/usecase             ← use cases                     │
│  internal/domain              ← entities + ports              │
└──────────────────────────────────────────────────────────────┘
```

## Layers

### `internal/domain` — entities + ports

Pure business types and interface contracts. **No external dependencies** beyond Go stdlib. Defines:

- Entities: `Credential`, `Command`, `CacheStatus`, `Policy`, `AgentDescriptor`
- Ports (interfaces): `SudoGateway`, `PtyGateway`, `CacheRepository`, `PolicyEvaluator`, `AuditLogger`, `AgentInstaller`, `ConfigStore`, `AgentDetector`

Coverage target: **100%** (no untested business logic).

### `internal/usecase` — application logic

Use cases that orchestrate domain entities via ports. **Depends only on `internal/domain`** and `context`. Examples:

- `AuthenticateUseCase`
- `ExecuteWithSudoUseCase`
- `CheckCacheUseCase`
- `EvaluatePolicyUseCase`
- `DetectAgentsUseCase`
- `InstallAdapterUseCase`

Coverage target: **≥80%**.

### `internal/infrastructure` — adapters

Concrete implementations of the ports. Lives behind subpackages:

| Subpackage | Implements | Tech |
|------------|-----------|------|
| `pty`      | `PtyGateway` | `creack/pty` |
| `sudo`     | `SudoGateway` | `os/exec` over PTY |
| `cache`    | `CacheRepository` | `os.Stat` on sudo timestamp file |
| `config`   | `ConfigStore` | `pelletier/go-toml/v2` |
| `policy`   | `PolicyEvaluator` | glob + regex matcher |
| `audit`    | `AuditLogger` | append-only JSONL writer |
| `agent`    | `AgentDetector`, `AgentInstaller` | per-CLI adapters |

OS-specific code uses build tags: `//go:build linux` and `//go:build darwin`.

### `internal/transport/cli` — boundary adapters

Cobra commands that parse CLI args, build DTOs, call use cases, format output. **Composition root** — wires every dependency here, not in `domain`/`usecase`.

### `cmd/sudoconsole` — main

Just `main()`. Imports the root cobra command from `transport/cli` and calls `Execute()`.

## Dependency rules

| Layer | May import |
|-------|-----------|
| `domain` | stdlib only |
| `usecase` | `domain` + stdlib + `context` |
| `infrastructure` | `domain` + stdlib + 3rd-party |
| `transport/cli` | `usecase` + `infrastructure` + `domain` + stdlib + 3rd-party |
| `cmd/...` | everything (composition root only) |

Enforced by:
- Manual code review (PR checklist)
- `go vet ./...` (catches many cycles)
- Future: `go-arch-lint` rule file

## Data flow: `sudoconsole exec apt update`

```
user → CLI (cobra) → ExecuteWithSudoUseCase
                          │
                          ├─→ CheckCacheUseCase
                          │     └─→ CacheRepository.IsActive()
                          │           └─→ infra/cache: stat timestamp file
                          │
                          ├─→ (if miss) → AuthenticateUseCase
                          │     └─→ SudoGateway.Authenticate()
                          │           └─→ infra/sudo → infra/pty (silent read)
                          │
                          ├─→ EvaluatePolicyUseCase
                          │     └─→ PolicyEvaluator.Evaluate("apt update")
                          │           └─→ infra/policy: matcher + categories
                          │
                          ├─→ (if allowed) → SudoGateway.Execute("apt update")
                          │     └─→ infra/sudo: exec via sudo
                          │
                          └─→ AuditLogger.Log(decision, cmd, ...)
                                └─→ infra/audit: append JSONL
```

## Cross-cutting concerns

- **Logging**: `log/slog` from stdlib, configurable level per output formatter
- **Errors**: domain errors wrapped with `%w`; transport layer maps to exit codes
- **Context**: all use cases accept `context.Context` for cancellation/timeout
- **Concurrency**: PTY reads use goroutines + channels; detector uses `errgroup`

## Why Clean Architecture for a CLI?

- **Testability**: ports are interfaces; we can mock them in unit tests without touching real PTY/sudo
- **Portability**: swapping `creack/pty` for another PTY lib is a single-package change
- **Security review**: the `security/policy` package is the only place that decides what runs — easy to audit
- **Evolution**: adding a new CLI agent adapter is `internal/infrastructure/agent/<name>.go` + tests, no domain changes