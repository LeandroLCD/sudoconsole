# Changelog

All notable changes to **sudoconsole** are documented in this file.
The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- `sudoconsole install` / `sudoconsole uninstall` for registering with
  every detected CLI agent (Kilo, Claude Code, Gemini, Aider, Codex,
  Copilot). Supports `--kind`, `--force`, `--dry-run`, `--yes`,
  `--bin-dir`, `--policy-mode`.
- `sudoconsole policy {list,test,show,validate}` for inspecting and
  linting the active policy. `validate` exits non-zero on bad regex /
  glob / ReDoS patterns.
- `exec --policy-override "<reason>"` records the supplied reason in
  the audit log; requires interactive confirm unless `--yes`.
- Release pipeline: `goreleaser` config builds 4 GOOS/GOARCH pairs,
  produces `.deb` / `.rpm` packages via nfpm, and updates the
  `LeandroLCD/homebrew-tap` formula.
- Integration test suite (`go test -tags=integration`) runs the
  compiled binary against a real sudo environment in a Docker matrix
  (Ubuntu 24.04, Debian 12, Fedora 41, Arch) and on macOS.
- Security hardening: `Credential` type with `Zeroize()`, RLIMIT_CORE=0
  on the parent before `sudo -v`, audit-log redaction of `--password=`
  substrings.
- Threat model documented in `docs/SECURITY.md`.

### Notes
- Initial public release. The next version will be tagged `v0.1.0`.