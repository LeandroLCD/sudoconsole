#!/usr/bin/env bash
# scripts/test-install.sh — local integration test for install.sh.
#
# Sets up a fake "release" directory, runs install.sh against it,
# and verifies the binary lands in the right place with the right
# permissions. Uses SUDOCONSOLE_VERSION=0.0.1-test to bypass the
# GitHub API call.
#
# Run: bash scripts/test-install.sh

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$REPO_ROOT"

# 1. Build a fake release artifact (the script accepts whatever
#    tarball we point it at).
TMP=$(mktemp -d -t sudoconsole-install-test.XXXXXX)
trap 'rm -rf "$TMP"' EXIT

VERSION="v0.0.1-test"
VERSION_NOPREFIX="${VERSION#v}"

# Build with the test version baked into the binary so the
# `binary version matches` assertion in step 5 is meaningful.
VERSION="$VERSION_NOPREFIX" make build >/dev/null
# 1. Build a fake release artifact and lay it out in a directory
#    tree that matches GitHub's URL scheme so install.sh can
#    download it. /latest -> a directory named "latest" that
#    contains an API-shaped JSON; /releases/download/<version>/...
#    -> the actual tarball + checksums.
SERVER_ROOT="$TMP/server"
mkdir -p "$SERVER_ROOT/releases/download/$VERSION"
mkdir -p "$SERVER_ROOT/releases"

cp "bin/sudoconsole" "$SERVER_ROOT/releases/download/$VERSION/sudoconsole"
(cd "$SERVER_ROOT/releases/download/$VERSION" \
	&& tar -czf "sudoconsole_${VERSION_NOPREFIX}_linux_amd64.tar.gz" sudoconsole)

# SHA-256 for the (unsigned) test artifact so install.sh's verify
# path can be exercised when --no-verify is *not* passed. We don't
# sign with cosign — that's the real-release path.
sha256sum "$SERVER_ROOT/releases/download/$VERSION/sudoconsole_${VERSION_NOPREFIX}_linux_amd64.tar.gz" \
	| awk -v a="sudoconsole_${VERSION_NOPREFIX}_linux_amd64.tar.gz" '{print $1"  "a}' \
	> "$SERVER_ROOT/releases/download/$VERSION/sudoconsole_${VERSION_NOPREFIX}_checksums.txt"

# 2. Stand up a local HTTP server rooted at the server directory.
python3 -m http.server 18765 --directory "$SERVER_ROOT" >/tmp/install-httpd.log 2>&1 &
HTTP_PID=$!
trap 'rm -rf "$TMP"; kill "$HTTP_PID" 2>/dev/null || true' EXIT
sleep 1

DEST="$TMP/bin"
mkdir -p "$DEST"

# 3. Run a copy of install.sh pointed at the local server.
TMP_INSTALL="$TMP/install.sh"
cp scripts/install.sh "$TMP_INSTALL"
sed -i 's|https://github.com/${REPO}|http://127.0.0.1:18765|g' "$TMP_INSTALL"
sed -i 's|https://api.github.com|http://127.0.0.1:18765|g' "$TMP_INSTALL"
chmod +x "$TMP_INSTALL"

SUDOCONSOLE_NO_VERIFY=1 \
SUDOCONSOLE_VERSION="$VERSION" \
SUDOCONSOLE_INSTALL_DIR="$DEST" \
	"$TMP_INSTALL" >/tmp/install-test.log 2>&1 || {
	echo "install.sh failed; log:"
	cat /tmp/install-test.log
	exit 1
}

# 5. Verify.
if [ ! -x "$DEST/sudoconsole" ]; then
	echo "binary missing from $DEST"
	exit 1
fi
"$DEST/sudoconsole" version >/tmp/install-version.log 2>&1 || {
	echo "binary failed to run"; cat /tmp/install-version.log; exit 1
}
grep -F "0.0.1-test" /tmp/install-version.log >/dev/null || {
	echo "version mismatch"; cat /tmp/install-version.log; exit 1
}

echo "OK: install.sh landed sudoconsole at $DEST/sudoconsole (version $VERSION)"
echo "    full log: /tmp/install-test.log"