# sudoconsole

> Secure sudo wrapper for CLI agents. Clean Architecture, multi-platform, policy-based command blocking.

[![Go](https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![CI](https://github.com/LeandroLCD/sudoconsole/workflows/CI/badge.svg)](.github/workflows/ci.yml)
[![Integration](https://github.com/LeandroLCD/sudoconsole/workflows/integration/badge.svg)](.github/workflows/integration.yml)
[![Release](https://github.com/LeandroLCD/sudoconsole/workflows/release/badge.svg)](.github/workflows/release.yml)

`sudoconsole` lets any AI coding agent (Kilo, Claude Code, Gemini, Aider, Codex, Copilot, …) execute `sudo` commands **without exposing the password** in logs, history, agent event streams, or `/proc/<pid>/{cmdline,environ}`. It also enforces a security policy that blocks remote-access and credential-exposure commands by default.

- 🔒 **Secret-safe.** Password is fed via PTY stdin (never argv); the credential is heap-allocated and zeroized on use.
- 📜 **Audited.** Every Block/Allow/Warn decision can be appended to a JSONL log.
- 🛡️ **Policy-first.** `blocklist` (default) / `allowlist` / `audit` modes; lint your custom patterns with `policy validate`.
- 🔌 **Adapter-friendly.** `sudoconsole install` registers with 7 CLI agents; add your own via the documented protocol.
- 🚀 **Single static binary.** 4 GOOS/GOARCH pairs shipped via goreleaser + Homebrew tap + .deb/.rpm.

## Quickstart

```bash
git clone https://github.com/LeandroLCD/sudoconsole
cd sudoconsole
make build

# prime the sudo cache — prompts for the password on a TTY, never echoed
./bin/sudoconsole auth

# inspect cache status
./bin/sudoconsole check
./bin/sudoconsole --format json check

# run a privileged command under the policy
./bin/sudoconsole exec apt update

# blocked by default
./bin/sudoconsole exec ssh user@host      # exit 64

# override once, with audit + interactive confirm
./bin/sudoconsole exec --policy-override "debugging" --yes ssh user@host

# install adapters for every detected CLI agent
./bin/sudoconsole install
```

See [`examples/README.md`](examples/README.md) for more recipes.

## Installation

Pick the channel that matches your platform.

```bash
# macOS / Linux — Homebrew tap
brew install LeandroLCD/tap/sudoconsole

# Debian / Ubuntu
curl -LO https://github.com/LeandroLCD/sudoconsole/releases/latest/download/sudoconsole_X.Y.Z_linux_amd64.deb
sudo dpkg -i sudoconsole_X.Y.Z_linux_amd64.deb

# Fedora / RHEL
sudo dnf install https://github.com/LeandroLCD/sudoconsole/releases/latest/download/sudoconsole_X.Y.Z_linux_amd64.rpm

# Static binary — every supported OS
curl -L https://github.com/LeandroLCD/sudoconsole/releases/latest/download/sudoconsole_X.Y.Z_$(uname -s)_$(uname -m).tar.gz | tar -xz
sudo install sudoconsole /usr/local/bin/

# Build from source (requires Go 1.25+)
make build
```

See [`docs/RELEASE.md`](docs/RELEASE.md) for the full release process, including signing/verification with cosign.

## Documentation

| Doc | What it covers |
|-----|---------------|
| [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) | Clean Architecture layout, ports &amp; adapters, why each layer exists |
| [`docs/AGENTS.md`](docs/AGENTS.md) | Adapter protocol; how to add support for a new CLI agent |
| [`docs/CONFIG.md`](docs/CONFIG.md) | Full TOML reference with annotated examples |
| [`docs/SECURITY.md`](docs/SECURITY.md) | Threat model + defense-in-depth (RLIMIT_CORE, redaction, fuzzing) |
| [`docs/RELEASE.md`](docs/RELEASE.md) | Tagging, signing, Homebrew tap, dry-runs, hotfix workflow |
| [`docs/FAQ.md`](docs/FAQ.md) | Common questions + troubleshooting |
| [`manpages/`](manpages/) | `man sudoconsole`, `man sudoconsole-install`, … |
| [`docs/cmd/`](docs/cmd/) | Markdown reference for every subcommand |
| [`examples/`](examples/) | Annotated config + usage recipes |

## CLI cheatsheet

```text
sudoconsole auth               capture the password, prime the sudo cache
sudoconsole check              inspect the cache (exit 0 if valid)
sudoconsole exec <cmd...>      run a privileged command under the policy
sudoconsole config show        print the effective configuration
sudoconsole config validate    lint the TOML before reloading
sudoconsole policy list        table of every binary → category → risk
sudoconsole policy test "<c>"  dry-run a command against the active policy
sudoconsole policy show        dump the effective policy
sudoconsole policy validate    lint every custom pattern (regex/glob/ReDoS)
sudoconsole install            register with every detected CLI agent
sudoconsole uninstall          remove every integration
sudoconsole detect             list detected agents
sudoconsole audit tail         tail the JSONL audit log
sudoconsole version            print build info
```

Every subcommand accepts `--format json` (where the config sets
`[output].format = "json"`) and `--config /path/to.toml`.

## Roadmap

| Version | Milestones | Status |
|---------|------------|--------|
| v0.1.0  | M0–M5   (bootstrap, domain, policy, infra, config, CLI) | ✅ shipped |
| v0.2.0  | M6–M10  (audit, agent detector/adapters, install, policy CLI) | ✅ shipped |
| v0.3.0  | M11–M12 (integration tests, hardening) | ✅ shipped |
| v1.0.0  | M13–M14 (release pipeline, docs) | ✅ shipped |

## Contributing

We follow the [Conventional Commits](https://www.conventionalcommits.org/)
format and require `go vet`, `golangci-lint`, `gosec`, and the
integration matrix to stay green. See [`CONTRIBUTING.md`](CONTRIBUTING.md)
for the dev workflow, conventions, and review checklist.

## Security

Please open a **private** security advisory (GitHub Security tab →
"Report a vulnerability"). Do not file public issues for security
bugs. The full threat model is in [`docs/SECURITY.md`](docs/SECURITY.md).

## License

MIT — see [`LICENSE`](LICENSE).