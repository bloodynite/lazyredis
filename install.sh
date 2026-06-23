#!/usr/bin/env bash
set -euo pipefail

INSTALL_DIR="${INSTALL_DIR:-$HOME/.local/bin}"
INSTALL_VERSION="${INSTALL_VERSION:-v0.1.0}"
PKG="github.com/bloodynite/lazyredis/cmd/lazyredis"
BIN_NAME="lazyredis"

mkdir -p "$INSTALL_DIR"
GOBIN="$INSTALL_DIR" go install "$PKG@$INSTALL_VERSION"

echo "Installed $BIN_NAME $INSTALL_VERSION to $INSTALL_DIR/$BIN_NAME"
