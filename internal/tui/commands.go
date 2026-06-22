package tui

import (
	"context"
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/bloodynite/lazyredis/internal/config"
	"github.com/bloodynite/lazyredis/internal/store"
)

type profilesLoadedMsg struct {
	cfg *config.File
	err error
}

type connectedMsg struct {
	client *store.Client
	err    error
}

type infoLoadedMsg struct {
	info *store.ServerInfo
	err  error
}

type keysLoadedMsg struct {
	keys    []string
	cursor  uint64
	append  bool
	pattern string
	gen     uint64
	err     error
}

type keyDetailMsg struct {
	detail      *store.KeyDetail
	err         error
	key         string
	gen         uint64
	chunk       bool
	appendOff   int
	appendLimit int
}

type detailDebounceMsg struct {
	key string
	gen uint64
}

type keySummaryMsg struct {
	summary *store.KeySummary
	err     error
	key     string
	gen     uint64
}

type actionDoneMsg struct {
	status string
	err    error
	reload bool
}

type autoRefreshMsg struct {
	gen uint64
}

type statusClearMsg struct {
	gen uint64
}

func loadProfiles() tea.Cmd {
	return func() tea.Msg {
		cfg, err := config.EnsureDefault()
		if err == nil && cfg == nil {
			cfg = config.DefaultProfiles()
		}
		return profilesLoadedMsg{cfg: cfg, err: err}
	}
}

func connectProfile(p config.Profile) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		client, err := store.Connect(ctx, p)
		return connectedMsg{client: client, err: err}
	}
}

func loadInfo(client *store.Client) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		info, err := client.Info(ctx)
		return infoLoadedMsg{info: info, err: err}
	}
}

func scanKeys(client *store.Client, cursor uint64, pattern string, appendKeys bool, gen uint64) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		pattern = store.NormalizeScanPattern(pattern)
		var keys []string
		cur := cursor
		for {
			batch, next, err := client.ScanKeys(ctx, cur, pattern, 100)
			if err != nil {
				return keysLoadedMsg{err: err, pattern: pattern, gen: gen, append: appendKeys}
			}
			keys = append(keys, batch...)
			cur = next
			if cur == 0 {
				break
			}
			if appendKeys {
				if len(batch) > 0 {
					break
				}
				continue
			}
			// TODO: non-append path breaks after first batch, so initial
			// connect/auto-refresh only loads 100 keys. Auto-refresh then
			// replaces m.Keys with first 100 again, dropping pagination.
			if len(batch) > 0 {
				break
			}
		}
		return keysLoadedMsg{keys: keys, cursor: cur, append: appendKeys, pattern: pattern, gen: gen}
	}
}

var (
	loadKeyDetailFn  = loadKeyDetail
	loadKeySummaryFn = loadKeySummary
)

func loadKeyDetail(client *store.Client, key string, offset, limit int, gen uint64, chunk bool) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		detail, err := client.GetKey(ctx, key, offset, limit)
		return keyDetailMsg{
			detail:     detail,
			err:        err,
			key:        key,
			gen:        gen,
			chunk:      chunk,
			appendOff:  offset,
			appendLimit: limit,
		}
	}
}

func loadKeySummary(client *store.Client, key string, gen uint64) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		summary, err := client.GetKeySummary(ctx, key)
		return keySummaryMsg{summary: summary, err: err, key: key, gen: gen}
	}
}

func scheduleDetailDebounce(key string, gen uint64) tea.Cmd {
	return tea.Tick(detailDebounceDuration, func(time.Time) tea.Msg {
		return detailDebounceMsg{key: key, gen: gen}
	})
}

func saveKeyBody(client *store.Client, key, keyType string, body store.KeyBody, ttl time.Duration, renameFrom string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := client.WriteKey(ctx, key, keyType, body, ttl); err != nil {
			return actionDoneMsg{err: err}
		}
		if renameFrom != "" && renameFrom != key {
			if err := client.DeleteKey(ctx, renameFrom); err != nil {
				return actionDoneMsg{err: err}
			}
		}
		return actionDoneMsg{status: "key saved", reload: true}
	}
}

func saveStringKey(client *store.Client, key, value string, ttl time.Duration, renameFrom string) tea.Cmd {
	return saveKeyBody(client, key, "string", store.KeyBody{String: value}, ttl, renameFrom)
}

func setHashField(client *store.Client, key, field, value string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		err := client.SetHashField(ctx, key, field, value)
		if err != nil {
			return actionDoneMsg{err: err}
		}
		return actionDoneMsg{status: "field saved", reload: true}
	}
}

func setTTL(client *store.Client, key string, ttl time.Duration) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		err := client.SetTTL(ctx, key, ttl)
		if err != nil {
			return actionDoneMsg{err: err}
		}
		return actionDoneMsg{status: "ttl updated", reload: true}
	}
}

func deleteKey(client *store.Client, key string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		err := client.DeleteKey(ctx, key)
		if err != nil {
			return actionDoneMsg{err: err}
		}
		return actionDoneMsg{status: "key deleted", reload: false}
	}
}

func flushDB(client *store.Client) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		err := client.FlushDB(ctx)
		if err != nil {
			return actionDoneMsg{err: err}
		}
		return actionDoneMsg{status: "database flushed", reload: false}
	}
}

func saveProfile(cfg *config.File, p config.Profile) tea.Cmd {
	return func() tea.Msg {
		if cfg == nil {
			cfg = &config.File{}
		}
		if err := cfg.Upsert(p); err != nil {
			return actionDoneMsg{err: err}
		}
		updated, err := config.Load()
		if err != nil {
			return actionDoneMsg{err: err}
		}
		if updated == nil {
			updated = &config.File{}
		}
		return profilesLoadedMsg{cfg: updated}
	}
}

func scheduleAutoRefresh(d time.Duration, gen uint64) tea.Cmd {
	if d <= 0 {
		return nil
	}
	return tea.Tick(d, func(time.Time) tea.Msg {
		return autoRefreshMsg{gen: gen}
	})
}

func clearStatusAfter(d time.Duration, gen uint64) tea.Cmd {
	return tea.Tick(d, func(time.Time) tea.Msg {
		return statusClearMsg{gen: gen}
	})
}

func saveRefreshInterval(cfg *config.File, sec int) tea.Cmd {
	return func() tea.Msg {
		if cfg == nil {
			return actionDoneMsg{err: fmt.Errorf("config not loaded")}
		}
		if err := cfg.SetRefreshIntervalSec(sec); err != nil {
			return actionDoneMsg{err: err}
		}
		label := "off"
		if sec > 0 {
			label = fmt.Sprintf("%ds", sec)
		}
		return actionDoneMsg{status: "auto refresh " + label}
	}
}

func deleteProfile(cfg *config.File, name string) tea.Cmd {
	return func() tea.Msg {
		if cfg == nil {
			return actionDoneMsg{err: fmt.Errorf("config not loaded")}
		}
		if err := cfg.Delete(name); err != nil {
			return actionDoneMsg{err: err}
		}
		updated, err := config.Load()
		if err != nil {
			return actionDoneMsg{err: err}
		}
		if updated == nil {
			updated = &config.File{}
		}
		return profilesLoadedMsg{cfg: updated}
	}
}

func patchStringValue(client *store.Client, key, value string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := client.SetString(ctx, key, value, 0); err != nil {
			return actionDoneMsg{err: err}
		}
		return actionDoneMsg{status: "value saved", reload: true}
	}
}

func patchHashField(client *store.Client, key, field, value string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := client.SetHashField(ctx, key, field, value); err != nil {
			return actionDoneMsg{err: err}
		}
		return actionDoneMsg{status: "field saved", reload: true}
	}
}

func addHashField(client *store.Client, key, line string) tea.Cmd {
	return func() tea.Msg {
		field, value, err := store.ParseHashFieldLine(line)
		if err != nil {
			return actionDoneMsg{err: err}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := client.SetHashField(ctx, key, field, value); err != nil {
			return actionDoneMsg{err: err}
		}
		return actionDoneMsg{status: "field added", reload: true}
	}
}

func removeHashField(client *store.Client, key, field string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := client.DeleteHashField(ctx, key, field); err != nil {
			return actionDoneMsg{err: err}
		}
		return actionDoneMsg{status: "field deleted", reload: true}
	}
}

func patchListItem(client *store.Client, key string, index int, value string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := client.SetListItem(ctx, key, index, value); err != nil {
			return actionDoneMsg{err: err}
		}
		return actionDoneMsg{status: "item saved", reload: true}
	}
}

func appendListItem(client *store.Client, key, value string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := client.AppendListItem(ctx, key, value); err != nil {
			return actionDoneMsg{err: err}
		}
		return actionDoneMsg{status: "item added", reload: true}
	}
}

func removeListItem(client *store.Client, key string, index int) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := client.DeleteListItem(ctx, key, index); err != nil {
			return actionDoneMsg{err: err}
		}
		return actionDoneMsg{status: "item deleted", reload: true}
	}
}

func addSetMember(client *store.Client, key, member string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := client.SetAddMember(ctx, key, member); err != nil {
			return actionDoneMsg{err: err}
		}
		return actionDoneMsg{status: "member added", reload: true}
	}
}

func replaceSetMember(client *store.Client, key, oldMember, newMember string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := client.SetReplaceMember(ctx, key, oldMember, newMember); err != nil {
			return actionDoneMsg{err: err}
		}
		return actionDoneMsg{status: "member saved", reload: true}
	}
}

func removeSetMember(client *store.Client, key, member string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := client.SetRemoveMember(ctx, key, member); err != nil {
			return actionDoneMsg{err: err}
		}
		return actionDoneMsg{status: "member deleted", reload: true}
	}
}

func addZSetMember(client *store.Client, key, line string) tea.Cmd {
	return func() tea.Msg {
		score, member, err := store.ParseZSetLine(line)
		if err != nil {
			return actionDoneMsg{err: err}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := client.ZSetAddMember(ctx, key, score, member); err != nil {
			return actionDoneMsg{err: err}
		}
		return actionDoneMsg{status: "member added", reload: true}
	}
}

func replaceZSetMember(client *store.Client, key string, oldMember, line string) tea.Cmd {
	return func() tea.Msg {
		newScore, newMember, err := store.ParseZSetLine(line)
		if err != nil {
			return actionDoneMsg{err: err}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := client.ZSetReplaceMember(ctx, key, oldMember, newMember, newScore); err != nil {
			return actionDoneMsg{err: err}
		}
		return actionDoneMsg{status: "member saved", reload: true}
	}
}

func removeZSetMember(client *store.Client, key, member string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := client.ZSetRemoveMember(ctx, key, member); err != nil {
			return actionDoneMsg{err: err}
		}
		return actionDoneMsg{status: "member deleted", reload: true}
	}
}

func addStreamEntry(client *store.Client, key, raw string) tea.Cmd {
	return func() tea.Msg {
		fields, err := store.ParseStreamFields(raw)
		if err != nil {
			return actionDoneMsg{err: err}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := client.StreamAddEntry(ctx, key, "*", fields); err != nil {
			return actionDoneMsg{err: err}
		}
		return actionDoneMsg{status: "entry added", reload: true}
	}
}

func replaceStreamEntry(client *store.Client, key, id, raw string) tea.Cmd {
	return func() tea.Msg {
		fields, err := store.ParseStreamFields(raw)
		if err != nil {
			return actionDoneMsg{err: err}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := client.StreamReplaceEntry(ctx, key, id, fields); err != nil {
			return actionDoneMsg{err: err}
		}
		return actionDoneMsg{status: "entry saved", reload: true}
	}
}

func removeStreamEntry(client *store.Client, key, id string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := client.StreamDeleteEntry(ctx, key, id); err != nil {
			return actionDoneMsg{err: err}
		}
		return actionDoneMsg{status: "entry deleted", reload: true}
	}
}

var (
	patchStringValueFn    = patchStringValue
	patchHashFieldFn      = patchHashField
	addHashFieldFn        = addHashField
	patchListItemFn       = patchListItem
	appendListItemFn      = appendListItem
	addSetMemberFn        = addSetMember
	replaceSetMemberFn    = replaceSetMember
	addZSetMemberFn       = addZSetMember
	replaceZSetMemberFn   = replaceZSetMember
	addStreamEntryFn      = addStreamEntry
	replaceStreamEntryFn  = replaceStreamEntry
)
