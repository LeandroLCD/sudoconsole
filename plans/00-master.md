# SudoConsole — Master Plan

> Secure sudo wrapper for CLI agents. Clean Architecture, multi-platform (Linux + macOS), policy-based command blocking.

**Repository**: https://github.com/LeandroLCD/sudoconsole (private)
**Branching**: Git Flow — `main` (stable), `develop` (integration), `feature/*` (per issue)
**Owner**: LeandroLCD
**Created**: 2026-07-29
**Last update**: 2026-07-29

---

## 1. Visión

Una utilidad CLI que cualquier agente (Kilo, Claude Code, Gemini, Aider, Codex, Copilot, etc.) pueda invocar para ejecutar comandos con `sudo` cuando sea necesario, **sin exponer la contraseña** en logs, history, events de la CLI ni en disco. Incluye motor de **policy** que bloquea comandos de exposición remota o de credenciales por defecto.

## 2. Objetivos

| # | Objetivo | Métrica de éxito |
|---|----------|------------------|
| O1 | Password nunca expuesta en ningún log | `grep -r PASS db` retorna 0 |
| O2 | Soporte Linux + macOS desde día 1 | CI matrix verde en ambos |
| O3 | Cualquier CLI agent puede integrarse en <5 min | `sudoconsole install` detecta + instala adapters |
| O4 | Policy engine bloquea remote access + credential exposure | Tests cubren `ssh`, `nc`, `bash`, `passwd`, etc. |
| O5 | Timeout de caché configurable | Flag `--cache-timeout` + TOML funcional |
| O6 | Clean Architecture mantenible | `go-arch-lint` valida dependencias entre capas |

## 3. Stack técnico

- **Go 1.22+**, single static binary
- **cobra** (CLI), **creack/pty** (TTY), **pelletier/go-toml/v2** (config)
- **goreleaser** (binarios, Homebrew tap, .deb, .rpm)
- **golangci-lint** + **gosec** (calidad y seguridad)
- **GitHub Actions** (CI matrix linux/darwin × amd64/arm64)

## 4. Capas (Clean Architecture)

```
domain/         ← entidades + ports (cero deps externas)
usecase/        ← casos de uso (solo depende de domain)
infrastructure/ ← impl de ports (PTY, sudo, cache, config, policy, audit, agents)
transport/cli/  ← comandos cobra (composition root)
cmd/            ← main.go (wiring)
```

Reglas validadas con `go-arch-lint`:
- `domain` no importa nada fuera de stdlib
- `usecase` solo importa `domain`
- `infrastructure` solo importa `domain` + libs externos
- `transport/cli` importa todo (composition root)

## 5. Hitos

Cada hito es una **feature branch** + **issue** + **PR a develop**. Detalle de tareas en archivos individuales (`M0-*.md`, `M1-*.md`, ...).

| # | Hito | Branch | Issue | Plan | Estimación |
|---|------|--------|-------|------|------------|
| **M0** | Bootstrap repo + CI | `feature/m0-bootstrap` | #1 | [plans/M0-bootstrap.md](M0-bootstrap.md) | 3h |
| **M1** | Domain layer (entities + ports) | `feature/m1-domain` | #2 | [plans/M1-domain.md](M1-domain.md) | 4h |
| **M2** | Policy engine (core) | `feature/m2-policy-engine` | #3 | [plans/M2-policy-engine.md](M2-policy-engine.md) | 5h |
| **M3** | Infrastructure: PTY + sudo + cache | `feature/m3-infra-core` | #4 | [plans/M3-infra-core.md](M3-infra-core.md) | 6h |
| **M4** | Config TOML + loader | `feature/m4-config` | #5 | [plans/M4-config.md](M4-config.md) | 2h |
| **M5** | CLI cobra (auth/check/exec) | `feature/m5-cli-core` | #6 | [plans/M5-cli-core.md](M5-cli-core.md) | 4h |
| **M6** | Audit logger | `feature/m6-audit` | #7 | [plans/M6-audit.md](M6-audit.md) | 2h |
| **M7** | CLI agent detector | `feature/m7-agent-detector` | #8 | [plans/M7-agent-detector.md](M7-agent-detector.md) | 4h |
| **M8** | CLI agent adapters (7 impls) | `feature/m8-agent-adapters` | #9 | [plans/M8-agent-adapters.md](M8-agent-adapters.md) | 6h |
| **M9** | Install/uninstall commands | `feature/m9-install` | #10 | [plans/M9-install.md](M9-install.md) | 3h |
| **M10** | Policy commands + override | `feature/m10-policy-cli` | #11 | [plans/M10-policy-cli.md](M10-policy-cli.md) | 3h |
| **M11** | Integration tests (Docker matrix) | `feature/m11-integration-tests` | #12 | [plans/M11-integration-tests.md](M11-integration-tests.md) | 4h |
| **M12** | Security hardening + fuzz | `feature/m12-hardening` | #13 | [plans/M12-hardening.md](M12-hardening.md) | 4h |
| **M13** | Release (goreleaser + Homebrew) | `feature/m13-release` | #14 | [plans/M13-release.md](M13-release.md) | 4h |
| **M14** | Documentación + sitio | `feature/m14-docs` | #15 | [plans/M14-docs.md](M14-docs.md) | 3h |

**Total estimado**: ~57h

## 6. Flujo de trabajo (Git Flow)

```
main         ← releases estables (tags semver)
  ↑
develop      ← integración continua
  ↑
feature/*    ← una por hito, PR → develop
  ↑
hotfix/*     ← fixes urgentes desde main, PR → main + cherry a develop
```

**Convenciones**:
- Branch: `feature/mN-<slug-kebab>`
- Commit: Conventional Commits (`feat:`, `fix:`, `chore:`, `docs:`, `test:`, `refactor:`)
- PR title: `[M#] Descripción corta`
- PR body: `Closes #N` + checklist de aceptación del hito
- Merge a `develop` con **squash**, mensaje = título del PR
- Release a `main`: PR `develop` → `main` con tag semver (`v0.1.0` → `v0.x.0` → `v1.0.0`)

## 7. Criterios de "Done" por hito

1. Código mergeado a `develop` vía PR aprobado
2. CI verde: `go test ./...`, `golangci-lint run`, `gosec ./...`
3. Coverage ≥80% en domain/usecase
4. Documentación actualizada en `README.md` o `docs/`
5. Issue original cerrado con `Closes #N`

## 8. Riesgos

| Riesgo | Mitigación |
|--------|------------|
| Diferencias en formato de timestamp sudo entre distros | M3 + M11 con matriz Docker |
| Cada CLI agent cambia su config schema | M8 con tests contractuales por adapter |
| PTY allocation falla en containers | Flag `--no-tty` con fallback |
| macOS Gatekeeper bloquea binario | M13 con firma Developer ID + notarización |
| ReDoS en patterns custom | M12 con validador + límites |

## 9. Roadmap de releases

- **v0.1.0** (MVP): M0–M5 → CLI funcional con auth/check/exec y policy por defecto
- **v0.2.0**: M6–M10 → audit + detección/instalación de agents + policy CLI
- **v0.3.0**: M11–M12 → tests integración + hardening
- **v1.0.0**: M13–M14 → release distribuido + docs públicas