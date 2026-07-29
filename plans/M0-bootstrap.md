# M0 — Bootstrap repo + CI

**Issue**: #1
**Branch**: `feature/m0-bootstrap`
**Estimación**: 3h

## Scope
Inicializar estructura del proyecto Go con Clean Architecture, configurar CI, agregar archivos base.

## Tareas

- [ ] **T0.1** Crear `go.mod` con Go 1.22, dependencias: cobra, creack/pty, pelletier/go-toml/v2, testify
- [ ] **T0.2** Crear estructura de carpetas: `cmd/sudoconsole/`, `internal/{domain,usecase,infrastructure,transport}/`
- [ ] **T0.3** Crear `cmd/sudoconsole/main.go` mínimo que imprima versión
- [ ] **T0.4** Crear `Makefile` con targets: `build`, `test`, `lint`, `coverage`, `release`, `clean`
- [ ] **T0.5** Crear `.golangci.yml` con linters: govet, staticcheck, gosec, revive, gocyclo, misspell
- [ ] **T0.6** Crear `.gitignore` (binarios, vendor, coverage, .env, *.log)
- [ ] **T0.7** Crear `LICENSE` (MIT) + `README.md` con intro y badges
- [ ] **T0.8** Crear `.github/workflows/ci.yml` con matrix Go 1.22/1.23 × linux/darwin
- [ ] **T0.9** Crear `.github/ISSUE_TEMPLATE/` (bug, feature, task)
- [ ] **T0.10** Crear `docs/ARCHITECTURE.md` con diagrama de capas
- [ ] **T0.11** Tag del repo: topics `go`, `sudo`, `security`, `cli-agent`, `clean-architecture`

## Acceptance criteria

- `make build` produce binario `bin/sudoconsole`
- `./bin/sudoconsole version` imprime versión
- CI corre en PR y reporta status
- Lint pasa sin warnings
- Estructura de carpetas documentada en ARCHITECTURE.md

## Output
PR a `develop` con todos los archivos. Cierra #1.