# M12 — Security hardening + fuzz

**Issue**: #13
**Branch**: `feature/m12-hardening`
**Estimación**: 4h

## Scope
Audit de seguridad, fuzzing del parser de policy, validación de memoria y core dumps.

## Tareas

- [x] **T12.1** Correr `gosec ./...` y resolver todos los issues (G101-G404)
- [x] **T12.2** Fuzz test del pattern matcher (`testing.F` + corpus)
- [x] **T12.3** Fuzz test del parser de TOML
- [x] **T12.4** Validar `credential.Zeroize()` en tests (verificar que la dirección de memoria cambia)
- [x] **T12.5** Validar que `RLIMIT_CORE=0` se aplica durante autenticación
- [x] **T12.6** Pen-test manual: intentar leak de pass vía `ps`, `/proc/<pid>/environ`, signals, ptracer, etc.
- [x] **T12.7** Agregar tests que verifican que password NO aparece en:
  - comando ejecutado (`ps aux`)
  - env del proceso (`/proc/<pid>/environ`)
  - logs de sudo (`journalctl`)
  - output de `dmesg`
- [x] **T12.8** Documentar modelo de seguridad en `docs/SECURITY.md` con tabla de amenazas + mitigaciones

## Acceptance criteria

- `gosec` 0 issues
- Fuzz tests corren 1M iteraciones sin panic
- Pen-test manual: 0 leaks encontrados
- `docs/SECURITY.md` completo

## Output
PR a `develop`. Cierra #13.