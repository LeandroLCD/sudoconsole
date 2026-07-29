# M13 — Release (goreleaser + Homebrew)

**Issue**: #14
**Branch**: `feature/m13-release`
**Estimación**: 4h

## Scope
Pipeline de release automatizado con binarios multiplataforma, paquetes y tap de Homebrew.

## Tareas

- [ ] **T13.1** `.goreleaser.yaml`: configuración completa (binarios, archives, changelog, brew, nfpm)
- [ ] **T13.2** Targets: linux/amd64, linux/arm64, darwin/amd64, darwin/arm64
- [ ] **T13.3** Crear repo `homebrew-tap` (`LeandroLCD/homebrew-tap`)
- [ ] **T13.4** Generar `.deb` y `.rpm` con nfpm
- [ ] **T13.5** Firma GPG de binarios
- [ ] **T13.6** macOS: firma con Developer ID + notarización (si está disponible la cuenta)
- [ ] **T13.7** GitHub Actions release workflow: tag → goreleaser → publish
- [ ] **T13.8** Script `make release-dry` que valida config sin publicar
- [ ] **T13.9** Documentar proceso en `docs/RELEASE.md`

## Acceptance criteria

- `goreleaser release --snapshot --clean` genera todos los artefactos localmente
- Release real publica binarios + tap + .deb + .rpm
- Install con `brew install LeandroLCD/tap/sudoconsole` funciona
- Install con `apt install ./sudoconsole.deb` funciona
- Tag `v0.1.0` → release `v0.1.0` con changelog auto

## Output
PR a `develop` + tag `v0.1.0` → release. Cierra #14.