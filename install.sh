#!/usr/bin/env bash
set -euo pipefail

INSTALL_DIR="${INSTALL_DIR:-$HOME/.local/bin}"
INSTALL_VERSION="${INSTALL_VERSION:-latest}"
PKG="github.com/bloodynite/lazyredis/cmd/lazyredis"
BIN_NAME="lazyredis"

mkdir -p "$INSTALL_DIR"
if [ "$INSTALL_VERSION" = "latest" ]; then
	GOBIN="$INSTALL_DIR" go install "$PKG@$INSTALL_VERSION"
else
	GOBIN="$INSTALL_DIR" go install -ldflags "-X github.com/bloodynite/lazyredis/internal/version.Version=$INSTALL_VERSION" "$PKG@$INSTALL_VERSION"
fi

echo "Installed $BIN_NAME $INSTALL_VERSION to $INSTALL_DIR/$BIN_NAME"
