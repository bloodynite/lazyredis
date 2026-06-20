# lazyredis

Terminal UI for browsing and editing Redis keys. Built with [Bubble Tea](https://github.com/charmbracelet/bubbletea).

## Screenshots

Browser connected to a local profile, browsing `demos:*` keys across Redis types:

![Browser with demo keys](docs/screenshots/browser-demos-keys.png)

Edit key modal with TTL, type, and value fields:

![Edit key modal](docs/screenshots/edit-key-modal.png)

## Features

- **Connection profiles** with standalone, cluster, and sentinel modes
- **Advanced connectivity**: TLS/SSL, SSH tunnel, HTTP proxy, SOCKS5 proxy
- Two-panel browser: key list (1/5 width) + detail view (4/5 width)
- Live key filter with `SCAN` (substring or glob pattern)
- **Paginated key loading** (`g`) for large databases
- Create and edit keys: **string**, **hash**, **list**, **set**, **zset**, **stream**
- **Detail panel CRUD**: add, edit, or delete individual hash fields, list items, set members, zset scores, stream entries
- **Copy value to system clipboard** (`c`) — uses `wl-copy`, `xclip`, or terminal OSC 52
- TTL editing in a modal (`t`); persist (no expiry)
- Auto-refresh (default 5s, configurable)
- Fixed **server info bar** (2 rows) + **status line** for messages + **keybar** (up to 2 rows)
- Configurable keybindings (`ctrl+`, `alt+`, comma-separated aliases)
- Adaptive theme (uses terminal colors)
- In-app help (`?`)

## Requirements

- Go 1.25+
- A running Redis instance (or cluster/sentinel setup)

## Install

From a release tag:

```bash
go install github.com/bloodynite/lazyredis/cmd/lazyredis@v0.0.2
```

Then run:

```bash
lazyredis
```

From source:

```bash
git clone https://github.com/bloodynite/lazyredis.git
cd lazyredis
go build -o lazyredis ./cmd/lazyredis
./lazyredis
```

On first launch, a default config is created at `~/.config/lazyredis/profiles.yaml`.

## Configuration

Config path: `~/.config/lazyredis/profiles.yaml`

### Example

```yaml
settings:
  refresh_interval_seconds: 5
  keybindings:
    browser.refresh: "ctrl+r"
    browser.flush: "ctrl+f"
    browser.filter: "alt+/"

profiles:
  - name: local
    mode: standalone
    addr: 127.0.0.1:6379
    password: ""
    db: 0

  - name: production
    mode: sentinel
    addrs:
      - 10.0.0.1:26379
      - 10.0.0.2:26379
    master_name: mymaster
    password: secret
    db: 0

  - name: cluster
    mode: cluster
    addrs:
      - node1:6379
      - node2:6379
      - node3:6379
    password: secret
    db: 0

  - name: secure-remote
    mode: standalone
    addr: 10.0.0.5:6379
    password: secret
    db: 0
    tls:
      enabled: true
      ca_cert: /etc/ssl/redis-ca.pem
      cert: /etc/ssl/redis-client.pem
      key: /etc/ssl/redis-client.key
      server_name: redis.internal
    ssh_tunnel:
      enabled: true
      host: jump.example.com:22
      user: deploy
      private_key: ~/.ssh/id_ed25519
      known_hosts: ~/.ssh/known_hosts
    proxy:
      type: socks5
      addr: corp-proxy:1080
      username: user
      password: pass
```

### Profile fields

| Field | Description |
|-------|-------------|
| `name` | Profile display name (required) |
| `mode` | `standalone`, `cluster`, or `sentinel` |
| `addr` | Redis address (`host:port`) for standalone, or fallback when `addrs` is empty |
| `addrs` | Address list for cluster/sentinel |
| `master_name` | Sentinel master name (required for sentinel) |
| `password` | Redis password |
| `sentinel_username` | Sentinel ACL username (optional) |
| `sentinel_password` | Sentinel password (optional) |
| `db` | Database number (standalone and sentinel) |
| `tls` | TLS/SSL settings (see below) |
| `ssh_tunnel` | SSH tunnel settings (see below) |
| `proxy` | HTTP or SOCKS5 proxy (see below) |

### TLS / SSL (`tls`)

| Field | Description |
|-------|-------------|
| `enabled` | Enable TLS for Redis connections |
| `insecure_skip_verify` | Skip server certificate verification (testing only) |
| `ca_cert` | Path to CA certificate (PEM) |
| `cert` | Client certificate for mutual TLS (PEM) |
| `key` | Client private key (PEM) |
| `server_name` | TLS SNI / hostname verification (defaults to Redis host) |

In the profile form, use **TLS** field: `off`, `on`, or `skip`. Certificate paths are best edited in YAML (preserved when saving from the form).

### SSH tunnel (`ssh_tunnel`)

Redis `addr` / `addrs` are resolved **from the SSH server** (not your laptop).

| Field | Description |
|-------|-------------|
| `enabled` | Enable SSH tunnel |
| `host` | SSH server `host:port` (default port 22) |
| `user` | SSH username |
| `password` | SSH password (optional if using key) |
| `private_key` | Path to SSH private key |
| `private_key_passphrase` | Key passphrase (optional) |
| `known_hosts` | Path to known_hosts file for host key verification |
| `insecure_skip_verify` | Skip host key verification (use only for testing) |

Form fields: **SSH** = `user@host:port`, **SSH private key** = key path. The form sets `insecure_skip_verify: true` unless you add `known_hosts` in YAML.

Connection order: **proxy → SSH → Redis** (then TLS if enabled).

### Proxy (`proxy`)

| Field | Description |
|-------|-------------|
| `type` | `http` or `socks5` |
| `addr` | Proxy `host:port` |
| `username` | Proxy username (optional) |
| `password` | Proxy password (optional) |

Form / compact syntax: `socks5://host:1080` or `http://user:pass@host:8080`

The proxy is used to reach the SSH server and/or Redis, depending on profile setup.

### Settings

| Field | Description |
|-------|-------------|
| `refresh_interval_seconds` | Auto-refresh interval in seconds (`0` = off). Default: `5` |
| `keybindings` | Map of action ID → key string (comma-separated for multiple binds) |

Action IDs match the defaults in [internal/tui/keys.go](internal/tui/keys.go), for example:

- `browser.edit`, `browser.delete`, `browser.ttl`, `browser.new_key`, `browser.copy`
- `browser.detail_add`, `browser.detail_edit`, `browser.detail_delete`
- `browser.refresh`, `browser.auto_refresh`, `browser.flush`
- `browser.filter`, `browser.more_keys`, `browser.disconnect`
- `app.help`, `app.force_quit`

Keys are normalized to lowercase. Supported modifiers: `ctrl+`, `alt+`, `shift+`.

## Screens

| Screen | Purpose |
|--------|---------|
| **Profiles** | Select, create, edit, or delete connection profiles |
| **Profile form** | Edit profile fields including TLS, proxy, and SSH (overlay) |
| **Browser** | Keys + detail panels with fixed chrome (see layout below) |
| **Key edit** | Modals for new/edit key, TTL, auto-refresh, detail item edit |
| **Confirm** | Destructive actions (delete key/profile, flush DB) |

Press `?` anytime for context-sensitive help.

## Default keybindings

### Profiles

| Key | Action |
|-----|--------|
| `j` / `↓` | Move down |
| `k` / `↑` | Move up |
| `Enter` | Connect |
| `n` | New profile |
| `e` | Edit profile |
| `d` | Delete profile |
| `q` | Quit |

### Browser

| Key | Action |
|-----|--------|
| `Tab` | Switch Keys / Detail panel |
| `j` / `↓`, `k` / `↑` | Navigate list or detail |
| `/` | Filter keys (live scan while typing; `Enter` apply, `Esc` cancel) |
| `n` | New key (type selector in modal) |
| `e` | Edit selected key, or edit detail item when Detail panel is focused |
| `c` | Copy value to system clipboard |
| `i` | Add item (hash/list/set/zset/stream; Detail panel) |
| `d` | Delete key, or delete detail item when Detail panel is focused |
| `t` | Edit TTL (modal) |
| `r` | Refresh keys + info |
| `a` | Auto-refresh interval |
| `g` | Load more keys (when scan cursor remains) |
| `ctrl+f` | Flush current DB (confirmation) |
| `q` | Disconnect |
| `?` | Help |
| `ctrl+c` | Force quit |

**Filter:** plain text matches as substring (`*texto*`); use `:` and `*` for glob patterns (e.g. `user:*`, `demo:*`).

**Pagination:** initial load returns up to 100 keys per batch. When more keys exist, the Keys panel footer shows `loaded/total · g` and `g` loads the next page.

Context actions (`copy`, `edit`, `delete`, `ttl`, `g`) are pinned on the keybar when relevant.

### Modals (new/edit key, TTL, profile form)

| Key | Action |
|-----|--------|
| `Tab` / `Shift+Tab` | Next / previous field |
| `↑` / `↓` | Change key type (new key modal, Type field focused) |
| `Ctrl+Enter` | Save (key form, stream entry edit) |
| `Enter` | Save (TTL, auto-refresh, single-line item edit) |
| `Esc` | Cancel |

### Confirm dialog

| Key | Action |
|-----|--------|
| `y` | Confirm |
| `n` / `Esc` | Cancel |

## Editing keys

### New key (`n`)

Modal fields (top to bottom):

1. **TTL** — empty, seconds (`300`), duration (`1h`), or `persist` / `-` for no expiry
2. **Type** — `string`, `hash`, `list`, `set`, `zset`, `stream` (list with `↑`/`↓` when focused)
3. **Key** — key name
4. **Value** — body format depends on type (scrollable textarea)

Save with `Ctrl+Enter`.

### Edit key (`e`)

From the **Keys** panel: opens the full key modal (same as new key), pre-filled. Available for **string**, **hash**, **list**, **set**, **zset**, **stream**.

Saving **replaces the entire key** (delete + recreate) and applies TTL. Changing the key name renames by writing the new key and deleting the old one.

### Detail panel (`Tab` to focus)

For composite types, navigate items with `j`/`k` and:

| Key | Action |
|-----|--------|
| `e` | Edit selected item |
| `i` | Add item |
| `d` | Delete selected item |
| `c` | Copy value (string: full value; hash: field value; list/set: item; zset: member; stream: entry fields) |

**String keys** in Detail: `e` edits value, `d` deletes the key.

### Copy (`c`)

Copies the current value to the **system clipboard** (paste anywhere with `Ctrl+V`). On Linux/Wayland uses `wl-copy` when available; falls back to `xclip` or terminal OSC 52. Shows `copied to clipboard` in the status line for 3 seconds.

### Value format by type

| Type | Textarea format |
|------|-----------------|
| string | Raw value |
| hash | One field per line: `field=value` |
| list | One item per line |
| set | One member per line |
| zset | One member per line: `score<TAB>member` or `score member` |
| stream | One entry per line: `id<TAB>field=value …` or `*<TAB>field=value` for auto ID |

Empty hash/list/set/zset body removes all members (key may disappear if empty).

### TTL input

Accepted values:

- Empty, `-`, or `persist` → no expiry
- Seconds: `300`
- Duration: `3600s`, `1h`, `30m`

Edit with `t` (modal, pre-filled with current TTL).

## Auto-refresh

When connected, the browser refreshes keys and server info automatically every `refresh_interval_seconds` (default **5**).

- Press `a` to change the interval (saved to config)
- Set `0` to disable
- Press `r` for immediate manual refresh

## Development

```bash
go test -short ./...
```

### Connection test lab (Docker)

Stack for standalone, TLS, SSH tunnel, SOCKS5, Sentinel, Cluster, and the full `secure-remote` chain.

```bash
cd test
./up.sh
```

This generates certs/keys, starts containers, and writes `test/profiles.generated.yaml` with absolute paths.

| Service | Host port | Purpose |
|---------|-----------|---------|
| redis | 16379 | Standalone + SOCKS5 target |
| redis-tls | 16666 | TLS Redis |
| ssh-jump | 12222 | SSH tunnel (user `testuser`, key auth) |
| socks5 | 1080 | SOCKS5 proxy (host network) |
| sentinel | 26379–26381 | Sentinel (master on 16380) |
| redis-cluster | 7000–7005 | Cluster (host network) |

Automated integration tests (requires stack running):

```bash
./test/verify.sh
```

Or:

```bash
LAZYREDIS_TEST_ROOT=/path/to/lazyredis go test -tags=integration ./internal/store -run TestIntegrationConnections -count=1
```

Generated profiles in `test/profiles.generated.yaml`:

| Profile | Tests |
|---------|-------|
| test-standalone | Basic Redis on :16379 |
| test-tls | TLS on :16666 |
| test-ssh | SSH jump → Redis |
| test-ssh-tls | SSH + TLS |
| test-socks5 | SOCKS5 proxy |
| test-sentinel | Sentinel on :26379–26381 |
| test-cluster | Cluster on :7000–7005 (host network) |
| test-secure-remote | SOCKS5 + SSH + TLS combined |

Port note: sentinel master uses **16380**, cluster uses **7000–7005**, SOCKS5 uses **1080** (host network). These avoid conflicting with a local Redis on :6379.

Manual TUI test:

```bash
cp test/profiles.generated.yaml ~/.config/lazyredis/profiles.yaml
go build -o lazyredis ./cmd/lazyredis
./lazyredis
```

Stop the lab:

```bash
cd test && ./down.sh
```

Skip integration tests in short mode:

```bash
go test -short ./...
```

## License

See repository license file if present.
