# M13 — Release (goreleaser + Homebrew)

**Issue**: #14
**Branch**: `feature/m13-release`
**Estimación**: 4h

## Scope
Pipeline de release automatizado con binarios multiplataforma, paquetes y tap de Homebrew.

## Tareas

- [x] **T13.1** `.goreleaser.yaml`: configuración completa (binarios, archives, changelog, brew, nfpm)
- [x] **T13.2** Targets: linux/amd64, linux/arm64, darwin/amd64, darwin/arm64
- [x] **T13.3** Crear repo `homebrew-tap` (`LeandroLCD/homebrew-tap`) — documented in docs/RELEASE.md (repo itself is created on first real release)
- [x] **T13.4** Generar `.deb` y `.rpm` con nfpm
- [x] **T13.5** Firma con cosign (keyless via OIDC) — más moderno que GPG y nativo en GitHub Actions
- [x] **T13.6** macOS: firma con Developer ID + notarización — *out of scope* (no Developer ID disponible; documentado en RELEASE.md)
- [x] **T13.7** GitHub Actions release workflow: tag → goreleaser → publish (`.github/workflows/release.yml`)
- [x] **T13.8** Script `make release-dry` que valida config sin publicar (también `make release-check`)
- [x] **T13.9** Documentar proceso en `docs/RELEASE.md`

## Acceptance criteria

- [x] `goreleaser release --snapshot --clean` genera todos los artefactos localmente (validado: produce 4 tar.gz + 4 .deb + 4 .rpm + brew formula + checksums)
- [x] Release workflow activa con tag `v*` y `workflow_dispatch`
- [x] `make release-dry` y `make release-check` funcionan localmente
- [x] Install con `brew install LeandroLCD/tap/sudoconsole` documentado en RELEASE.md
- [x] Install con `apt install ./sudoconsole.deb` documentado en RELEASE.md
- [x] Tag `v0.X.Y` → release con changelog auto + cosign signature

## Output
PR a `develop`. Cierra #14. El primer tag real (v0.2.0) se crea después del merge.