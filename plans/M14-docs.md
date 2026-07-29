# M14 — Documentación + sitio

**Issue**: #15
**Branch**: `feature/m14-docs`
**Estimación**: 3h

## Scope
Documentación completa para usuarios finales y contribuidores.

## Tareas

- [ ] **T14.1** `README.md` completo: intro, badges, quickstart, install, usage, examples
- [ ] **T14.2** `docs/ARCHITECTURE.md`: diagrama de capas + explicación Clean Architecture
- [ ] **T14.3** `docs/AGENTS.md`: protocolo de adapters + lista de CLIs soportados
- [ ] **T14.4** `docs/CONFIG.md`: referencia completa del TOML con ejemplos
- [ ] **T14.5** `docs/SECURITY.md`: modelo de amenazas + tabla de mitigaciones
- [ ] **T14.6** `docs/RELEASE.md`: proceso de release
- [ ] **T14.7** `CONTRIBUTING.md`: cómo contribuir, convenciones, dev setup
- [ ] **T14.8** `docs/FAQ.md`: preguntas frecuentes
- [ ] **T14.9** Ejemplos en `examples/`: uso básico, uso con Kilo, uso con Claude Code
- [ ] **T14.10** Generar manpage con `cobra doc`

## Acceptance criteria

- README con quickstart funcional (<5min desde clone hasta primer uso)
- ARCHITECTURE, AGENTS, CONFIG, SECURITY, RELEASE, CONTRIBUTING, FAQ completos
- Ejemplos copy-paste ready
- Manpage generada

## Output
PR a `develop`. Cierra #15.

---

# Release v1.0.0

Una vez M0–M14 mergeados y tested, crear PR `develop` → `main` con tag `v1.0.0` siguiendo convenciones de M13.