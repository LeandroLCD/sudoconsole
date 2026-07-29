# M3 — Infrastructure: PTY + sudo + cache

**Issue**: #4
**Branch**: `feature/m3-infra-core`
**Estimación**: 6h

## Scope
Implementaciones concretas de los ports del domain para interactuar con el sistema operativo: PTY allocation, invocación de sudo, lectura del timestamp cache.

## Tareas

- [ ] **T3.1** `internal/infrastructure/pty/pty_unix.go`: wrapper sobre `creack/pty` con build tag `//go:build linux || darwin`
- [ ] **T3.2** `internal/infrastructure/pty/prompt.go`: helper para prompt silencioso (`stty -echo` temporal, `read -rs` o equivalente Go)
- [ ] **T3.3** `internal/infrastructure/pty/resize.go`: manejo de SIGWINCH para propagar resize al PTY
- [ ] **T3.4** `internal/infrastructure/sudo/sudowrap.go`: implementa `domain.SudoGateway` — envuelve `sudo` con `-S -p ""` o usa PTY
- [ ] **T3.5** `internal/infrastructure/sudo/auth.go`: `Authenticate(ctx) error` que abre PTY, lee pass silenciosa, valida
- [ ] **T3.6** `internal/infrastructure/sudo/exec.go`: `Execute(ctx, cmd) (stdout, stderr, exitCode, error)` post-autenticación
- [ ] **T3.7** `internal/infrastructure/cache/timestamp_linux.go`: lee `/var/db/sudo/ts/<encoded-user>` y calcula edad
- [ ] **T3.8** `internal/infrastructure/cache/timestamp_darwin.go`: lee `/var/run/sudo/ts/<encoded-user>` con formato BSD
- [ ] **T3.9** `internal/infrastructure/cache/cache.go`: implementa `domain.CacheRepository.IsActive()`, `Refresh()`, `TimeRemaining()`
- [ ] **T3.10** Tests con mocks de filesystem (`testing/fstest`) + tests integración en Docker (Ubuntu, Debian, Fedora, macOS-latest)

## Acceptance criteria

- PTY funciona en Linux (probado en Ubuntu 24.04, Fedora 41) y macOS-latest
- Password no aparece en `ps aux`, `dmesg`, ni en el comando ejecutado
- Cache lee correctamente los 2 formatos (Linux ext4 + macOS BSD)
- SIGWINCH propagado
- `sudo -v` refresca caché silenciosamente

## Output
PR a `develop`. Cierra #4.