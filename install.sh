#!/usr/bin/env bash
set -euo pipefail

INSTALL_DIR="${INSTALL_DIR:-$HOME/.local/bin}"
BIN_NAME="lazyredis"

cd "$(dirname "$0")"

mkdir -p "$INSTALL_DIR"
go build -o "$INSTALL_DIR/$BIN_NAME" ./cmd/lazyredis

echo "Installed $BIN_NAME to $INSTALL_DIR/$BIN_NAME"
