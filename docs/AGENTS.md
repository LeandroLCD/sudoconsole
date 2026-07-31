# Agent Adapter Protocol

> How sudoconsole integrates with each supported CLI coding agent.

`sudoconsole install` registers itself with every detected CLI agent by
writing a small, idempotent marker file or settings entry. `sudoconsole
uninstall` removes only what `install` wrote — it never touches files
that do not contain the `sudoconsole-marker: <kind>` line.

## Marker convention

Every adapter writes (or patches) a single block identified by a
comment line:

```
# sudoconsole-marker: <kind>
```

Adapters use this marker for:

- **Idempotency**: a second `install` call sees the marker and exits
  without rewriting the file (unless `--force` is passed).
- **Uninstall scoping**: only blocks prefixed by the marker are
  removed; everything else is left untouched.
- **Audit traceability**: the marker is also recorded in the audit
  log so operators can correlate the integration with a given kind.

## Supported adapters

| Kind     | Display name   | Target file                                   | Adapter command           |
|----------|----------------|-----------------------------------------------|---------------------------|
| kilo     | Kilo CLI       | `~/.config/kilo/commands/sudoconsole.md`      | `sudoconsole`             |
| claude   | Claude Code    | `~/.claude/commands/sudoconsole.md` + `~/.claude/settings.json` (`sudoconsole` key) | `sudoconsole`             |
| gemini   | Gemini CLI     | `~/.gemini/tools/sudoconsole.toml`             | `sudoconsole`             |
| aider    | Aider          | `~/.aider.conf.yml` (`alias:` block)          | `sudoconsole`             |
| codex    | OpenAI Codex   | `~/.codex/sudosafe.toml`                      | `sudoconsole`             |
| copilot  | GitHub Copilot | `gh alias set sudoconsole 'sudoconsole'`      | `sudoconsole`             |
| generic  | Generic shell  | `~/.bashrc` + `~/.zshrc` (`alias:` block)     | `sudosafe`                |

`generic` is **opt-in**: `sudoconsole install` does not touch rc
files unless the user passes `--kind generic`.

## Per-adapter examples

### Kilo

Drop a slash command file and you're done.

```
~/.config/kilo/commands/sudoconsole.md
```

User invokes `/sudoconsole exec apt update` from the agent chat.

### Claude Code

Two files:

```
~/.claude/commands/sudoconsole.md
~/.claude/settings.json   # adds { "sudoconsole": { ... } }
```

`sudoconsole uninstall` removes both. The settings.json key is removed
without touching any other keys.

### Gemini

```
~/.gemini/tools/sudoconsole.toml
```

```toml
[tool]
name = "sudoconsole"
description = "Run a shell command under sudo with policy enforcement."
command = "sudoconsole"
```

### Aider

Appends to `~/.aider.conf.yml`:

```yaml
# sudoconsole-marker: aider
alias:
  sudoconsole: sudoconsole
```

Aider re-reads its config on every invocation so no restart is
needed.

### Codex

```
~/.codex/sudosafe.toml
```

### Copilot

Delegates to the `gh` CLI:

```
gh alias set sudoconsole 'sudoconsole'
gh alias delete sudoconsole    # on uninstall
```

If `gh` is not installed, the install call returns
`ErrAgentInstallFailed` and the user is informed.

### Generic

Appends to `~/.bashrc` and `~/.zshrc`:

```yaml
# sudoconsole-marker: generic
alias:
  sudosafe: sudoconsole
```

The alias name is `sudosafe` to avoid colliding with the bare
`sudoconsole` binary already on PATH.

## Idempotency guarantees

`install` is safe to re-run:

1. If the target file does not exist, it is created.
2. If the target file exists and contains the marker, the call is a
   no-op (unless `--force` is passed).
3. If the target file exists without the marker, the install **refuses
   to overwrite** unless `--force` is passed; the failure is reported
   per-adapter in the JSON output under `failed[]`.

`uninstall` is similarly safe:

1. Missing files are silently skipped.
2. Blocks without the marker are left untouched.
3. After uninstall, the file (if non-empty) preserves every other
   line.

## Operational flow

```bash
# 1. discover what is installed
sudoconsole detect

# 2. preview the install (no writes)
sudoconsole install --dry-run

# 3. install everything that was detected
sudoconsole install

# 4. install a single agent
sudoconsole install --kind kilo

# 5. force overwrite an existing integration
sudoconsole install --force --kind claude

# 6. clean up
sudoconsole uninstall
```

## Adding a new adapter

To add support for a new CLI agent:

1. Append a `Spec` entry to `Specs` in
   `internal/infrastructure/agent/detector.go`.
2. Create `internal/infrastructure/agent/<kind>.go` implementing the
   five `domain.AgentInstaller` methods (`Name`, `Detect`, `Install`,
   `Uninstall`, `AdapterCommand`).
3. Register the constructor in `NewForKind` in
   `internal/infrastructure/agent/registry.go`.
4. Add contract tests in `contract_test.go` covering: install,
   idempotent install, force overwrite, uninstall, idempotent
   uninstall, dry-run.
5. Add a row to the table above and a per-adapter example.