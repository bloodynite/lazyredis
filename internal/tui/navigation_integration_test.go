//go:build integration

package tui

import (
	"context"
	"sort"
	"sync"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/bloodynite/lazyredis/internal/store"
)

// TestNavigationGlyphversoFirst10Keys walks the model through every adjacent
// pair of the first 10 alphabetically sorted glyphverso keys, measuring
// wall-clock time from moveKeyCursor to KeyDetail populated. The test fails
// if any single move exceeds navigationSloMs; it logs every measurement so a
// regression stands out.
//
// The harness drives Bubble Tea manually instead of running tea.NewProgram:
// moveKeyCursor returns a tea.Cmd that is a Tick. We invoke the cmd, deliver
// each resulting msg back into Update, and stop when KeyDetail for the
// selected key is in place.
//
// Run with:
//   go test -tags=integration -run TestNavigationGlyphversoFirst10Keys -v ./internal/tui
func TestNavigationGlyphversoFirst10Keys(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}

	p := loadGlyphversoProfileForTUI(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	client, err := store.Connect(ctx, p)
	if err != nil {
		t.Skipf("glyphverso unreachable at %s: %v", p.Addr, err)
	}
	t.Cleanup(func() { _ = client.Close() })

	keys, _, err := client.ScanKeys(ctx, 0, "*", 1000)
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if len(keys) == 0 {
		t.Skip("glyphverso has no keys")
	}
	sort.Strings(keys)
	if len(keys) > 10 {
		keys = keys[:10]
	}

	// Pin loadKeySummaryFn / loadKeyDetailFn to real fakes that talk to
	// glyphverso but record their elapsed time. We restore them on cleanup
	// so other tests in the same binary are unaffected.
	origSum, origDet := loadKeySummaryFn, loadKeyDetailFn
	t.Cleanup(func() {
		loadKeySummaryFn = origSum
		loadKeyDetailFn = origDet
	})

	var mu sync.Mutex
	type record struct {
		key       string
		keyType   string
		summaryMs time.Duration
		detailMs  time.Duration
		totalMs   time.Duration
	}
	records := make([]record, 0, len(keys)-1)
	var (
		moveStart    time.Time
		lastSummaryT time.Time
	)
	loadKeySummaryFn = func(c *store.Client, key string, gen uint64) tea.Cmd {
		return func() tea.Msg {
			mu.Lock()
			lastSummaryT = time.Now()
			mu.Unlock()
			s, err := c.GetKeySummary(ctx, key)
			mu.Lock()
			summaryMs := time.Since(lastSummaryT)
			if s != nil {
				for i := range records {
					if records[i].key == key && records[i].keyType == "" {
						records[i].keyType = s.Meta.Type
						records[i].summaryMs = summaryMs
						break
					}
				}
			}
			mu.Unlock()
			if err != nil {
				return keySummaryMsg{key: key, gen: gen, err: err}
			}
			return keySummaryMsg{key: key, gen: gen, summary: s}
		}
	}
	loadKeyDetailFn = func(c *store.Client, key string, offset, limit int, gen uint64, chunk bool) tea.Cmd {
		return func() tea.Msg {
			t0 := time.Now()
			d, err := c.GetKey(ctx, key, offset, limit)
			dur := time.Since(t0)
			mu.Lock()
			for i := range records {
				if records[i].key == key && records[i].detailMs == 0 {
					records[i].detailMs = dur
					records[i].totalMs = time.Since(moveStart)
					break
				}
			}
			mu.Unlock()
			if err != nil {
				return keyDetailMsg{key: key, gen: gen, err: err}
			}
			return keyDetailMsg{key: key, gen: gen, detail: d, chunk: chunk, appendOff: offset, appendLimit: limit}
		}
	}

	m := newBrowserWithKeys(t, keys)
	m.Client = client
	m.SelectedKey = keys[0]
	m.KeyCursor = 0
	m.detailGen = 1
	// Prime detail for the first key so subsequent moves are pure navigation.
	m.detailGen++
	primeModel(t, m, keys[0])

	t.Logf("navigating %d adjacent moves across keys: %v", len(keys)-1, keys)
	for i := 1; i < len(keys); i++ {
		moveStart = time.Now()
		mu.Lock()
		records = append(records, record{key: keys[i]})
		mu.Unlock()

		next, cmd := m.moveKeyCursor(1)
		m = next.(*Model)
		if m.SelectedKey != keys[i] {
			t.Fatalf("move %d: SelectedKey=%q, want %q", i, m.SelectedKey, keys[i])
		}

		// Walk the debounce tick. We invoke the cmd (a Tick) to receive
		// detailDebounceMsg, feed it back, then invoke the resulting
		// summary cmd, then the detail cmd.
		if cmd == nil {
			t.Fatalf("move %d: expected debounce cmd, got nil", i)
		}
		dbgMsg := cmd()
		dbg, ok := dbgMsg.(detailDebounceMsg)
		if !ok {
			t.Fatalf("move %d: debounce cmd returned %T, want detailDebounceMsg", i, dbgMsg)
		}
		next, cmd = m.Update(dbg)
		m = next.(*Model)
		if cmd == nil {
			t.Fatalf("move %d: summary cmd missing", i)
		}
		sumMsg := cmd()
		next, cmd = m.Update(sumMsg)
		m = next.(*Model)
		if cmd == nil {
			// Could be the end of a full-fetch path that needs another cmd.
			// Continue: if KeyDetail already populated, we're done.
		} else {
			detMsg := cmd()
			next, _ = m.Update(detMsg)
			m = next.(*Model)
		}

		if m.KeyDetail == nil || m.KeyDetail.Meta.Key != keys[i] {
			t.Fatalf("move %d to %q: KeyDetail not populated (got %+v)", i, keys[i], m.KeyDetail)
		}
		mu.Lock()
		rec := records[i-1]
		mu.Unlock()
		t.Logf("move %d -> %-30s | type=%-6s | summary=%-9s detail=%-9s total=%-9s",
			i, rec.key, rec.keyType,
			rec.summaryMs.Round(time.Microsecond),
			rec.detailMs.Round(time.Microsecond),
			rec.totalMs.Round(time.Microsecond))

		if rec.totalMs > navigationSloMs {
			t.Errorf("move %d to %s: total %s exceeds SLO %s",
				i, rec.key, rec.totalMs.Round(time.Millisecond), navigationSloMs)
		}
	}
}

// TestNavigationDebounceFloorIsTheBug proves the navigation floor is the
// 80ms debounce on moveKeyCursor, not the Redis round-trip. It uses fakes
// for summary/detail (each returns in <1ms) and walks one move, recording
// the wall-clock time from moveKeyCursor to KeyDetail populated. Without
// the debounce the test should finish in single-digit milliseconds; with
// it, ~80ms.
//
// Run with:
//   go test -tags=integration -run TestNavigationDebounceFloorIsTheBug -v ./internal/tui
func TestNavigationDebounceFloorIsTheBug(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	origSum, origDet := loadKeySummaryFn, loadKeyDetailFn
	t.Cleanup(func() {
		loadKeySummaryFn = origSum
		loadKeyDetailFn = origDet
	})

	loadKeySummaryFn = func(c *store.Client, key string, gen uint64) tea.Cmd {
		return func() tea.Msg {
			time.Sleep(50 * time.Microsecond) // fake summary work
			return keySummaryMsg{
				key:     key,
				gen:     gen,
				summary: &store.KeySummary{Meta: store.KeyMeta{Key: key, Type: "string"}, Total: 5},
			}
		}
	}
	loadKeyDetailFn = func(c *store.Client, key string, offset, limit int, gen uint64, chunk bool) tea.Cmd {
		return func() tea.Msg {
			time.Sleep(50 * time.Microsecond) // fake detail work
			return keyDetailMsg{
				key: key, gen: gen, chunk: chunk, appendOff: offset, appendLimit: limit,
				detail: &store.KeyDetail{Meta: store.KeyMeta{Key: key, Type: "string"}, String: "v"},
			}
		}
	}

	m := newBrowserWithKeys(t, []string{"a", "b"})
	m.Client = &store.Client{}
	m.SelectedKey = "a"
	m.KeyCursor = 0
	m.detailGen = 1
	primeModel(t, m, "a")

	t0 := time.Now()
	next, cmd := m.moveKeyCursor(1)
	m = next.(*Model)
	if cmd == nil {
		t.Fatal("expected debounce cmd")
	}
	dbgMsg := cmd()
	next, cmd = m.Update(dbgMsg)
	m = next.(*Model)
	if cmd == nil {
		t.Fatal("expected summary cmd")
	}
	sumMsg := cmd()
	next, cmd = m.Update(sumMsg)
	m = next.(*Model)
	if cmd == nil {
		t.Fatal("expected detail cmd")
	}
	detMsg := cmd()
	next, _ = m.Update(detMsg)
	m = next.(*Model)
	total := time.Since(t0)
	if m.KeyDetail == nil || m.KeyDetail.Meta.Key != "b" {
		t.Fatalf("detail not populated for b: %+v", m.KeyDetail)
	}
	t.Logf("navigation latency (Redis faked, debounce real): total=%s "+
		"(debounce floor=%s, fake fetch ~100µs)",
		total.Round(time.Millisecond), detailDebounceDuration)
	// With fake fetch in microseconds, any latency significantly above
	// detailDebounceDuration is the debounce. We assert the total sits
	// within the debounce window plus a generous margin for tick accuracy
	// and goroutine scheduling.
	upperBound := detailDebounceDuration + 100*time.Millisecond
	if total > upperBound {
		t.Errorf("navigation took %s; expected near %s (debounce) plus <100ms scheduling",
			total.Round(time.Millisecond), detailDebounceDuration)
	}
}

// newBrowserWithKeys returns a Model in the browser screen, Screen size
// enough to render every key, with the given key list already loaded.
func newBrowserWithKeys(t *testing.T, keys []string) *Model {
	t.Helper()
	m := New()
	m.Width = 200
	m.Height = 60
	m.Screen = ScreenBrowser
	m.PanelFocus = panelKeys
	m.Keys = append([]string(nil), keys...)
	m.scanGen = 1
	return m
}

// primeModel fires a synchronous summary + detail for the first key so the
// model has a populated KeyDetail before navigation starts. It mirrors the
// real loadKeySummaryFn/loadKeyDetailFn but bypasses the debounce.
func primeModel(t *testing.T, m *Model, key string) {
	t.Helper()
	sumCmd := loadKeySummaryFn(m.Client, key, m.detailGen)
	if sumCmd == nil {
		t.Fatal("prime: summary cmd nil")
	}
	next, cmd := m.Update(sumCmd())
	m = next.(*Model)
	if cmd == nil {
		t.Fatal("prime: detail cmd missing after summary")
	}
	detMsg := cmd()
	next, _ = m.Update(detMsg)
	m = next.(*Model)
	// Re-bind for the caller since Model is a value receiver on most helpers.
	*m = *next.(*Model)
	if m.KeyDetail == nil || m.KeyDetail.Meta.Key != key {
		t.Fatalf("prime: detail not populated for %s: %+v", key, m.KeyDetail)
	}
}

const navigationSloMs = 500 * time.Millisecond
