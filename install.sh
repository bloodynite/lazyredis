#!/bin/sh
# Install lazyredis from GitHub Releases.
# POSIX sh compatible — safe to run via `curl ... | sh`.
#
# Environment overrides:
#   INSTALL_DIR      destination directory (default: $HOME/.local/bin)
#   INSTALL_VERSION  release tag (default: latest, e.g. v0.2.0)

set -eu

REPO="bloodynite/lazyredis"
BIN_NAME="lazyredis"
INSTALL_DIR="${INSTALL_DIR:-$HOME/.local/bin}"
INSTALL_VERSION="${INSTALL_VERSION:-latest}"

log() {
    printf '%s\n' "$*" >&2
}

die() {
    log "error: $*"
    exit 1
}

require() {
    command -v "$1" >/dev/null 2>&1 || die "required tool '$1' not found in PATH"
}

require curl
require uname
require mktemp
require awk
require chmod

tmpdir="$(mktemp -d)"
trap 'rm -rf "$tmpdir"' EXIT INT TERM

resolve_latest() {
    url="https://api.github.com/repos/$REPO/releases/latest"
    body_file="$tmpdir/latest.json"
    if ! curl -fsSL --retry 3 -o "$body_file" "$url"; then
        die "could not fetch latest release from $url"
    fi
    tag=$(sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' "$body_file" | head -n 1)
    if [ -z "$tag" ]; then
        die "could not parse tag_name from latest release response"
    fi
    printf '%s' "$tag"
}

case "$INSTALL_VERSION" in
    latest) INSTALL_VERSION=$(resolve_latest) ;;
    v*)     : ;;
    *)      die "INSTALL_VERSION must be 'latest' or start with 'v' (got: $INSTALL_VERSION)" ;;
esac

log "Installing $BIN_NAME $INSTALL_VERSION to $INSTALL_DIR/$BIN_NAME"

uname_s=$(uname -s)
uname_m=$(uname -m)

case "$uname_s" in
    Linux*)                asset_os=linux ;;
    Darwin*)               asset_os=darwin ;;
    FreeBSD*)              asset_os=freebsd ;;
    MINGW*|MSYS*|CYGWIN*)  die "windows: please use scoop, winget or download the .exe from $REPO/releases" ;;
    *)                     die "unsupported operating system: $uname_s" ;;
esac

case "$uname_m" in
    x86_64|amd64)   asset_arch=amd64 ;;
    aarch64|arm64)  asset_arch=arm64 ;;
    *)              die "unsupported architecture: $uname_m" ;;
esac

asset_name="$BIN_NAME-$asset_os-$asset_arch"
case "$asset_os" in
    windows) asset_name="$asset_name.exe" ;;
esac

base_url="https://github.com/$REPO/releases/download/$INSTALL_VERSION"
binary_url="$base_url/$asset_name"
sums_url="$base_url/SHA256SUMS"

log "Downloading $binary_url"
if ! curl -fL --retry 3 -o "$tmpdir/$asset_name" "$binary_url"; then
    die "download failed (HTTP error). Check that $INSTALL_VERSION was published with asset $asset_name"
fi
chmod +x "$tmpdir/$asset_name"

if curl -fsSL --retry 3 -o "$tmpdir/SHA256SUMS" "$sums_url" 2>/dev/null; then
    expected=$(awk -v n="$asset_name" '$2 == n {print $1}' "$tmpdir/SHA256SUMS")
    if [ -z "$expected" ]; then
        log "warning: SHA256SUMS does not list $asset_name; skipping integrity check"
    else
        if command -v sha256sum >/dev/null 2>&1; then
            actual=$(sha256sum "$tmpdir/$asset_name" | awk '{print $1}')
        elif command -v shasum >/dev/null 2>&1; then
            actual=$(shasum -a 256 "$tmpdir/$asset_name" | awk '{print $1}')
        elif command -v openssl >/dev/null 2>&1; then
            actual=$(openssl dgst -sha256 -r "$tmpdir/$asset_name" | awk '{print $1}')
        else
            die "no sha256 tool found (need one of: sha256sum, shasum, openssl)"
        fi
        if [ "$expected" != "$actual" ]; then
            log "SHA256 mismatch for $asset_name"
            log "  expected: $expected"
            log "  actual:   $actual"
            exit 1
        fi
        log "SHA256 verified"
    fi
else
    log "warning: SHA256SUMS not published for $INSTALL_VERSION; skipping integrity check"
fi

mkdir -p "$INSTALL_DIR"
mv "$tmpdir/$asset_name" "$INSTALL_DIR/$BIN_NAME"

case ":$PATH:" in
    *":$INSTALL_DIR:"*) ;;
    *) log "note: $INSTALL_DIR is not on your PATH. Add it or call $INSTALL_DIR/$BIN_NAME directly" ;;
esac

log "Installed $BIN_NAME $INSTALL_VERSION to $INSTALL_DIR/$BIN_NAME"
