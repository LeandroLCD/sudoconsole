# Frequently asked questions

## General

### Why does this exist?

CLI agents (Kilo, Claude Code, Aider, Codex, Copilot, ...) need to
run `sudo` to do anything privileged. Without help they either:

1. store the plaintext password in environment variables (which
   leaks through `/proc/<pid>/environ`, agent event streams, and
   `ps`-style spies), or
2. ask the human to type the password for every privileged command,
   breaking the agent's loop.

`sudoconsole` lets the human authenticate once and then grants
the agent access for a bounded window — without ever putting the
password on argv or in a config file.

### Is `sudoconsole` a sudo replacement?

No. It is a wrapper that uses the real `sudo(8)` for every privileged
invocation. The cache that lets the agent run multiple commands
without re-prompting is `sudo`'s own timestamp cache, not a custom
implementation.

### Does it require root?

To run `sudoconsole` you do not need to be root. To perform
privileged actions you must be in the `sudo` group (or otherwise
authorised by `/etc/sudoers`).

## Behaviour

### Why does `exec ssh user@host` exit 64?

`ssh` is in the built-in `remote_access` category. The default policy
mode (`blocklist`) refuses to execute it. Use `--policy-override
"<reason>" --yes` to bypass once, or relax the policy in your config
(see [`docs/CONFIG.md`](CONFIG.md)).

### My `--policy-override` was rejected. Why?

Interactive confirm. Either type `y` at the prompt or pass `--yes`:

```bash
sudoconsole exec --policy-override "debugging" --yes ssh user@host
```

### The audit log file isn't being written. Why?

Two common reasons:

1. `policy.audit.log_file` is empty in the config — there is nothing
   to write to.
2. The parent directory does not exist (we do not `MkdirAll` because
   we don't want to surprise the user with permissions on their
   `$XDG_DATA_HOME`).

```bash
mkdir -p ~/.local/share/sudoconsole
echo 'audit = { log_file = "~/.local/share/sudoconsole/audit.log", log_blocked = true, log_allowed = true }' >> ~/.config/sudoconsole/config.toml
```

### `check` reports `active: false` immediately after `auth`

Check runs `sudo -v` first, then re-reads the timestamp file. The
`timestamp_timeout` in your sudoers controls how long the cache
remains valid. On distros where this defaults to `0`, the cache
expires after every command. Set it to `60` or longer in
`/etc/sudoers.d/sudoconsole`.

## Configuration

### Where is the config file?

| Platform | Path |
|----------|------|
| Linux    | `~/.config/sudoconsole/config.toml` |
| macOS    | `~/Library/Application Support/sudoconsole/config.toml` |

Override with `--config` or `SUDOCONSOLE_CONFIG`.

### I edited `config.toml` but the change didn't take effect.

Most sub-commands load the config at startup. Restart the binary.
`config show` prints the *effective* (defaults-merged) values, which
is the easiest way to verify.

### Can I use environment variables for secrets?

Only `SUDOCONSOLE_PASSWORD` is recognised, and only when `--no-tty`
is supplied to `auth`. There is no way to put the password in
`config.toml`; that is a deliberate decision.

## Operations

### Does it work without a TTY (e.g. in CI)?

`auth` refuses to run without a TTY unless you also pass `--no-tty`.
With `--no-tty` the binary reads `SUDOCONSOLE_PASSWORD` from the
environment instead of prompting. CI pipelines should:

1. set `require_tty = false` in `[security]`,
2. set `--no-tty` and supply `SUDOCONSOLE_PASSWORD=<...>`.

### Why isn't the audit log structured for `journald`?

The audit log is JSONL (one event per line) because we want it to
be ingestable by every SIEM, not just `journalctl`. If you want
syslog forwarding, run `tail -F ~/.local/share/sudoconsole/audit.log | logger -t sudoconsole`
in a background process.

## Building

### `go build` fails with "command not found: pty"

You're missing the `creack/pty` C dependency. On Linux:

```bash
sudo apt install libutempter-dev   # Debian/Ubuntu
sudo dnf install systemd-devel      # Fedora
```

Then `go build`.

### How do I cross-compile for macOS from Linux?

```bash
GOOS=darwin GOARCH=arm64 go build -o dist/sudoconsole_darwin_arm64 ./cmd/sudoconsole
GOOS=darwin GOARCH=amd64 go build -o dist/sudoconsole_darwin_amd64 ./cmd/sudoconsole
```

The binary is statically linked (`CGO_ENABLED=0`); no Mac
toolchain required.

## Security

### Could a co-resident process read my password?

We defeat the most common vectors:

- The password never appears on argv (`ps` / `/proc/<pid>/cmdline`).
- `RLIMIT_CORE=0` prevents coredumps.
- The credential buffer is heap-allocated and overwritten on use.

See [`docs/SECURITY.md`](SECURITY.md) for the full threat model.

### I want to run the agent under `strace` to debug it.

`strace` (or any `ptrace` from the same UID) can read the password
out of the PTY write buffer. The protection is best-effort; if your
threat model includes a same-UID attacker with ptrace, you need
additional isolation (e.g. a separate user account).

## Where to go next

- [`docs/RELEASE.md`](RELEASE.md) — tagging, signing, Homebrew tap setup
- [`docs/CONFIG.md`](CONFIG.md) — full config reference
- [`docs/ARCHITECTURE.md`](ARCHITECTURE.md) — how the binary fits together
- [`docs/AGENTS.md`](AGENTS.md) — adapter protocol for new CLI agents