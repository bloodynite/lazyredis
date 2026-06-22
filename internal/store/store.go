package store

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/bloodynite/lazyredis/internal/config"
	"github.com/redis/go-redis/v9"
)

type Client struct {
	rdb       redis.UniversalClient
	profile   config.Profile
	resources *connResources
}

type ServerInfo struct {
	Version      string
	Uptime       string
	Connected    string
	UsedMemory   string
	PeakMemory   string
	TotalKeys    int64
	OpsPerSec    string
	Role         string
	UsedMemoryHR string
}

type KeyMeta struct {
	Key  string
	Type string
	TTL  time.Duration
}

// KeySummary is the metadata-only view of a key, fetched in a single
// pipeline so the UI can show type/ttl and decide whether to fetch the
// full value lazily. For composite types Total is the O(1) length
// (HLEN/LLEN/SCARD/ZCARD/XLEN); for strings Total is STRLEN. Total is
// -1 if the type does not expose a length or the lookup failed.
type KeySummary struct {
	Meta  KeyMeta
	Total int64
}

type KeyDetail struct {
	Meta   KeyMeta
	String string
	Hash   map[string]string
	List   []string
	Set    []string
	ZSet   []redis.Z
	Stream []StreamEntry
}

type StreamEntry struct {
	ID     string
	Fields map[string]string
}

func needsCustomDial(p config.Profile) bool {
	if p.Proxy != nil && p.Proxy.Type != "" && p.Proxy.Addr != "" {
		return true
	}
	if p.SSHTunnel != nil && p.SSHTunnel.Enabled {
		return true
	}
	return false
}

func Connect(ctx context.Context, p config.Profile) (*Client, error) {
	bundle, err := prepareDial(p)
	if err != nil {
		return nil, err
	}

	var rdb redis.UniversalClient
	switch p.Mode {
	case config.ModeCluster:
		opts := &redis.ClusterOptions{
			Addrs:    nonEmptyAddrs(p),
			Password: p.Password,
		}
		applyDial(bundle, opts)
		rdb = redis.NewClusterClient(opts)
	case config.ModeSentinel:
		if p.MasterName == "" {
			_ = bundle.resources.Close()
			return nil, fmt.Errorf("master_name required for sentinel mode")
		}
		opts := &redis.FailoverOptions{
			MasterName:       p.MasterName,
			SentinelAddrs:    nonEmptyAddrs(p),
			Password:         p.Password,
			SentinelUsername: p.SentinelUsername,
			SentinelPassword: p.SentinelPassword,
			DB:               p.DB,
		}
		applyDial(bundle, opts)
		rdb = redis.NewFailoverClient(opts)
	default:
		opts := &redis.Options{
			Addr:     p.Addr,
			Password: p.Password,
			DB:       p.DB,
		}
		applyDial(bundle, opts)
		rdb = redis.NewClient(opts)
	}
	if err := rdb.Ping(ctx).Err(); err != nil {
		_ = rdb.Close()
		_ = bundle.resources.Close()
		return nil, err
	}
	return &Client{rdb: rdb, profile: p, resources: bundle.resources}, nil
}

func nonEmptyAddrs(p config.Profile) []string {
	if len(p.Addrs) > 0 {
		return p.Addrs
	}
	if p.Addr != "" {
		return []string{p.Addr}
	}
	return nil
}

func (c *Client) Close() error {
	if c == nil {
		return nil
	}
	var err error
	if c.rdb != nil {
		err = c.rdb.Close()
	}
	if closeErr := c.resources.Close(); closeErr != nil && err == nil {
		err = closeErr
	}
	return err
}

func (c *Client) Profile() config.Profile {
	return c.profile
}

func (c *Client) Ping(ctx context.Context) error {
	return c.rdb.Ping(ctx).Err()
}

func (c *Client) Info(ctx context.Context) (*ServerInfo, error) {
	raw, err := c.rdb.Info(ctx, "server", "clients", "memory", "stats", "replication").Result()
	if err != nil {
		return nil, err
	}
	m := parseInfo(raw)
	dbSize, err := c.rdb.DBSize(ctx).Result()
	if err != nil {
		return nil, err
	}
	return &ServerInfo{
		Version:      m["redis_version"],
		Uptime:       m["uptime_in_seconds"],
		Connected:    m["connected_clients"],
		UsedMemory:   m["used_memory_human"],
		PeakMemory:   m["used_memory_peak_human"],
		TotalKeys:    dbSize,
		OpsPerSec:    m["instantaneous_ops_per_sec"],
		Role:         m["role"],
		UsedMemoryHR: m["used_memory_human"],
	}, nil
}

func parseInfo(raw string) map[string]string {
	out := make(map[string]string)
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 {
			out[parts[0]] = parts[1]
		}
	}
	return out
}

func NormalizeScanPattern(input string) string {
	pattern := strings.TrimSpace(input)
	if pattern == "" || pattern == "*" {
		return "*"
	}
	if strings.ContainsAny(pattern, "*?[") {
		return pattern
	}
	return "*" + pattern + "*"
}

func (c *Client) ScanKeys(ctx context.Context, cursor uint64, pattern string, count int64) ([]string, uint64, error) {
	pattern = NormalizeScanPattern(pattern)
	return c.rdb.Scan(ctx, cursor, pattern, count).Result()
}

func (c *Client) KeyMeta(ctx context.Context, key string) (*KeyMeta, error) {
	exists, err := c.rdb.Exists(ctx, key).Result()
	if err != nil {
		return nil, err
	}
	if exists == 0 {
		return nil, fmt.Errorf("key not found")
	}
	t, err := c.rdb.Type(ctx, key).Result()
	if err != nil {
		return nil, err
	}
	ttl, err := c.rdb.TTL(ctx, key).Result()
	if err != nil {
		return nil, err
	}
	return &KeyMeta{Key: key, Type: t, TTL: ttl}, nil
}

// GetKey returns the key detail. offset and limit control the entry
// window for composite types (hash/list/set/zset/stream). offset < 0 or
// limit <= 0 means "load everything" (legacy full-load behaviour). The
// returned KeyDetail only contains the requested window; combine with
// GetKeySummary to learn the total length without paying for a full
// fetch.
func (c *Client) GetKey(ctx context.Context, key string, offset, limit int) (*KeyDetail, error) {
	meta, err := c.KeyMeta(ctx, key)
	if err != nil {
		return nil, err
	}
	d := &KeyDetail{Meta: *meta}
	full := offset < 0 || limit <= 0
	switch meta.Type {
	case "string":
		val, err := c.rdb.Get(ctx, key).Result()
		if err != nil {
			return nil, err
		}
		d.String = val
	case "hash":
		d.Hash = c.loadHashWindow(ctx, key, offset, limit, full)
	case "list":
		var stop int64 = -1
		if !full {
			stop = int64(offset + limit - 1)
		}
		val, err := c.rdb.LRange(ctx, key, int64(offset), stop).Result()
		if err != nil {
			return nil, err
		}
		d.List = val
	case "set":
		d.Set = c.loadSetWindow(ctx, key, offset, limit, full)
	case "zset":
		var stop int64 = -1
		if !full {
			stop = int64(offset + limit - 1)
		}
		val, err := c.rdb.ZRangeWithScores(ctx, key, int64(offset), stop).Result()
		if err != nil {
			return nil, err
		}
		d.ZSet = val
	case "stream":
		d.Stream = c.loadStreamWindow(ctx, key, offset, limit, full)
	default:
		return nil, fmt.Errorf("unsupported type %s", meta.Type)
	}
	return d, nil
}

// GetKeySummary returns type, TTL, and O(1) length for the key.
// Callers can show metadata immediately and decide whether to fetch
// the full value lazily based on Total.
func (c *Client) GetKeySummary(ctx context.Context, key string) (*KeySummary, error) {
	t, err := c.rdb.Type(ctx, key).Result()
	if err != nil {
		return nil, err
	}
	if t == "none" {
		return nil, fmt.Errorf("key not found")
	}

	pipe := c.rdb.Pipeline()
	ttlCmd := pipe.TTL(ctx, key)
	var lenCmd *redis.IntCmd
	switch t {
	case "string":
		lenCmd = pipe.StrLen(ctx, key)
	case "hash":
		lenCmd = pipe.HLen(ctx, key)
	case "list":
		lenCmd = pipe.LLen(ctx, key)
	case "set":
		lenCmd = pipe.SCard(ctx, key)
	case "zset":
		lenCmd = pipe.ZCard(ctx, key)
	case "stream":
		lenCmd = pipe.XLen(ctx, key)
	}
	if _, err := pipe.Exec(ctx); err != nil && err != redis.Nil {
		return nil, err
	}

	ttl, err := ttlCmd.Result()
	if err != nil {
		return nil, err
	}
	total := int64(-1)
	if lenCmd != nil {
		if v, err := lenCmd.Result(); err == nil {
			total = v
		}
	}
	return &KeySummary{Meta: KeyMeta{Key: key, Type: t, TTL: ttl}, Total: total}, nil
}

// loadHashWindow returns hash entries for the requested window. Full
// mode uses HGETALL. Paged mode walks HSCAN cursors until it has the
// requested window. The field order is whatever HSCAN yields.
func (c *Client) loadHashWindow(ctx context.Context, key string, offset, limit int, full bool) map[string]string {
	if full {
		v, err := c.rdb.HGetAll(ctx, key).Result()
		if err != nil {
			return nil
		}
		return v
	}
	out := make(map[string]string, limit)
	var cursor uint64
	seen := 0
	for {
		batch, next, err := c.rdb.HScan(ctx, key, cursor, "", int64(limit)).Result()
		if err != nil {
			return out
		}
		for i := 0; i+1 < len(batch); i += 2 {
			if seen >= offset {
				out[batch[i]] = batch[i+1]
				if len(out) >= limit {
					return out
				}
			}
			seen++
		}
		if next == 0 {
			return out
		}
		cursor = next
	}
}

// loadSetWindow returns set members for the requested window via SSCAN.
// The order is whatever SSCAN yields.
func (c *Client) loadSetWindow(ctx context.Context, key string, offset, limit int, full bool) []string {
	if full {
		v, err := c.rdb.SMembers(ctx, key).Result()
		if err != nil {
			return nil
		}
		return v
	}
	out := make([]string, 0, limit)
	var cursor uint64
	seen := 0
	for {
		batch, next, err := c.rdb.SScan(ctx, key, cursor, "", int64(limit)).Result()
		if err != nil {
			return out
		}
		for _, m := range batch {
			if seen >= offset {
				out = append(out, m)
				if len(out) >= limit {
					return out
				}
			}
			seen++
		}
		if next == 0 {
			return out
		}
		cursor = next
	}
}

// loadStreamWindow returns stream entries for the requested window,
// ordered by ID ascending. Full mode uses XRange without COUNT (an explicit
// COUNT 0 is rejected by Redis with "redis: nil" — see fix note in commit).
// Paged mode uses XRangeN with COUNT=limit and skips offset items.
func (c *Client) loadStreamWindow(ctx context.Context, key string, offset, limit int, full bool) []StreamEntry {
	var msgs []redis.XMessage
	if full {
		var err error
		msgs, err = c.rdb.XRange(ctx, key, "-", "+").Result()
		if err != nil {
			return nil
		}
	} else {
		var err error
		msgs, err = c.rdb.XRangeN(ctx, key, "-", "+", int64(limit)).Result()
		if err != nil {
			return nil
		}
		if offset > 0 {
			if offset >= len(msgs) {
				return []StreamEntry{}
			}
			msgs = msgs[offset:]
		}
	}
	out := make([]StreamEntry, 0, len(msgs))
	for _, msg := range msgs {
		fields := make(map[string]string, len(msg.Values))
		for k, v := range msg.Values {
			fields[k] = fmt.Sprint(v)
		}
		out = append(out, StreamEntry{ID: msg.ID, Fields: fields})
	}
	return out
}

func (c *Client) SetString(ctx context.Context, key, value string, ttl time.Duration) error {
	return c.rdb.Set(ctx, key, value, ttl).Err()
}

func (c *Client) SetHashField(ctx context.Context, key, field, value string) error {
	return c.rdb.HSet(ctx, key, field, value).Err()
}

type KeyBody struct {
	String string
	Hash   map[string]string
	List   []string
	Set    []string
	ZSet   []redis.Z
	Stream []StreamEntry
}

func NormalizeKeyType(keyType string) string {
	switch strings.ToLower(strings.TrimSpace(keyType)) {
	case "hash", "list", "set", "zset", "stream":
		return strings.ToLower(strings.TrimSpace(keyType))
	default:
		return "string"
	}
}

func EncodeKeyBody(d *KeyDetail) string {
	switch d.Meta.Type {
	case "string":
		return d.String
	case "hash":
		fields := make([]string, 0, len(d.Hash))
		for k := range d.Hash {
			fields = append(fields, k)
		}
		sortStrings(fields)
		lines := make([]string, 0, len(fields))
		for _, f := range fields {
			lines = append(lines, f+"="+d.Hash[f])
		}
		return strings.Join(lines, "\n")
	case "list":
		return strings.Join(d.List, "\n")
	case "set":
		members := append([]string(nil), d.Set...)
		sortStrings(members)
		return strings.Join(members, "\n")
	case "zset":
		lines := make([]string, len(d.ZSet))
		for i, z := range d.ZSet {
			member, _ := z.Member.(string)
			lines[i] = FormatZSetLine(z.Score, member)
		}
		return strings.Join(lines, "\n")
	case "stream":
		var lines []string
		for _, e := range d.Stream {
			names := make([]string, 0, len(e.Fields))
			for k := range e.Fields {
				names = append(names, k)
			}
			sortStrings(names)
			for _, name := range names {
				lines = append(lines, e.ID+"\t"+name+"="+e.Fields[name])
			}
		}
		return strings.Join(lines, "\n")
	default:
		return ""
	}
}

func ParseKeyBody(keyType, raw string) (KeyBody, error) {
	switch keyType {
	case "string":
		return KeyBody{String: raw}, nil
	case "hash":
		fields := map[string]string{}
		for i, line := range strings.Split(raw, "\n") {
			line = strings.TrimRight(line, "\r")
			if strings.TrimSpace(line) == "" {
				continue
			}
			parts := strings.SplitN(line, "=", 2)
			if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" {
				return KeyBody{}, fmt.Errorf("hash line %d: use field=value", i+1)
			}
			fields[strings.TrimSpace(parts[0])] = parts[1]
		}
		return KeyBody{Hash: fields}, nil
	case "list":
		var items []string
		for _, line := range strings.Split(raw, "\n") {
			line = strings.TrimRight(line, "\r")
			if line == "" {
				continue
			}
			items = append(items, line)
		}
		return KeyBody{List: items}, nil
	case "set":
		var members []string
		seen := map[string]struct{}{}
		for i, line := range strings.Split(raw, "\n") {
			line = strings.TrimRight(line, "\r")
			if line == "" {
				continue
			}
			if _, ok := seen[line]; ok {
				return KeyBody{}, fmt.Errorf("set line %d: duplicate member", i+1)
			}
			seen[line] = struct{}{}
			members = append(members, line)
		}
		return KeyBody{Set: members}, nil
	case "zset":
		var items []redis.Z
		for i, line := range strings.Split(raw, "\n") {
			line = strings.TrimRight(line, "\r")
			if strings.TrimSpace(line) == "" {
				continue
			}
			score, member, err := ParseZSetLine(line)
			if err != nil {
				return KeyBody{}, fmt.Errorf("zset line %d: %w", i+1, err)
			}
			items = append(items, redis.Z{Score: score, Member: member})
		}
		return KeyBody{ZSet: items}, nil
	case "stream":
		order := []string{}
		byID := map[string]map[string]string{}
		for i, line := range strings.Split(raw, "\n") {
			line = strings.TrimRight(line, "\r")
			if strings.TrimSpace(line) == "" {
				continue
			}
			id, rest, ok := strings.Cut(line, "\t")
			if !ok {
				return KeyBody{}, fmt.Errorf("stream line %d: use id\\tfield=value", i+1)
			}
			id = strings.TrimSpace(id)
			if id == "" {
				return KeyBody{}, fmt.Errorf("stream line %d: missing entry id", i+1)
			}
			parts := strings.SplitN(rest, "=", 2)
			if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" {
				return KeyBody{}, fmt.Errorf("stream line %d: use id\\tfield=value", i+1)
			}
			field := strings.TrimSpace(parts[0])
			if byID[id] == nil {
				byID[id] = map[string]string{}
				order = append(order, id)
			}
			byID[id][field] = parts[1]
		}
		items := make([]StreamEntry, 0, len(order))
		for _, id := range order {
			items = append(items, StreamEntry{ID: id, Fields: byID[id]})
		}
		return KeyBody{Stream: items}, nil
	default:
		return KeyBody{}, fmt.Errorf("unsupported type %s", keyType)
	}
}

func (c *Client) WriteKey(ctx context.Context, key, keyType string, body KeyBody, ttl time.Duration) error {
	if err := c.rdb.Del(ctx, key).Err(); err != nil {
		return err
	}
	switch keyType {
	case "string":
		exp := ttl
		if exp < 0 {
			exp = 0
		}
		return c.rdb.Set(ctx, key, body.String, exp).Err()
	case "hash":
		if len(body.Hash) == 0 {
			return c.applyKeyTTL(ctx, key, ttl)
		}
		if err := c.rdb.HSet(ctx, key, body.Hash).Err(); err != nil {
			return err
		}
		return c.applyKeyTTL(ctx, key, ttl)
	case "list":
		if len(body.List) > 0 {
			args := make([]interface{}, len(body.List))
			for i, item := range body.List {
				args[i] = item
			}
			if err := c.rdb.RPush(ctx, key, args...).Err(); err != nil {
				return err
			}
		}
		return c.applyKeyTTL(ctx, key, ttl)
	case "set":
		members := make([]interface{}, len(body.Set))
		for i, m := range body.Set {
			members[i] = m
		}
		if len(members) > 0 {
			if err := c.rdb.SAdd(ctx, key, members...).Err(); err != nil {
				return err
			}
		}
		return c.applyKeyTTL(ctx, key, ttl)
	case "zset":
		if len(body.ZSet) > 0 {
			if err := c.rdb.ZAdd(ctx, key, body.ZSet...).Err(); err != nil {
				return err
			}
		}
		return c.applyKeyTTL(ctx, key, ttl)
	case "stream":
		for _, entry := range body.Stream {
			id := entry.ID
			if id == "" {
				id = "*"
			}
			if len(entry.Fields) == 0 {
				continue
			}
			if err := c.rdb.XAdd(ctx, &redis.XAddArgs{
				Stream: key,
				ID:     id,
				Values: entry.Fields,
			}).Err(); err != nil {
				return err
			}
		}
		return c.applyKeyTTL(ctx, key, ttl)
	default:
		return fmt.Errorf("unsupported type %s", keyType)
	}
}

func (c *Client) applyKeyTTL(ctx context.Context, key string, ttl time.Duration) error {
	if ttl < 0 {
		return c.rdb.Persist(ctx, key).Err()
	}
	if ttl == 0 {
		return nil
	}
	return c.rdb.Expire(ctx, key, ttl).Err()
}

func FormatZSetLine(score float64, member string) string {
	return strconv.FormatFloat(score, 'g', -1, 64) + "\t" + member
}

func ParseZSetLine(line string) (float64, string, error) {
	if idx := strings.Index(line, "\t"); idx >= 0 {
		score, err := strconv.ParseFloat(strings.TrimSpace(line[:idx]), 64)
		if err != nil {
			return 0, "", fmt.Errorf("invalid score")
		}
		return score, line[idx+1:], nil
	}
	parts := strings.SplitN(strings.TrimSpace(line), " ", 2)
	if len(parts) != 2 {
		return 0, "", fmt.Errorf("use score<TAB>member or score member")
	}
	score, err := strconv.ParseFloat(parts[0], 64)
	if err != nil {
		return 0, "", fmt.Errorf("invalid score")
	}
	return score, parts[1], nil
}

func sortStrings(items []string) {
	for i := 0; i < len(items); i++ {
		for j := i + 1; j < len(items); j++ {
			if items[j] < items[i] {
				items[i], items[j] = items[j], items[i]
			}
		}
	}
}

func (c *Client) DeleteHashField(ctx context.Context, key, field string) error {
	return c.rdb.HDel(ctx, key, field).Err()
}

func (c *Client) SetListItem(ctx context.Context, key string, index int, value string) error {
	return c.rdb.LSet(ctx, key, int64(index), value).Err()
}

func (c *Client) DeleteListItem(ctx context.Context, key string, index int) error {
	val, err := c.rdb.LIndex(ctx, key, int64(index)).Result()
	if err != nil {
		return err
	}
	return c.rdb.LRem(ctx, key, 1, val).Err()
}

func (c *Client) AppendListItem(ctx context.Context, key, value string) error {
	return c.rdb.RPush(ctx, key, value).Err()
}

func (c *Client) SetAddMember(ctx context.Context, key, member string) error {
	return c.rdb.SAdd(ctx, key, member).Err()
}

func (c *Client) SetRemoveMember(ctx context.Context, key, member string) error {
	return c.rdb.SRem(ctx, key, member).Err()
}

func (c *Client) SetReplaceMember(ctx context.Context, key, oldMember, newMember string) error {
	if oldMember == newMember {
		return nil
	}
	pipe := c.rdb.TxPipeline()
	pipe.SRem(ctx, key, oldMember)
	pipe.SAdd(ctx, key, newMember)
	_, err := pipe.Exec(ctx)
	return err
}

func (c *Client) ZSetAddMember(ctx context.Context, key string, score float64, member string) error {
	return c.rdb.ZAdd(ctx, key, redis.Z{Score: score, Member: member}).Err()
}

func (c *Client) ZSetRemoveMember(ctx context.Context, key, member string) error {
	return c.rdb.ZRem(ctx, key, member).Err()
}

func (c *Client) ZSetReplaceMember(ctx context.Context, key string, oldMember, newMember string, newScore float64) error {
	pipe := c.rdb.TxPipeline()
	pipe.ZRem(ctx, key, oldMember)
	pipe.ZAdd(ctx, key, redis.Z{Score: newScore, Member: newMember})
	_, err := pipe.Exec(ctx)
	return err
}

func (c *Client) StreamAddEntry(ctx context.Context, key, id string, fields map[string]string) error {
	if id == "" {
		id = "*"
	}
	return c.rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: key,
		ID:     id,
		Values: fields,
	}).Err()
}

func (c *Client) StreamDeleteEntry(ctx context.Context, key, id string) error {
	return c.rdb.XDel(ctx, key, id).Err()
}

func (c *Client) StreamReplaceEntry(ctx context.Context, key, id string, fields map[string]string) error {
	if err := c.rdb.XDel(ctx, key, id).Err(); err != nil {
		return err
	}
	return c.StreamAddEntry(ctx, key, id, fields)
}

func EncodeStreamFields(fields map[string]string) string {
	names := make([]string, 0, len(fields))
	for k := range fields {
		names = append(names, k)
	}
	sortStrings(names)
	lines := make([]string, 0, len(names))
	for _, name := range names {
		lines = append(lines, name+"="+fields[name])
	}
	return strings.Join(lines, "\n")
}

func ParseHashFieldLine(line string) (string, string, error) {
	parts := strings.SplitN(line, "=", 2)
	if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" {
		return "", "", fmt.Errorf("use field=value")
	}
	return strings.TrimSpace(parts[0]), parts[1], nil
}

func ParseStreamFields(raw string) (map[string]string, error) {
	fields := map[string]string{}
	for i, line := range strings.Split(raw, "\n") {
		line = strings.TrimRight(line, "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		field, value, err := ParseHashFieldLine(line)
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", i+1, err)
		}
		fields[field] = value
	}
	if len(fields) == 0 {
		return nil, fmt.Errorf("at least one field=value required")
	}
	return fields, nil
}

func (c *Client) SetTTL(ctx context.Context, key string, ttl time.Duration) error {
	if ttl < 0 {
		return c.rdb.Persist(ctx, key).Err()
	}
	return c.rdb.Expire(ctx, key, ttl).Err()
}

func (c *Client) DeleteKey(ctx context.Context, key string) error {
	return c.rdb.Del(ctx, key).Err()
}

func (c *Client) FlushDB(ctx context.Context) error {
	return c.rdb.FlushDB(ctx).Err()
}

func FormatTTL(ttl time.Duration) string {
	switch {
	case ttl == -2*time.Second:
		return "does not exist"
	case ttl == -1*time.Second:
		return "no expiry"
	case ttl == 0:
		return "0s"
	default:
		return ttl.Round(time.Second).String()
	}
}

func ParseTTLInput(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if s == "" || s == "-" || strings.EqualFold(s, "persist") {
		return -1, nil
	}
	if d, err := time.ParseDuration(s); err == nil {
		return d, nil
	}
	if sec, err := strconv.Atoi(s); err == nil {
		return time.Duration(sec) * time.Second, nil
	}
	return 0, fmt.Errorf("invalid ttl: use 3600s, 1h, 300 or persist")
}
