# M6 — Audit logger

**Issue**: #7
**Branch**: `feature/m6-audit`
**Estimación**: 2h

## Scope
Logger append-only de todas las decisiones de policy y comandos ejecutados, para auditoría forense.

## Tareas

- [ ] **T6.1** `internal/infrastructure/audit/logger.go`: implementa `domain.AuditLogger`
- [ ] **T6.2** Formato de entrada JSONL: `{ts, user, hostname, decision, command, categories, policy_hash, session_id}`
- [ ] **T6.3** Archivo append-only con `chmod 0600` y `O_APPEND` flag
- [ ] **T6.4** Rotación por tamaño (10MB) con timestamp suffix
- [ ] **T6.5** Redacción automática: passwords en commands (`sudo -S` con pass en stdin se redacta)
- [ ] **T6.6** Comando `sudoconsole audit tail` para ver últimas N entradas
- [ ] **T6.7** Tests con fixtures de logs

## Acceptance criteria

- Cada ejecución genera una línea JSONL
- Archivo append-only (no se borra accidentalmente)
- Rotación funciona sin pérdida de entradas
- `audit tail` lee últimas N
- Redacción: comando `sudo -S` con pass en pipe aparece como `sudo -S <REDACTED>`

## Output
PR a `develop`. Cierra #7.