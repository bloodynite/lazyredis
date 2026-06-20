#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
TEST="$ROOT/test"

"$TEST/gen-certs.sh"

cd "$TEST"
docker compose down -v 2>/dev/null || true
docker compose up -d --build

echo "Waiting for services..."
sleep 5

docker compose ps

KEY="$TEST/keys/id_ed25519"
PROFILES="$TEST/profiles.generated.yaml"

cat > "$PROFILES" <<EOF
profiles:
  - name: test-standalone
    mode: standalone
    addr: 127.0.0.1:16379
    password: redis_secret
    db: 0

  - name: test-tls
    mode: standalone
    addr: 127.0.0.1:16666
    password: redis_secret
    db: 0
    tls:
      enabled: true
      insecure_skip_verify: true
      server_name: localhost

  - name: test-ssh
    mode: standalone
    addr: redis:6379
    password: redis_secret
    db: 0
    ssh_tunnel:
      enabled: true
      host: 127.0.0.1:12222
      user: testuser
      private_key: $KEY
      insecure_skip_verify: true

  - name: test-socks5
    mode: standalone
    addr: 127.0.0.1:16379
    password: redis_secret
    db: 0
    proxy:
      type: socks5
      addr: 127.0.0.1:1080

  - name: test-sentinel
    mode: sentinel
    addrs:
      - 127.0.0.1:26379
      - 127.0.0.1:26380
      - 127.0.0.1:26381
    master_name: mymaster
    password: redis_secret
    db: 0

  - name: test-cluster
    mode: cluster
    addrs:
      - 127.0.0.1:7000
      - 127.0.0.1:7001
      - 127.0.0.1:7002
      - 127.0.0.1:7003
      - 127.0.0.1:7004
      - 127.0.0.1:7005
    password: ""
    db: 0

  - name: test-ssh-tls
    mode: standalone
    addr: redis-tls:6666
    password: redis_secret
    db: 0
    tls:
      enabled: true
      insecure_skip_verify: true
      server_name: localhost
    ssh_tunnel:
      enabled: true
      host: 127.0.0.1:12222
      user: testuser
      private_key: $KEY
      insecure_skip_verify: true

  - name: test-secure-remote
    mode: standalone
    addr: redis-tls:6666
    password: redis_secret
    db: 0
    tls:
      enabled: true
      insecure_skip_verify: true
      server_name: localhost
    ssh_tunnel:
      enabled: true
      host: 127.0.0.1:12222
      user: testuser
      private_key: $KEY
      insecure_skip_verify: true
    proxy:
      type: socks5
      addr: 127.0.0.1:1080
EOF

echo
echo "Profiles written to: $PROFILES"
echo
echo "Run automated checks:"
echo "  LAZYREDIS_TEST_ROOT=$ROOT go test -tags=integration ./internal/store -run TestIntegrationConnections -count=1"
echo
echo "Or merge profiles into your config:"
echo "  mkdir -p ~/.config/lazyredis"
echo "  cp $PROFILES ~/.config/lazyredis/profiles.yaml"
echo "  $ROOT/lazyredis"
