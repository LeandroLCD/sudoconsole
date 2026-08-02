# sudoconsole

> Secure sudo wrapper for CLI agents. Clean Architecture, multi-platform, policy-based command blocking.

[![Go](https://img.shields.io/badge/Go-1.23+-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![CI](https://github.com/LeandroLCD/sudoconsole/workflows/CI/badge.svg)](.github/workflows/ci.yml)

## What is it?

`sudoconsole` is a CLI utility that lets any AI coding agent (Kilo, Claude Code, Gemini, Aider, Codex, Copilot, ...) execute commands with `sudo` **without ever exposing the password** in logs, history, agent event streams, or disk.

It also enforces a **security policy** that blocks remote-access and credential-exposure commands by default.

## Status

✅ **Install + Policy CLI (M9 + M10)** — `sudoconsole install` orchestrates agent adapters with confirm / dry-run / `--bin-dir` / `--policy-mode` flags. `sudoconsole policy {list,test,show,validate}` lets you inspect and lint the active policy. `exec --policy-override <reason>` records the reason in the audit log.

See [`plans/00-master.md`](plans/00-master.md) for the full roadmap, [`plans/M9-install.md`](plans/M9-install.md) / [`plans/M10-policy-cli.md`](plans/M10-policy-cli.md) for the latest iterations, and [`docs/AGENTS.md`](docs/AGENTS.md) for the adapter protocol.

## Quickstart

```bash
git clone https://github.com/LeandroLCD/sudoconsole
cd sudoconsole
make build

# prime the sudo cache (prompts for the password on a TTY, never echoed)
./bin/sudoconsole auth

# inspect cache status
./bin/sudoconsole check         # human
./bin/sudoconsole --format json check

# run a privileged command under the policy
./bin/sudoconsole exec apt update
./bin/sudoconsole --format json exec systemctl restart nginx

# blocked by default
./bin/sudoconsole exec ssh user@host   # exit 64
```

## Roadmap

| Version | Milestones | Status |
|---------|------------|--------|
| v0.1.0  | M0–M5   (bootstrap, domain, policy, infra, config, CLI) | ✅ shipped |
| v0.2.0  | M6–M10  (audit, agent detector/adapters, install, policy CLI) | ✅ shipped |
| v0.3.0  | M11–M12 (integration tests, hardening) | ✅ shipped |
| v1.0.0  | M13–M14 (release pipeline, docs) | 🚧 in progress |

## Integration tests

The end-to-end suite under [`test/integration/`](test/integration) drives the compiled binary against a real sudo environment in a Docker matrix (Ubuntu 24.04, Debian 12, Fedora 41, Arch) plus a macOS host runner.

```bash
make build
make test-integration   # runs the suite locally; requires sudo + docker
```

The CI workflow at [`.github/workflows/integration.yml`](.github/workflows/integration.yml) runs the full matrix on every PR.

## Releases

Tagged releases (`v0.X.Y`) trigger [`.github/workflows/release.yml`](.github/workflows/release.yml), which uses [goreleaser](https://goreleaser.com) to produce:

- `tar.gz` archives for `linux/amd64`, `linux/arm64`, `darwin/amd64`, `darwin/arm64`;
- `.deb` and `.rpm` packages via nfpm;
- a Homebrew formula at `LeandroLCD/homebrew-tap`;
- a draft GitHub release with auto-generated changelog;
- cosign-signed SHA-256 checksums.

See [`docs/RELEASE.md`](docs/RELEASE.md) for the full procedure (tagging, signing, Homebrew tap setup, dry-run via `make release-dry`).

## Architecture

Clean Architecture in Go, with 4 layers:

```
cmd/sudoconsole/         ← composition root
internal/domain/         ← entities + ports (no external deps)
internal/usecase/        ← use cases (depends only on domain)
internal/infrastructure/ ← adapters (PTY, sudo, cache, config, policy, audit, agents)
internal/transport/cli/  ← cobra commands
```

See [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) for the full design.

## Contributing

See [`CONTRIBUTING.md`](CONTRIBUTING.md) (TODO: M14).

## License

MIT — see [`LICENSE`](LICENSE).