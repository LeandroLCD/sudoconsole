# M1 — Domain layer (entities + ports)

**Issue**: #2
**Branch**: `feature/m1-domain`
**Estimación**: 4h

## Scope
Definir entidades de dominio puras (sin dependencias externas) y contratos (ports) que las capas inferiores implementarán.

## Tareas

- [ ] **T1.1** `internal/domain/credential.go`: `Credential` struct con método `Zeroize()` para purga de memoria
- [ ] **T1.2** `internal/domain/command.go`: `Command` entity (path, args, raw, env)
- [ ] **T1.3** `internal/domain/cache.go`: `CacheStatus` (Active, Expired, Unknown) + `CacheConfig` (timeout, refresh_before)
- [ ] **T1.4** `internal/domain/policy.go`: `Policy` entity (mode, blocked, allowed, audit, override) + `Decision` enum (Allow, Warn, Block, Audit)
- [ ] **T1.5** `internal/domain/agent.go`: `AgentDescriptor` (name, path, version, config_dir)
- [ ] **T1.6** `internal/domain/ports.go`: interfaces `SudoGateway`, `PtyGateway`, `CacheRepository`, `PolicyEvaluator`, `AuditLogger`, `AgentInstaller`, `ConfigStore`, `AgentDetector`
- [ ] **T1.7** `internal/domain/errors.go`: errores tipados (`ErrAuthFailed`, `ErrPolicyBlocked`, `ErrCacheMiss`, etc.)
- [ ] **T1.8** Tests unitarios por cada entity (table-driven) — coverage 100% domain

## Acceptance criteria

- `go test ./internal/domain/...` verde
- Coverage 100% en domain
- `go-arch-lint` (o regla manual) confirma cero imports fuera de stdlib
- Sin uso de `panic`, solo errores retornados
- Sin dependencias de librerías externas en este paquete

## Output
PR a `develop`. Cierra #2.