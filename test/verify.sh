#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
export LAZYREDIS_TEST_ROOT="$ROOT"
cd "$ROOT"
go test -tags=integration ./internal/store -run TestIntegrationConnections -count=1 -v "$@"
