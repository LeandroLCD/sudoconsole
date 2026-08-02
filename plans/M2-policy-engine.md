# M2 — Policy engine (core)

**Issue**: #3
**Branch**: `feature/m2-policy-engine`
**Estimación**: 5h

## Scope
Implementar el motor de evaluación de políticas: matching de patrones, categorización de comandos, decisión (allow/warn/block/audit).

## Tareas

- [ ] **T2.1** `internal/infrastructure/policy/categories.go`: mapa bin→categoría con ~80 comandos (apt, ssh, nc, bash, curl, passwd, visudo, etc.)
- [ ] **T2.2** `internal/infrastructure/policy/matcher.go`: matcher que combina glob + regex sobre línea completa
- [ ] **T2.3** `internal/infrastructure/policy/evaluator.go`: implementa `domain.PolicyEvaluator`
- [ ] **T2.4** `internal/usecase/evaluate_policy.go`: orquesta categorización + matching + decisión final
- [ ] **T2.5** `internal/usecase/execute.go`: integra policy en el flujo de ejecución (rechaza antes de llamar SudoGateway)
- [ ] **T2.6** Validación de patterns en carga de config: rechaza regex ReDoS (límite de longitud, `?{N,M}` con N>3)
- [ ] **T2.7** Modo dry-run en `PolicyEvaluator.Evaluate()` que retorna decisión sin ejecutar
- [ ] **T2.8** Tests:
  - Bloqueo de `ssh -R`, `nc -e`, `bash -c '...'`, `tee /etc/shadow`, `python -c "..."`
  - Allow de `apt`, `systemctl`, `tee /var/log/app.log`
  - Categorización correcta de bins ambiguos (`curl` = medium, `passwd` = high)
  - Override flag funciona
  - Pattern custom inválido rechazado
  - ReDoS pattern rechazado

## Acceptance criteria

- 100% de los comandos de la lista negra del plan maestro correctamente bloqueados
- 0 falsos positivos en `apt update`, `systemctl restart nginx`, `tee /tmp/file`
- Override flag documentado y testeado
- Pattern validator rechaza ReDoS

## Output
PR a `develop`. Cierra #3.