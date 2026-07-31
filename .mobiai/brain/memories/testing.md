# Testing Patterns

<!--
Reusable testing patterns discovered for this project.
Append entries with: mobiai brain save testing (coming in Phase 2).
Include the problem, the pattern that solved it and a minimal example.
-->

## M4: testing pattern — in-memory FS + cobra CLI tests

- id: m4-testing-pattern-in-memory-fs-cobra-cli-tests-20260731-123831
- type: testing_pattern
- status: active
- platform: shared
- area: config_loader
- date: 2026-07-31

## Pattern
- Config-package unit tests use a `memFS` (in `loader_test.go`) that satisfies the `config.FS` interface. This lets `Store.Load`/`Store.Save` be exercised without touching the host filesystem.
- CLI tests use `cobra.Command.SetArgs(...)` + `SetOut`/`SetErr` buffers. This lets the full command tree be executed including flag parsing and override layering.
- All tests must pass both `go test ./... -race` and `golangci-lint run ./...` (currently 0 issues).

## Why
- Keeps tests deterministic across machines (no env var dependency).
- Avoids flakiness from `t.TempDir` cleanup on CI.
- Mirrors the production code path exactly while keeping the FS swappable.

## Files
- `internal/infrastructure/config/loader_test.go` (memFS + decode/encode tests)
- `internal/transport/cli/cli_test.go` (cobra tree tests)

### Files
- internal/infrastructure/config/loader_test.go
- internal/transport/cli/cli_test.go
