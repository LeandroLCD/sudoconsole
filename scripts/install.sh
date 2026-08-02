#!/usr/bin/env bash
#
# scripts/install.sh — one-line installer for sudoconsole.
#
# Canonical URL (served via GitHub Pages):
#   https://LeandroLCD.github.io/sudoconsole/install.sh
#
# Usage:
#   curl -fsSL https://LeandroLCD.github.io/sudoconsole/install.sh | sh
#   curl -fsSL .../install.sh | sh -s -- --to ~/.bin --version v0.3.0
#   curl -fsSL .../install.sh | sh -s -- --no-verify    # skip signature check
#
# Exit codes:
#   0  success
#   1  generic error
#   2  unsupported platform
#   3  download failed
#   4  signature verification failed
#   5  install path not writable and no sudo
#
set -eu

REPO="LeandroLCD/sudoconsole"
DEFAULT_VERSION="latest"
INSTALL_DIR_SYSTEM="/usr/local/bin"
INSTALL_DIR_USER="${HOME}/.local/bin"
PATH_LINE='export PATH="$HOME/.local/bin:$PATH"'

usage() {
	cat <<EOF
Usage: install.sh [options]

Options:
  --to <dir>         Install to <dir> instead of detecting the default.
  --version <tag>    Release tag to install (default: latest).
  --no-verify        Skip cosign signature verification.
  --no-modify-path    Don't print PATH instructions on user installs.
  -h, --help          Show this help and exit.

Environment:
  SUDOCONSOLE_INSTALL_DIR   Override install location.
  SUDOCONSOLE_VERSION       Override release tag.
  SUDOCONSOLE_NO_VERIFY=1   Same as --no-verify.
  SUDOCONSOLE_NO_MODIFY_PATH=1  Same as --no-modify-path.
EOF
}

log() { printf '\033[1;36m[install]\033[0m %s\n' "$*"; }
warn() { printf '\033[1;33m[install]\033[0m %s\n' "$*" >&2; }
err() { printf '\033[1;31m[install]\033[0m %s\n' "$*" >&2; }
ok() { printf '\033[1;32m[install]\033[0m %s\n' "$*"; }

# ---------------------------------------------------------------------------
# Argument parsing
# ---------------------------------------------------------------------------
INSTALL_DIR="${SUDOCONSOLE_INSTALL_DIR:-}"
VERSION="${SUDOCONSOLE_VERSION:-$DEFAULT_VERSION}"
VERIFY=1
MODIFY_PATH=1
EXPLICIT_DIR=0

while [ $# -gt 0 ]; do
	case "$1" in
		--to)              INSTALL_DIR="$2"; EXPLICIT_DIR=1; shift 2 ;;
		--version)         VERSION="$2"; shift 2 ;;
		--no-verify)       VERIFY=0; shift ;;
		--no-modify-path)  MODIFY_PATH=0; shift ;;
		-h|--help)         usage; exit 0 ;;
		*) err "unknown argument: $1"; usage; exit 1 ;;
	esac
done

[ "${SUDOCONSOLE_NO_VERIFY:-0}" = "1" ] && VERIFY=0
[ "${SUDOCONSOLE_NO_MODIFY_PATH:-0}" = "1" ] && MODIFY_PATH=0

# ---------------------------------------------------------------------------
# Platform detection
# ---------------------------------------------------------------------------
detect_platform() {
	local os arch
	case "$(uname -s)" in
		Linux)   os="linux" ;;
		Darwin)  os="darwin" ;;
		FreeBSD) os="freebsd" ;;
		*) err "unsupported OS: $(uname -s)"; exit 2 ;;
	esac
	case "$(uname -m)" in
		x86_64|amd64)   arch="amd64" ;;
		aarch64|arm64)  arch="arm64" ;;
		*) err "unsupported architecture: $(uname -m)"; exit 2 ;;
	esac
	echo "$os $arch"
}

# Split "os arch" into the two globals. We use awk rather than
# `read OS ARCH < <(detect_platform)` because process substitution
# (`<()`) is a bashism that dash (Debian/Ubuntu's /bin/sh) does
# not understand — the installer is frequently piped into `sh`,
# bypassing the `#!/usr/bin/env bash` shebang.
DETECTED="$(detect_platform)"
OS="$(echo "$DETECTED" | awk '{print $1}')"
ARCH="$(echo "$DETECTED" | awk '{print $2}')"
log "detected platform: ${OS}/${ARCH}"

# ---------------------------------------------------------------------------
# Pick install dir
# ---------------------------------------------------------------------------
pick_install_dir() {
	if [ -n "$INSTALL_DIR" ]; then
		echo "$INSTALL_DIR"; return
	fi
	if [ -w "$INSTALL_DIR_SYSTEM" ]; then
		echo "$INSTALL_DIR_SYSTEM"
	else
		echo "$INSTALL_DIR_USER"
	fi
}

if [ "$EXPLICIT_DIR" = "0" ]; then
	INSTALL_DIR=$(pick_install_dir)
fi
log "install dir: $INSTALL_DIR"

if [ ! -d "$INSTALL_DIR" ]; then
	if mkdir -p "$INSTALL_DIR" 2>/dev/null; then
		ok "created $INSTALL_DIR"
	else
		err "cannot create $INSTALL_DIR; pass --to <writable-dir> or run as root"
		exit 5
	fi
fi

if [ ! -w "$INSTALL_DIR" ]; then
	err "$INSTALL_DIR is not writable; pass --to <writable-dir> or run as root"
	exit 5
fi

# ---------------------------------------------------------------------------
# Version resolution (handle "latest")
# ---------------------------------------------------------------------------
if [ "$VERSION" = "latest" ]; then
	log "resolving latest release..."
	VERSION=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
		| sed -n 's/.*"tag_name":[[:space:]]*"\([^"]*\)".*/\1/p' | head -n1)
	if [ -z "$VERSION" ]; then
		err "could not resolve latest release tag (network or rate-limited?)"
		exit 3
	fi
fi
log "version: $VERSION"

case "$VERSION" in
	v*) VERSION_NOPREFIX="${VERSION#v}" ;;
	*)  VERSION_NOPREFIX="$VERSION" ;;
esac

ARTIFACT="sudoconsole_${VERSION_NOPREFIX}_${OS}_${ARCH}.tar.gz"
URL_BASE="https://github.com/${REPO}/releases/download/${VERSION}"
URL_TARBALL="${URL_BASE}/${ARTIFACT}"
URL_CHECKSUM="${URL_BASE}/sudoconsole_${VERSION_NOPREFIX}_checksums.txt"
URL_SIG="${URL_CHECKSUM}.sig"

# ---------------------------------------------------------------------------
# Download
# ---------------------------------------------------------------------------
TMPDIR=$(mktemp -d -t sudoconsole-install.XXXXXX)
trap 'rm -rf "$TMPDIR"' EXIT

log "downloading $ARTIFACT..."
if ! curl -fsSL --retry 3 -o "$TMPDIR/$ARTIFACT" "$URL_TARBALL"; then
	err "download failed: $URL_TARBALL"
	err "verify that release $VERSION exists at https://github.com/${REPO}/releases/tag/$VERSION"
	exit 3
fi

if [ "$VERIFY" = "1" ]; then
	if command -v cosign >/dev/null 2>&1; then
		log "verifying signature with cosign..."
		if ! curl -fsSL -o "$TMPDIR/checksums.txt" "$URL_CHECKSUM"; then
			err "download checksums failed"; exit 3
		fi
		if ! curl -fsSL -o "$TMPDIR/checksums.txt.sig" "$URL_SIG"; then
			err "download signature failed"; exit 3
		fi
		if ! cosign verify-blob \
				--bundle "$TMPDIR/checksums.txt.sig" \
				--certificate-identity-regexp "https://github.com/${REPO}" \
				--certificate-oidc-issuer "https://token.actions.githubusercontent.com" \
				"$TMPDIR/checksums.txt" >/dev/null 2>&1; then
			err "cosign signature verification FAILED — refusing to install"
			exit 4
		fi
		ok "cosign signature verified"

		# Now check that the artifact's SHA-256 matches.
		expected=$(grep -F "  $ARTIFACT" "$TMPDIR/checksums.txt" | awk '{print $1}')
		actual=$(sha256sum "$TMPDIR/$ARTIFACT" | awk '{print $1}')
		if [ "$expected" != "$actual" ]; then
			err "sha256 mismatch: expected=$expected actual=$actual"
			exit 4
		fi
		ok "sha256 verified"
	else
		warn "cosign not installed; skipping signature check (install cosign for hardened installs)"
	fi
else
	warn "signature verification skipped (--no-verify)"
fi

# ---------------------------------------------------------------------------
# Extract + install
# ---------------------------------------------------------------------------
log "extracting..."
tar -xzf "$TMPDIR/$ARTIFACT" -C "$TMPDIR"
if [ ! -f "$TMPDIR/sudoconsole" ]; then
	err "expected 'sudoconsole' binary inside the archive; got:"
	ls -la "$TMPDIR"
	exit 1
fi

TARGET="$INSTALL_DIR/sudoconsole"
log "installing to $TARGET..."
mv "$TMPDIR/sudoconsole" "$TARGET"
chmod 0755 "$TARGET"
ok "installed $(basename "$TARGET")"

# ---------------------------------------------------------------------------
# Verify + post-install messaging
# ---------------------------------------------------------------------------
if command -v sudoconsole >/dev/null 2>&1; then
	VERSION_REPORTED=$(sudoconsole version 2>/dev/null | head -n1 || true)
	if [ -n "$VERSION_REPORTED" ]; then
		ok "verified: $VERSION_REPORTED"
	fi
else
	case "$INSTALL_DIR" in
		*/.local/bin)
			if [ "$MODIFY_PATH" = "1" ]; then
				warn "$INSTALL_DIR is not on your PATH. Add this to your shell rc:"
				printf '  %s\n' "$PATH_LINE"
			fi
			;;
		*)
			warn "$INSTALL_DIR is not on your PATH (unusual)"
			;;
	esac
fi

cat <<EOF

$(ok "next steps:")
  sudoconsole version
  sudoconsole auth         # prime the sudo cache on a TTY
  sudoconsole install      # register with every detected CLI agent

Documentation: https://github.com/${REPO}#readme
EOF