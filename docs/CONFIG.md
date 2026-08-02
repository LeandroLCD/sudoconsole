# Configuration reference

`sudoconsole` reads a single TOML file from one of:

| Platform | Path |
|----------|------|
| Linux    | `$XDG_CONFIG_HOME/sudoconsole/config.toml` (falls back to `~/.config/sudoconsole/config.toml`) |
| macOS    | `$HOME/Library/Application Support/sudoconsole/config.toml` |

You can override the path with the `--config /path/to/file` flag or
the `SUDOCONSOLE_CONFIG` environment variable.

A complete example is shipped at [`examples/config.toml`](../examples/config.toml).
The CLI also has a `config show` subcommand to print the effective
(everything-merged) configuration:

```bash
sudoconsole config show
sudoconsole --format json config show
```

## Top-level sections

| Section | Purpose |
|---------|---------|
| `[cache]` | sudo timestamp cache behaviour |
| `[security]` | memory hardening, core-dump policy, TTY requirement |
| `[policy]` | allow/block/audit rules + remote-access + credential-exposure categories |
| `[agent]` | agent auto-detection and per-agent installation knobs |
| `[output]` | output format + log level |

## `[cache]`

| Key | Type | Default | Valid range | Notes |
|-----|------|---------|-------------|-------|
| `timeout_seconds` | int | `900` | `60-3600` | Total lifetime of the sudo cache after a successful `auth`. |
| `refresh_before_seconds` | int | `60` | `< timeout_seconds` | Cache is refreshed (no prompt) when the remaining lifetime drops below this value. |
| `disabled` | bool | `false` | — | When true, `auth` is a no-op and every `exec` requires `--policy-override` or a fresh sudo invocation. |

## `[security]`

| Key | Type | Default | Notes |
|-----|------|---------|-------|
| `purge_memory` | bool | `true` | Zeroize credential bytes after `auth` returns. |
| `disable_core_dumps` | bool | `true` | Set `RLIMIT_CORE=0` on the parent before spawning sudo so credentials cannot leak via coredumps. |
| `require_tty` | bool | `true` | Refuse to run if stdin is not a TTY. Defeats most scripted credential leaks. |

## `[policy]`

### `[policy.mode]`

The active mode determines how a Block decision is handled:

| Mode | Block becomes | Default use |
|------|--------------|-------------|
| `blocklist` (default) | exit 64, no execution | safe-by-default |
| `allowlist` | exit 64 unless the command matches an `[policy.allowed]` entry | very strict environments |
| `audit` | log to the audit file, then execute anyway | roll-out period |

### `[policy.blocked]`

Categories of binaries that are blocked by default. You can extend
this list, but be aware that doing so weakens the safe-by-default
position.

```toml
[policy.blocked]
categories = ["remote_access", "credential_exposure"]
commands   = ["custom-privileged-tool"]
patterns   = ["re:ssh\\s+-R\\s+", "curl\\s+.*\\|\\s*sh"]
```

Three list types:

| Key | Type | Match style |
|-----|------|-------------|
| `categories` | `[]string` | one of `package_manager`, `remote_access`, `shell_spawn`, `credential_exposure`, `system_admin`, `file_destroyer` |
| `commands` | `[]string` | first argv element, exact match |
| `patterns` | `[]string` | path-style glob by default; prefix with `re:` for full regex. ReDoS-prone patterns are rejected at config load. |

### `[policy.allowed]`

Same shape as `[policy.blocked]`. Used in `allowlist` mode to
define the small set of commands that are permitted. Ignored in
`blocklist` mode.

### `[policy.audit]`

| Key | Type | Default | Notes |
|-----|------|---------|-------|
| `log_file` | string | empty | If empty, audit logging is a no-op. Otherwise a JSONL file is appended. The parent directory must exist (we do not create it). |
| `log_blocked` | bool | `false` | Append every Block decision to the log. |
| `log_allowed` | bool | `false` | Append every Allow decision. |
| `log_warned` | bool | `false` | Append every Warn decision. |
| `max_bytes` | int | `10485760` (10 MiB) | Truncate+rotate the log when it exceeds this size. |

### `[policy.extra_patterns]`

`[]string` — patterns evaluated after every block match. Useful for
shaping the policy without modifying the built-in defaults.

### `[policy.remote_access]` and `[policy.credential_exposure]`

These two sections are flags that *enable* extra sub-classification
inside the matching categories. They do not add commands; they make
the matcher stricter.

```toml
[policy.remote_access]
block_reverse_tunnels = true   # ssh -R, ncat -e, chisel reverse
block_port_forward    = true   # ssh -L, ncat -l
block_shell_spawn     = true   # ssh + bash, python -c 'import pty'
block_tunnels         = true   # ssh -w, wireguard

[policy.credential_exposure]
block_passwd_change = true     # passwd, chpasswd
block_shadow_edit    = true     # vipw, edit /etc/shadow
block_sudoers_edit   = true     # visudo, edit /etc/sudoers
block_ssh_key_export = true     # ssh-keygen, ssh-copy-id
block_secret_export  = true     # printenv, env, cat .env
```

## `[agent]`

| Key | Type | Default | Notes |
|-----|------|---------|-------|
| `auto_detect` | bool | `true` | Discover installed agents before `install` / `uninstall`. |
| `install` | `[]string` | empty | Restrict `install` to these agents. Valid values: `kilo`, `claude`, `gemini`, `aider`, `codex`, `copilot`, `generic`. |
| `bin_dir` | string | `~/bin` | Where `install` writes wrapper scripts. |

## `[output]`

| Key | Type | Default | Choices |
|-----|------|---------|---------|
| `format` | string | `human` | `human`, `json` |
| `log_level` | string | `info` | `silent`, `error`, `warn`, `info`, `debug` |

> **Note:** `output.format` set in the config takes precedence over
> `--format` on the command line. The CLI flag is honoured only when
> the config does not specify a format. (See [release history] for
> the rationale.)

## CLI flag overrides

The following flags apply on every invocation and override the
file-based values:

| Flag | Effect |
|------|--------|
| `--cache-timeout N` | override `cache.timeout_seconds` for this invocation |
| `--format json` | request machine-readable output |
| `--log-level info` | raise or lower verbosity |
| `--config /path/to.toml` | use an alternative config file |

## See also

- [`docs/SECURITY.md`](SECURITY.md) — threat model
- [`docs/ARCHITECTURE.md`](ARCHITECTURE.md) — how the loader fits in
- [`examples/config.toml`](../examples/config.toml) — annotated example