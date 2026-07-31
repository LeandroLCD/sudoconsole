# sudoconsole

> Secure sudo wrapper for CLI agents. Clean Architecture, multi-platform, policy-based command blocking.

[![Go](https://img.shields.io/badge/Go-1.23+-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![CI](https://github.com/LeandroLCD/sudoconsole/workflows/CI/badge.svg)](.github/workflows/ci.yml)

## What is it?

`sudoconsole` is a CLI utility that lets any AI coding agent (Kilo, Claude Code, Gemini, Aider, Codex, Copilot, ...) execute commands with `sudo` **without ever exposing the password** in logs, history, agent event streams, or disk.

It also enforces a **security policy** that blocks remote-access and credential-exposure commands by default.

## Status

🚧 **CLI core (M5)** — `sudoconsole auth|check|exec|version|config` are wired end-to-end with policy enforcement, audit logging and human/JSON output.

See [`plans/00-master.md`](plans/00-master.md) for the full roadmap and [`plans/M5-cli-core.md`](plans/M5-cli-core.md) for the current iteration.

## Quickstart (functional in M5)

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
| v0.1.0  | M0–M5   (bootstrap, domain, policy, infra, config, CLI) | in progress |
| v0.2.0  | M6–M10  (audit, agent detector/adapters, install, policy CLI) | planned |
| v0.3.0  | M11–M12 (integration tests, hardening) | planned |
| v1.0.0  | M13–M14 (release pipeline, docs) | planned |

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