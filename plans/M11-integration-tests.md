# M11 — Integration tests (Docker matrix)

**Issue**: #12
**Branch**: `feature/m11-integration-tests`
**Estimación**: 4h

## Scope
Tests E2E con sudo real en múltiples distros para validar comportamiento OS-específico.

## Tareas

- [x] **T11.1** `test/integration/Dockerfile.ubuntu`: imagen base Ubuntu 24.04 con sudo + pass preconfigurada
- [x] **T11.2** `test/integration/Dockerfile.debian`: Debian 12
- [x] **T11.3** `test/integration/Dockerfile.fedora`: Fedora 41
- [x] **T11.4** `test/integration/Dockerfile.arch`: Arch Linux latest
- [x] **T11.5** `test/integration/integration_test.go`: tests E2E (build tag `integration`)
- [x] **T11.6** `test/integration/macos_test.go`: equivalente para macOS
- [x] **T11.7** Tests cubren:
  - Auth con pass correcta / incorrecta
  - Cache hit / miss / expired
  - PTY prompt silencioso
  - Policy block ssh/nc/bash
  - Override con audit
  - Multi-comando en sesión cacheada
- [x] **T11.8** GitHub Actions: matrix job que build Docker, corre tests

## Acceptance criteria

- Tests verdes en Ubuntu 24.04, Debian 12, Fedora 41, Arch Linux
- macOS test verde en `macos-latest`
- Cobertura: auth, cache, policy, override, audit
- Tiempo total de suite <5min

## Output
PR a `develop`. Cierra #12.