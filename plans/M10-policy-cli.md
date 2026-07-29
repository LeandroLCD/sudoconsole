# M10 — Policy commands + override

**Issue**: #11
**Branch**: `feature/m10-policy-cli`
**Estimación**: 3h

## Scope
Comandos para inspeccionar y testear la policy antes de ejecutar.

## Tareas

- [ ] **T10.1** `internal/transport/cli/policy_cmd.go`: comando padre `sudoconsole policy`
- [ ] **T10.2** `sudoconsole policy list`: tabla de categorías y comandos clasificados (riesgo, categoría)
- [ ] **T10.3** `sudoconsole policy test "<cmd>"`: dry-run que retorna decisión (allow/warn/block) + razón
- [ ] **T10.4** `sudoconsole policy show`: dump de la config efectiva (defaults + user + flags)
- [ ] **T10.5** `sudoconsole policy validate`: chequea que los patterns custom sean regex válidas y no ReDoS
- [ ] **T10.6** Flag `--policy-override allow` en `exec`: pide confirmación + audit obligatorio
- [ ] **T10.7** Output en `--format json` para parseo por agentes
- [ ] **T10.8** Tests con fixtures de policies

## Acceptance criteria

- `policy test "ssh user@host"` retorna blocked
- `policy test "apt update"` retorna allowed
- `policy show` muestra config efectiva
- `policy validate` rechaza pattern ReDoS
- Override funciona con confirmación + audit

## Output
PR a `develop`. Cierra #11.