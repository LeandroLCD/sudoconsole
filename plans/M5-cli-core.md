# M5 — CLI cobra (auth/check/exec)

**Issue**: #6
**Branch**: `feature/m5-cli-core`
**Estimación**: 4h

## Scope
Implementar la interfaz CLI con subcomandos para autenticación, verificación de caché y ejecución de comandos con sudo.

## Tareas

- [ ] **T5.1** `internal/transport/cli/root.go`: comando root con flags globales (`--config`, `--cache-timeout`, `--format`, `--log-level`)
- [ ] **T5.2** `internal/transport/cli/auth_cmd.go`: `sudoconsole auth` — solo autentica, cachea
- [ ] **T5.3** `internal/transport/cli/check_cmd.go`: `sudoconsole check` — exit 0 si caché activa, exit 1 si requiere auth
- [ ] **T5.4** `internal/transport/cli/exec_cmd.go`: `sudoconsole exec <cmd...>` — ejecuta con sudo, refresh caché silencioso si necesario
- [ ] **T5.5** `internal/transport/cli/version_cmd.go`: `sudoconsole version` — info de build (commit, version, go version, OS)
- [ ] **T5.6** Output formatter: human (coloreado con `fatih/color`) y json (struct → json.Marshal)
- [ ] **T5.7** Logger configurable (silent/error/warn/info/debug) via `log/slog`
- [ ] **T5.8** Exit codes consistentes: 0=OK, 1=cache miss, 2=auth fail, 64=policy block, 65=policy warn, 66=override needed
- [ ] **T5.9** Help text completo con ejemplos
- [ ] **T5.10** Wire composition root en `cmd/sudoconsole/main.go` (config → usecases → infra → CLI)

## Acceptance criteria

- `./bin/sudoconsole auth` solicita pass y cachea
- `./bin/sudoconsole check` retorna exit code correcto
- `./bin/sudoconsole exec apt update` funciona end-to-end
- Output `--format json` válido
- `--cache-timeout 0` deshabilita caché (prompt siempre)
- Help text generado por cobra

## Output
PR a `develop`. Cierra #6.