#!/usr/bin/env sh

set -eu

OWNER="malleus35"
REPO="agentcom"
VERSION="${VERSION:-v0.1.5}"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"

OS="$(uname -s)"
ARCH="$(uname -m)"

case "$OS" in
  Darwin) PLATFORM="darwin" ;;
  Linux) PLATFORM="linux" ;;
  *)
    printf 'unsupported operating system: %s\n' "$OS" >&2
    exit 1
    ;;
esac

case "$ARCH" in
  x86_64|amd64) GOARCH="amd64" ;;
  arm64|aarch64) GOARCH="arm64" ;;
  *)
    printf 'unsupported architecture: %s\n' "$ARCH" >&2
    exit 1
    ;;
esac

ARCHIVE="agentcom_${VERSION#v}_${PLATFORM}_${GOARCH}.tar.gz"
URL="https://github.com/${OWNER}/${REPO}/releases/download/${VERSION}/${ARCHIVE}"

TMPDIR="$(mktemp -d)"
cleanup() {
  rm -rf "$TMPDIR"
}
trap cleanup EXIT INT TERM

printf 'Downloading %s\n' "$URL"
curl -fsSL "$URL" -o "$TMPDIR/$ARCHIVE"
tar -xzf "$TMPDIR/$ARCHIVE" -C "$TMPDIR"

mkdir -p "$INSTALL_DIR"
install "$TMPDIR/agentcom" "$INSTALL_DIR/agentcom"

printf 'Installed agentcom to %s/agentcom\n' "$INSTALL_DIR"
"$INSTALL_DIR/agentcom" version
