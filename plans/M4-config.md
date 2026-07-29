# M4 — Config TOML + loader

**Issue**: #5
**Branch**: `feature/m4-config`
**Estimación**: 2h

## Scope
Cargar configuración desde TOML en XDG dir (Linux) o Application Support (macOS), con override por flags CLI.

## Tareas

- [ ] **T4.1** `internal/infrastructure/config/schema.go`: structs Go reflejando el TOML (`Config`, `CacheConfig`, `SecurityConfig`, `PolicyConfig`, `AgentConfig`, `OutputConfig`)
- [ ] **T4.2** `internal/infrastructure/config/loader.go`: implementa `domain.ConfigStore.Load(path)` con pelletier/go-toml/v2
- [ ] **T4.3** `internal/infrastructure/config/paths.go`: retorna XDG_CONFIG_HOME o fallback (`~/Library/Application Support/sudoconsole` en mac)
- [ ] **T4.4** `internal/infrastructure/config/defaults.go`: `DefaultConfig()` con valores seguros (mode=blocklist, timeout=900, refresh=120)
- [ ] **T4.5** `internal/infrastructure/config/validate.go`: valida config al cargar (regex válida, paths existen, rangos OK)
- [ ] **T4.6** Override por flag CLI en `transport/cli`: `--cache-timeout`, `--config`, `--format`, `--log-level`
- [ ] **T4.7** Tests con fixtures TOML válidos e inválidos

## Acceptance criteria

- `~/.config/sudoconsole/config.toml` se carga si existe
- Defaults aplicados si no existe
- Override por flag tiene precedencia
- Config inválida retorna error claro (línea + campo)
- Funciona idéntico en Linux y macOS (path detection correcto)

## Output
PR a `develop`. Cierra #5.