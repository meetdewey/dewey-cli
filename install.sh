#!/usr/bin/env sh
# Install the latest dewey CLI binary.
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/meetdewey/dewey-cli/main/install.sh | sh
#
# The binary is placed in /usr/local/bin by default.
# Override with: INSTALL_DIR=/usr/bin sh install.sh

set -e

REPO="meetdewey/dewey-cli"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"

# ── Detect OS and arch ────────────────────────────────────────────────────────

OS="$(uname -s)"
ARCH="$(uname -m)"

case "$OS" in
  Darwin) OS="darwin" ;;
  Linux)  OS="linux" ;;
  *)
    echo "Unsupported OS: $OS" >&2
    echo "Download manually from https://github.com/$REPO/releases/latest" >&2
    exit 1
    ;;
esac

case "$ARCH" in
  x86_64)          ARCH="amd64" ;;
  arm64 | aarch64) ARCH="arm64" ;;
  *)
    echo "Unsupported architecture: $ARCH" >&2
    echo "Download manually from https://github.com/$REPO/releases/latest" >&2
    exit 1
    ;;
esac

# ── Resolve latest version ────────────────────────────────────────────────────

VERSION="$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" \
  | grep '"tag_name"' \
  | head -1 \
  | sed 's/.*"tag_name": *"\([^"]*\)".*/\1/')"

if [ -z "$VERSION" ]; then
  echo "Could not determine latest version from GitHub API." >&2
  exit 1
fi

echo "Installing dewey $VERSION ($OS/$ARCH) → $INSTALL_DIR/dewey"

# ── Download and extract ──────────────────────────────────────────────────────

ARCHIVE="dewey_${VERSION#v}_${OS}_${ARCH}.tar.gz"
URL="https://github.com/$REPO/releases/download/$VERSION/$ARCHIVE"
CHECKSUM_URL="https://github.com/$REPO/releases/download/$VERSION/checksums.txt"

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

curl -fsSL "$URL" -o "$TMP/$ARCHIVE"

# Verify checksum. macOS ships shasum (perl); Linux ships sha256sum (coreutils).
curl -fsSL "$CHECKSUM_URL" -o "$TMP/checksums.txt"
if [ "$OS" = "darwin" ] && command -v shasum >/dev/null 2>&1; then
  (cd "$TMP" && grep "$ARCHIVE" checksums.txt | shasum -a 256 -c)
elif command -v sha256sum >/dev/null 2>&1; then
  (cd "$TMP" && grep "$ARCHIVE" checksums.txt | sha256sum -c)
else
  echo "Warning: no checksum tool found, skipping verification." >&2
fi

tar -xzf "$TMP/$ARCHIVE" -C "$TMP"

# ── Install ───────────────────────────────────────────────────────────────────

if [ -w "$INSTALL_DIR" ]; then
  mv "$TMP/dewey" "$INSTALL_DIR/dewey"
else
  sudo mv "$TMP/dewey" "$INSTALL_DIR/dewey"
fi

echo "dewey installed successfully."
"$INSTALL_DIR/dewey" version
