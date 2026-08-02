# Release process

This document describes how `sudoconsole` is released to the
public. The pipeline is fully automated via GitHub Actions +
[goreleaser](https://goreleaser.com) but a human still needs to
push the tag.

## Versioning

We follow [Semantic Versioning](https://semver.org/spec/v2.0.0.html):

- **MAJOR** — incompatible API/CLI breakage. Not yet applicable
  (we are pre-1.0).
- **MINOR** — backwards-compatible feature additions (M7 → M8 → M9).
- **PATCH** — backwards-compatible bug fixes.

Pre-1.0 we use `0.MINOR.PATCH` (e.g. `0.2.0`). The upcoming first
public release will be tagged `v0.2.0` to match the current state of
the project.

## Cutting a release

1. Ensure `develop` is green: the unit-test + lint + gosec +
   integration matrix all pass.
2. Update `CHANGELOG.md`: move the entries under `[Unreleased]` into
   a new dated section. Commit it directly to `develop`.
3. Tag the merge commit:
   ```bash
   git checkout develop && git pull
   git tag -a v0.X.Y -m "v0.X.Y"
   git push origin v0.X.Y
   ```
4. The `.github/workflows/release.yml` workflow fires automatically:
   - builds 4 GOOS/GOARCH pairs (`linux/amd64`, `linux/arm64`,
     `darwin/amd64`, `darwin/arm64`);
   - archives them as `tar.gz`;
   - generates `.deb` and `.rpm` packages via nfpm;
   - computes `sha256` checksums;
   - cosign-signs the checksum (keyless via GitHub OIDC);
   - writes a draft GitHub release with the auto-generated changelog
     grouped by `feat:` / `fix:` / `*`;
   - pushes a Homebrew formula to `LeandroLCD/homebrew-tap`.
5. Review the draft release in the GitHub UI. Publish it once the
   artifacts look good.

## Verifying a release

The CI workflow signs the checksums file with cosign keyless
signing. To verify locally:

```bash
cosign verify-blob \
  --bundle sudoconsole_X.Y.Z_SHA256SUMS.txt.sig \
  --certificate-identity-regexp 'https://github.com/LeandroLCD/sudoconsole' \
  --certificate-oidc-issuer 'https://token.actions.githubusercontent.com' \
  sudoconsole_X.Y.Z_SHA256SUMS.txt
```

Then check each artifact against the verified SHA-256:

```bash
sha256sum -c <(grep -F "$(basename artifact.tar.gz)" sudoconsole_X.Y.Z_SHA256SUMS.txt)
```

## Installing from the release

```bash
# Linux/macOS: pick the matching tarball from the release page.
curl -sSL https://github.com/LeandroLCD/sudoconsole/releases/download/v0.X.Y/sudoconsole_0.X.Y_linux_amd64.tar.gz | tar -xz -C /tmp
sudo install /tmp/sudoconsole /usr/local/bin/

# Debian/Ubuntu:
sudo dpkg -i sudoconsole_0.X.Y_linux_amd64.deb

# Fedora/RHEL:
sudo dnf install ./sudoconsole_0.X.Y_linux_amd64.rpm

# macOS via Homebrew:
brew install LeandroLCD/tap/sudoconsole
```

## Homebrew tap setup

The goreleaser config expects a tap at `LeandroLCD/homebrew-tap`.
On first release that repo does not exist; create it as a *public*
repository containing only a `README.md` (or empty `master` branch).
The release workflow will push the formula via the
`LeandroLCD/homebrew-tap` repository's `GORELEASER_HOMEBREW_TOKEN`
secret, or — when that secret is absent — fall back to the workflow's
own `GITHUB_TOKEN`, which only works for repos in the same
organisation.

If publishing the tap fails because of permissions, create a
fine-grained PAT with `contents: write` on `homebrew-tap` and add it
as the `GORELEASER_HOMEBREW_TOKEN` repository secret.

## Snapshot (dry-run)

To validate the pipeline without burning a tag, run locally:

```bash
make release-dry
```

This invokes `goreleaser release --snapshot --clean --skip=publish,sign`
which produces every artefact under `./dist/` *except* the GitHub
release / Homebrew push / cosign signatures.

To do a full dry-run that exercises the publish path too, run
`.github/workflows/release.yml` manually via the *Run workflow*
button in the GitHub UI (`workflow_dispatch`).

## macOS code-signing (future)

`scripts/release-apple-notary.sh` is reserved for when the
maintainer obtains an Apple Developer ID. Today the macOS
binaries are unsigned; Homebrew users on macOS will get a
"unidentified developer" Gatekeeper prompt the first time they
invoke the binary. Fix with `xattr -d com.apple.quarantine
$(which sudoconsole)` until we ship signed binaries.

## Hotfix workflow

A bug found in a released version gets a `PATCH` release that
branches off the release tag:

```bash
git checkout -b hotfix/v0.X.Z v0.X.Y
# fix + test
git commit ...
git tag -a v0.X.Z -m "v0.X.Z hotfix"
git push origin v0.X.Z
```

Then open a PR back into `develop` so the fix lands in the mainline.