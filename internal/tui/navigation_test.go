package tui

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/bloodynite/lazyredis/internal/config"
	"github.com/bloodynite/lazyredis/internal/store"
)

func TestStaleKeyDetailDropped(t *testing.T) {
	m := New()
	m.Screen = ScreenBrowser
	m.Client = &store.Client{}
	m.SelectedKey = "k"
	m.detailGen = 1

	// Stale response for the previous selection must not overwrite state.
	next, _ := m.Update(keyDetailMsg{
		detail: &store.KeyDetail{Meta: store.KeyMeta{Type: "string", Key: "stale"}, String: "stale"},
		key:    "stale",
		gen:    0,
	})
	m = next.(*Model)
	if m.KeyDetail != nil {
		t.Fatalf("stale detail should be dropped, got %v", m.KeyDetail)
	}

	// Fresh response with matching key + gen is accepted.
	next, _ = m.Update(keyDetailMsg{
		detail: &store.KeyDetail{Meta: store.KeyMeta{Type: "string", Key: "k"}, String: "ok"},
		key:    "k",
		gen:    1,
	})
	m = next.(*Model)
	if m.KeyDetail == nil || m.KeyDetail.String != "ok" {
		t.Fatalf("fresh detail not applied, got %+v", m.KeyDetail)
	}
}

func TestKeysLoadedPreservesKeyScroll(t *testing.T) {
	m := New()
	m.Width = 120
	m.Height = 40
	m.Screen = ScreenBrowser
	m.Client = &store.Client{}
	m.Keys = []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j",
		"k", "l", "m", "n", "o", "p", "q", "r", "s", "t",
		"u", "v", "w", "x", "y", "z"}
	m.scanGen = 1
	m.KeyCursor = 12
	m.KeyScroll = 8
	m.SelectedKey = "m"

	// Refresh replaces the key list. The selected key is still present, so
	// the cursor must follow it and the scroll window must be preserved.
	next, _ := m.Update(keysLoadedMsg{
		keys: m.Keys,
		gen:  1,
	})
	m = next.(*Model)
	if m.KeyCursor != 12 {
		t.Fatalf("KeyCursor = %d, want 12", m.KeyCursor)
	}
	if m.KeyScroll != 8 {
		t.Fatalf("KeyScroll = %d, want 8 (refresh must not reset scroll)", m.KeyScroll)
	}
	if m.SelectedKey != "m" {
		t.Fatalf("SelectedKey = %q, want m", m.SelectedKey)
	}
}

func TestKeysLoadedClampsCursorWhenSelectedKeyGone(t *testing.T) {
	m := New()
	m.Screen = ScreenBrowser
	m.Client = &store.Client{}
	m.scanGen = 1
	m.Keys = []string{"a", "b", "c"}
	m.KeyCursor = 2
	m.SelectedKey = "c"

	// Refresh drops the selected key entirely. Cursor clamps to last item
	// instead of snapping to 0.
	next, _ := m.Update(keysLoadedMsg{
		keys: []string{"a", "b"},
		gen:  1,
	})
	m = next.(*Model)
	if m.KeyCursor != 1 {
		t.Fatalf("KeyCursor = %d, want 1", m.KeyCursor)
	}
	if m.SelectedKey != "b" {
		t.Fatalf("SelectedKey = %q, want b", m.SelectedKey)
	}
}

func TestStaleAutoRefreshDropped(t *testing.T) {
	m := New()
	m.Screen = ScreenBrowser
	m.Client = &store.Client{}
	m.Config = &config.File{}
	m.refreshGen = 7

	// Tick from an older refresh generation must drop the data refresh
	// but keep the timer alive so auto-refresh does not silently die.
	next, cmd := m.Update(autoRefreshMsg{gen: 5})
	m = next.(*Model)
	if cmd == nil {
		t.Fatal("stale auto refresh must still reschedule the next tick")
	}
	if m.Loading {
		t.Fatal("stale auto refresh should not trigger refresh")
	}
	if m.refreshGen != 7 {
		t.Fatalf("stale auto refresh must not advance refreshGen, got %d, want 7", m.refreshGen)
	}
	if msg := cmd(); msg == nil {
		t.Fatal("stale reschedule must yield a tick")
	} else if arm, ok := msg.(autoRefreshMsg); !ok {
		t.Fatalf("stale reschedule: expected autoRefreshMsg, got %T", msg)
	} else if arm.gen != 5 {
		t.Fatalf("stale reschedule: autoRefreshMsg.gen = %d, want 5 (must carry stale gen)", arm.gen)
	}

	// Current generation tick reschedules normally.
	m.refreshGen = 8
	next, cmd = m.Update(autoRefreshMsg{gen: 8})
	m = next.(*Model)
	if cmd == nil {
		t.Fatal("current auto refresh must reschedule")
	}
	if !m.Loading {
		t.Fatal("current auto refresh must trigger refresh")
	}
	if m.refreshGen != 9 {
		t.Fatalf("current auto refresh must advance refreshGen, got %d, want 9", m.refreshGen)
	}
}

func TestMoveKeyCursorSchedulesDebounce(t *testing.T) {
	m := New()
	m.Screen = ScreenBrowser
	m.Client = &store.Client{}
	m.Keys = []string{"a", "b", "c"}
	m.SelectedKey = "a"
	m.KeyCursor = 0
	m.detailGen = 1

	next, cmd := m.moveKeyCursor(1)
	m = next.(*Model)
	if m.SelectedKey != "b" {
		t.Fatalf("SelectedKey = %q, want b", m.SelectedKey)
	}
	if m.detailGen != 2 {
		t.Fatalf("detailGen = %d, want 2 (moved)", m.detailGen)
	}
	if cmd == nil {
		t.Fatal("move should schedule debounced detail load")
	}
	// Debounce timer must not have already fired: the command must be a
	// tea.Tick that yields detailDebounceMsg, not an immediate loadKeyDetail.
	msg := cmd()
	if _, ok := msg.(detailDebounceMsg); !ok {
		t.Fatalf("expected detailDebounceMsg, got %T", msg)
	}
}

func TestDebounceDroppedAfterSelectionChanged(t *testing.T) {
	m := New()
	m.Screen = ScreenBrowser
	m.Client = &store.Client{}
	m.Keys = []string{"a", "b", "c"}
	m.SelectedKey = "a"
	m.KeyCursor = 0
	m.detailGen = 1

	// Simulate debounce fired for "a" but user has since navigated to "b".
	m.SelectedKey = "b"
	m.KeyCursor = 1
	m.detailGen = 2

	next, cmd := m.Update(detailDebounceMsg{key: "a", gen: 1})
	m = next.(*Model)
	if cmd != nil {
		t.Fatal("stale debounce must not trigger loadKeyDetail")
	}
	if m.Loading {
		t.Fatal("stale debounce must not flip loading")
	}
}

func TestScheduleAutoRefreshIncrementsGen(t *testing.T) {
	m := New()
	m.Client = &store.Client{}
	m.Config = &config.File{}
	m.refreshGen = 1

	cmd := m.scheduleAutoRefreshCmd()
	if cmd == nil {
		t.Fatal("expected refresh command")
	}
	if m.refreshGen != 2 {
		t.Fatalf("refreshGen = %d, want 2 after scheduling", m.refreshGen)
	}
	// Issued tick must carry the new gen so a parallel older tick gets dropped.
	msg := cmd()
	arm, ok := msg.(autoRefreshMsg)
	if !ok {
		t.Fatalf("expected autoRefreshMsg, got %T", msg)
	}
	if arm.gen != 2 {
		t.Fatalf("autoRefreshMsg.gen = %d, want 2", arm.gen)
	}
}

func TestHashFieldsSorted(t *testing.T) {
	// Regression: hashFields used to bubble-sort with a nested loop and was
	// O(n²). Sorted order must still be correct after the rewrite.
	fields := hashFields(map[string]string{
		"banana": "1",
		"apple":  "2",
		"cherry": "3",
	})
	if len(fields) != 3 || fields[0] != "apple" || fields[1] != "banana" || fields[2] != "cherry" {
		t.Fatalf("hashFields not sorted: %v", fields)
	}
}

// detailDebounceDuration is exposed via the command file but re-asserted here
// to guarantee the value used in moveKeyCursor matches what tests expect.
func TestDetailDebounceDurationPositive(t *testing.T) {
	if detailDebounceDuration <= 0 {
		t.Fatal("detailDebounceDuration must be > 0")
	}
	if detailDebounceDuration > 500*time.Millisecond {
		t.Fatal("detailDebounceDuration too large, navigation would feel laggy")
	}
	_ = tea.KeyMsg{} // keep import in case future tests need it
}