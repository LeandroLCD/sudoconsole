# M7 — CLI agent detector

**Issue**: #8
**Branch**: `feature/m7-agent-detector`
**Estimación**: 4h

## Scope
Detectar qué CLI agents están instalados en el sistema para luego instalar adapters.

## Tareas

- [ ] **T7.1** `internal/infrastructure/agent/detector.go`: implementa `domain.AgentDetector`
- [ ] **T7.2** Estrategia de detección:
  - Binarios en PATH (`which kilo`, `which claude`, ...)
  - Config dirs presentes (`~/.config/kilo/`, `~/.claude/`, ...)
  - Marcador propio (`~/.config/sudoconsole/installed-agents.toml`)
- [ ] **T7.3** Definir lista de agents detectables: kilo, claude, gemini, aider, codex, copilot, generic
- [ ] **T7.4** Detección paralela con `errgroup` (timeout 2s total)
- [ ] **T7.5** Output: `[]AgentDescriptor{Name, Version, Path, ConfigDir}`
- [ ] **T7.6** Comando `sudoconsole detect` con `--json` para parseo
- [ ] **T7.7** Tests con mocks de filesystem (`testing/fstest`) y PATH fake

## Acceptance criteria

- Detecta Kilo, Claude Code, Gemini, Aider, Codex, Copilot si están instalados
- Output `--json` válido
- Detección completa en <500ms
- No falla si un agent no está, continúa con los demás

## Output
PR a `develop`. Cierra #8.