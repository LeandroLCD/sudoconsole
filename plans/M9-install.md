# M9 — Install/uninstall commands

**Issue**: #10
**Branch**: `feature/m9-install`
**Estimación**: 3h

## Scope
Comandos de alto nivel que orquestan el detector + los adapters.

## Tareas

- [ ] **T9.1** `internal/transport/cli/install_cmd.go`: `sudoconsole install [--agent NAME]... [--force]`
- [ ] **T9.2** `internal/transport/cli/uninstall_cmd.go`: `sudoconsole uninstall [--agent NAME]...`
- [ ] **T9.3** `internal/transport/cli/detect_cmd.go`: `sudoconsole detect [--json]`
- [ ] **T9.4** `internal/usecase/install_adapter.go`: orquesta detector → adapters
- [ ] **T9.5** Flag `--policy-mode` para que install respete la policy configurada
- [ ] **T9.6** Flag `--bin-dir` para override de `~/bin`
- [ ] **T9.7** Output tabular en human, JSON en `--format json`
- [ ] **T9.8** Confirmación interactiva antes de instalar (a menos que `--yes`)
- [ ] **T9.9** Tests E2E con filesystem fake

## Acceptance criteria

- `sudoconsole install` detecta e instala todos los adapters
- `sudoconsole install --agent kilo` solo Kilo
- `sudoconsole uninstall --agent claude` limpia
- Confirmación interactiva funciona
- Idempotencia validada

## Output
PR a `develop`. Cierra #10.