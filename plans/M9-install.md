# M9 — Install/uninstall commands

**Issue**: #10
**Branch**: `feature/m9-install`
**Estimación**: 3h

## Scope
Comandos de alto nivel que orquestan el detector + los adapters.

## Tareas

- [x] **T9.1** `internal/transport/cli/install_cmd.go`: `sudoconsole install [--kind NAME]... [--force]`
- [x] **T9.2** `internal/transport/cli/install_cmd.go` (`newUninstallCmd`): `sudoconsole uninstall [--kind NAME]...`
- [x] **T9.3** `internal/transport/cli/detect_cmd.go`: `sudoconsole detect [--format json]`
- [x] **T9.4** `internal/usecase/install_adapter.go`: orquesta detector → adapters
- [x] **T9.5** Flag `--policy-mode` para que install respete la policy configurada
- [x] **T9.6** Flag `--bin-dir` para override de `~/bin`
- [x] **T9.7** Output tabular en human, JSON en `--format json`
- [x] **T9.8** Confirmación interactiva antes de instalar (a menos que `--yes`)
- [x] **T9.9** Tests E2E con filesystem fake

## Acceptance criteria

- `sudoconsole install` detecta e instala todos los adapters
- `sudoconsole install --kind kilo` solo Kilo
- `sudoconsole uninstall --kind claude` limpia
- Confirmación interactiva funciona
- Idempotencia validada

## Output
PR a `develop`. Cierra #10.