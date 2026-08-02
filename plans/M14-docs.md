# M14 — Documentación + sitio

**Issue**: #15
**Branch**: `feature/m14-docs`
**Estimación**: 3h

## Scope
Documentación completa para usuarios finales y contribuidores.

## Tareas

- [x] **T14.1** `README.md` completo: intro, badges, quickstart, install, usage, examples
- [x] **T14.2** `docs/ARCHITECTURE.md`: diagrama de capas + explicación Clean Architecture
- [x] **T14.3** `docs/AGENTS.md`: protocolo de adapters + lista de CLIs soportados
- [x] **T14.4** `docs/CONFIG.md`: referencia completa del TOML con ejemplos
- [x] **T14.5** `docs/SECURITY.md`: modelo de amenazas + tabla de mitigaciones
- [x] **T14.6** `docs/RELEASE.md`: proceso de release
- [x] **T14.7** `CONTRIBUTING.md`: cómo contribuir, convenciones, dev setup
- [x] **T14.8** `docs/FAQ.md`: preguntas frecuentes
- [x] **T14.9** Ejemplos en `examples/`: uso básico, uso con Kilo, uso con Claude Code
- [x] **T14.10** Generar manpage con `cobra doc` (`cmd/docgen/main.go` + `make docgen`)

## Acceptance criteria

- README con quickstart funcional (<5min desde clone hasta primer uso) ✅
- ARCHITECTURE, AGENTS, CONFIG, SECURITY, RELEASE, CONTRIBUTING, FAQ completos ✅
- Ejemplos copy-paste ready (`examples/config.toml`, `examples/README.md`) ✅
- Manpage generada (`manpages/sudoconsole.1` + 18 subcomandos) ✅

## Output
PR a `develop`. Cierra #15.

---

# Release v1.0.0

Una vez M0–M14 mergeados y tested, crear PR `develop` → `main` con tag `v1.0.0` siguiendo convenciones de M13.