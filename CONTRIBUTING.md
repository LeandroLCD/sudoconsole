# Contributing

Thanks for considering a contribution. This document covers the
end-to-end dev workflow, the conventions we follow, and how to
land a change.

## Ground rules

1. **One milestone per PR.** Each PR should map to exactly one
   `plans/M*.md` task. Multi-milestone PRs are hard to review and
   hard to revert.
2. **Tests first or tests with the change.** No PR without new
   unit-test coverage (use cases / ports) **and** an updated CLI
   test if the command surface changed.
3. **`go vet`, `golangci-lint`, `gosec` must stay clean.** The CI
   pipeline blocks merges when any of these fail.
4. **No new public API without discussion.** Open an issue first if
   you're adding ports, use cases, or CLI commands.

## Development setup

```bash
git clone https://github.com/LeandroLCD/sudoconsole
cd sudoconsole

# Required: Go 1.25+, GNU make, git.
# Recommended: golangci-lint, gosec, goreleaser.
make tools   # install go-based tools (see below)

make build
make test
make test-integration   # requires sudo + docker; CI runs this in the matrix
make lint
make security
make release-dry       # produces all artifacts under ./dist/ without publishing
```

`make tools` installs the following go binaries into `$(go env GOPATH)/bin`:

| Tool | Why |
|------|-----|
| `golangci-lint` | aggregate linter (gofmt, govet, errcheck, ineffassign, gocritic, gocyclo, gosec, …) |
| `gosec` | security scanner (G101-G404) |
| `goreleaser` | release pipeline (only for M13) |

The CI workflow installs the same versions, so failures will match
locally.

## Code layout

```
cmd/sudoconsole/         ← composition root (main package)
cmd/docgen/              ← cobra-doc generator (manpages + md)
internal/domain/         ← entities + ports (zero external deps)
internal/usecase/        ← use cases (depend only on domain)
internal/infrastructure/ ← adapters (pty, sudo, cache, config, policy, audit, agents)
internal/transport/cli/  ← cobra commands
test/integration/        ← E2E suite with `integration` build tag
plans/                   ← milestone plans + master roadmap
docs/                    ← ARCHITECTURE / AGENTS / CONFIG / SECURITY / RELEASE / FAQ
manpages/                ← generated man pages (do not edit by hand)
```

See [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) for the design
rationale.

## Conventions

### Style

- `gofmt` (no custom rules).
- `golangci-lint` runs with the project config in `.golangci.yml`.
  Specifically: gocyclo `15` is the upper bound per function. Extract
  helpers when a function grows past that.
- No exported names from `internal/` packages. (They're `internal/`
  so this is enforced by the Go toolchain.)

### Domain types

- Add new entities to `internal/domain/`. They must not import
  anything outside the standard library and `internal/domain` itself.
- Add new ports (interfaces) to `internal/domain/ports.go` or to a
  dedicated `<feature>_ports.go`. Implementations live under
  `internal/infrastructure/<feature>/`.

### Use cases

- Each use case is a struct with its own `Execute(ctx, in) (out, error)`
  method. Constructor is `New<Name>UseCase(deps...)`.
- Use cases depend only on `domain` ports, never on infrastructure
  types directly.

### CLI commands

- One file per command (`install_cmd.go`, `exec_cmd.go`, …).
- Command function `new<X>Cmd(app *App) *cobra.Command`.
- `RunE` calls into a `runX` helper that builds the use case,
  invokes it, formats the result, and returns the cobra error.
- Human formatter: a `printX(bw, r, c)` function in `output.go`.
- JSON formatter: same result struct, encoded by the shared
  `jsonFormatter.Print`.

### Tests

- Unit tests live next to the code (`foo.go` → `foo_test.go`,
  same package).
- Use table-driven tests for port / use-case variants.
- Use `testing/fstest.MapFS` for filesystem adapters — never use
  `t.TempDir()` for unit tests (it makes the test order-dependent).
- CLI tests use the existing `cmdFromArgs(app, args)` helper in
  `commands_test.go`. Prefer adding cases to the table there over
  writing a new helper.
- Integration tests live in `test/integration/` with the
  `integration` build tag. See the [README](../README.md#integration-tests)
  for how to run them.

### Commit messages

- `feat:` / `fix:` / `refactor:` / `chore:` / `docs:` / `test:` /
  `ci:` prefixes (Conventional Commits).
- Subject ≤ 72 chars. Body wrapped at 72.
- Reference the milestone in the subject: `[M9] install/uninstall
  orchestration use case + flags`.
- Body should explain *why*, not *what* — the diff shows the *what*.

## Working on a milestone

1. Create a branch from `develop`:
   ```bash
   git checkout develop && git pull
   git checkout -b feature/mN-<slug>
   ```
2. Read the plan: `plans/MN-<name>.md`. Each task has a checkbox.
3. Use TDD where it makes sense: write the use-case test first,
   then the implementation, then the CLI test.
4. Run `make test lint security` locally before pushing.
5. Push and open a PR. Title: `[MN] <short description>`. Body must
   reference the issue (`Closes #N`) and list what was delivered.
6. CI runs the unit suite on 4 GOOS/GOARCH pairs plus the
   integration matrix. Wait for green; then merge with **squash**.
7. Update `plans/MN-<name>.md`: check the boxes you completed.

## Review checklist

For each PR, the reviewer confirms:

- [ ] `go test ./... -count=1` is green
- [ ] `golangci-lint run ./...` reports 0 issues
- [ ] `gosec ./...` reports 0 issues
- [ ] The integration matrix (4 distros + macOS) is green for
      touch-the-binary changes
- [ ] New public types/functions have documentation comments
- [ ] New ports have at least one in-memory fake used by tests
- [ ] The PR title follows `[MN] <description>`
- [ ] The PR body references the issue it closes

## Adding a new CLI agent adapter

New agents are added under `internal/infrastructure/agent/<name>.go`
and registered in `internal/infrastructure/agent/registry.go`. See
[`docs/AGENTS.md`](docs/AGENTS.md) for the protocol and
`internal/infrastructure/agent/kilo.go` for the canonical example.

## Reporting security issues

Please open a **private** security advisory (GitHub Security tab
→ "Report a vulnerability"). Do **not** file a public issue for
security bugs — see [`docs/SECURITY.md`](docs/SECURITY.md) for the
full disclosure process.

## Code of conduct

Be respectful. Critique ideas, not people. Focus on what is best
for the project and its users.