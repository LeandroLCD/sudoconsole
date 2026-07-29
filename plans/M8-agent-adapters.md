# M8 — CLI agent adapters (7 implementaciones)

**Issue**: #9
**Branch**: `feature/m8-agent-adapters`
**Estimación**: 6h

## Scope
Implementar la interfaz `AgentInstaller` para cada CLI agent soportado.

## Tareas

- [ ] **T8.1** `internal/infrastructure/agent/kilo.go`: instala slash-command en `~/.config/kilo/commands/sudoconsole.md` + symlink `~/bin/sudosafe`
- [ ] **T8.2** `internal/infrastructure/agent/claude.go`: instala `~/.claude/commands/sudoconsole.md` + entry en `settings.json`
- [ ] **T8.3** `internal/infrastructure/agent/gemini.go`: instala `~/.gemini/tools/sudoconsole.toml`
- [ ] **T8.4** `internal/infrastructure/agent/aider.go`: patch a `~/.aider.conf.yml` con alias
- [ ] **T8.5** `internal/infrastructure/agent/codex.go`: instala `~/.codex/sudosafe.toml`
- [ ] **T8.6** `internal/infrastructure/agent/copilot.go`: `gh alias set sudosafe 'sudoconsole'`
- [ ] **T8.7** `internal/infrastructure/agent/generic.go`: agrega `alias sudosafe="sudoconsole"` a `~/.bashrc`/`~/.zshrc`
- [ ] **T8.8** Cada adapter implementa: `Name()`, `Detect()`, `Install()`, `Uninstall()`, `AdapterCommand()`
- [ ] **T8.9** Tests contractuales por adapter: schema de config válido, install idempotente, uninstall limpio
- [ ] **T8.10** Documentación en `docs/AGENTS.md` con ejemplo por adapter

## Acceptance criteria

- 7 adapters funcionando
- Install es idempotente (segunda ejecución no duplica)
- Uninstall limpia todo
- Cada adapter tiene test contractual
- Documentación de protocolo en `docs/AGENTS.md`

## Output
PR a `develop`. Cierra #9.