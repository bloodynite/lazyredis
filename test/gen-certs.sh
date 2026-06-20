#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
TEST="$ROOT/test"
CERTS="$TEST/certs"
KEYS="$TEST/keys"
SSH="$TEST/ssh"

mkdir -p "$CERTS" "$KEYS" "$SSH"

if [[ ! -f "$CERTS/ca.crt" ]]; then
  openssl req -x509 -newkey rsa:2048 -nodes \
    -keyout "$CERTS/ca.key" -out "$CERTS/ca.crt" -days 3650 \
    -subj "/CN=lazyredis-test-ca"
fi

if [[ ! -f "$CERTS/redis.crt" ]]; then
  openssl req -newkey rsa:2048 -nodes \
    -keyout "$CERTS/redis.key" -out "$CERTS/redis.csr" \
    -subj "/CN=localhost"
  openssl x509 -req -in "$CERTS/redis.csr" \
    -CA "$CERTS/ca.crt" -CAkey "$CERTS/ca.key" -CAcreateserial \
    -out "$CERTS/redis.crt" -days 3650
  rm -f "$CERTS/redis.csr" "$CERTS/ca.srl"
fi

if [[ ! -f "$KEYS/id_ed25519" ]]; then
  ssh-keygen -t ed25519 -N "" -f "$KEYS/id_ed25519" -C "lazyredis-test"
fi

install -m 600 "$KEYS/id_ed25519.pub" "$SSH/authorized_keys"

chmod 644 "$CERTS/ca.crt" "$CERTS/redis.crt" "$CERTS/redis.key" 2>/dev/null || true

echo "Generated test assets in $TEST"
