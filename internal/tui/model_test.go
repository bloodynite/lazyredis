package tui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/bloodynite/lazyredis/internal/config"
	"github.com/bloodynite/lazyredis/internal/store"
)

func TestProfilesLoadedStartup(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	m := New()
	msg := loadProfiles()()
	pl, ok := msg.(profilesLoadedMsg)
	if !ok {
		t.Fatalf("unexpected message type %T", msg)
	}
	if pl.err != nil {
		t.Fatal(pl.err)
	}
	if pl.cfg == nil {
		t.Fatal("loadProfiles returned nil cfg")
	}

	next, _ := m.Update(pl)
	m = next.(*Model)
	if m.Config == nil {
		t.Fatal("model config is nil after profilesLoadedMsg")
	}
	if len(m.Profiles) == 0 {
		t.Fatal("expected default profiles")
	}
}

func TestProfilesNavigation(t *testing.T) {
	m := New()
	m.Profiles = []config.Profile{
		{Name: "a", Addr: "127.0.0.1:6379"},
		{Name: "b", Addr: "127.0.0.1:6380"},
	}
	m.Screen = ScreenProfiles

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = next.(*Model)
	if m.ProfileCursor != 1 {
		t.Fatalf("cursor = %d, want 1", m.ProfileCursor)
	}

	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m = next.(*Model)
	if m.ProfileCursor != 0 {
		t.Fatalf("cursor = %d, want 0", m.ProfileCursor)
	}
}

func TestConfirmCancelReturnsToPreviousScreen(t *testing.T) {
	m := New()
	m.Screen = ScreenConfirm
	m.PrevScreen = ScreenBrowser
	m.ConfirmAction = confirmDeleteKey

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	m = next.(*Model)
	if m.Screen != ScreenBrowser {
		t.Fatalf("screen = %v, want Browser", m.Screen)
	}
	if m.ConfirmAction != confirmNone {
		t.Fatal("confirm action should reset")
	}
}

func TestSearchPatternAppliedLive(t *testing.T) {
	m := New()
	m.Screen = ScreenBrowser
	m.Client = &store.Client{}
	m.SearchFocus = true
	m.SearchInput.Focus()
	m.SearchInput.SetValue("user")

	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{':'}})
	m = next.(*Model)
	if m.ScanPattern != "*user:*" {
		t.Fatalf("pattern = %q, want *user:*", m.ScanPattern)
	}
	if cmd == nil {
		t.Fatal("expected scan command on live filter")
	}
}

func TestSearchPatternApplied(t *testing.T) {
	m := New()
	m.Screen = ScreenBrowser
	m.Client = &store.Client{}
	m.SearchFocus = true
	m.SearchInput.Focus()
	m.SearchInput.SetValue("user:*")

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(*Model)
	if m.ScanPattern != "user:*" {
		t.Fatalf("pattern = %q, want user:*", m.ScanPattern)
	}
	if m.SearchFocus {
		t.Fatal("search should blur after enter")
	}
}

func TestStaleScanResultIgnored(t *testing.T) {
	m := New()
	m.Screen = ScreenBrowser
	m.Client = &store.Client{}
	m.scanGen = 2
	m.Loading = true

	next, _ := m.Update(keysLoadedMsg{keys: []string{"stale"}, gen: 1})
	m = next.(*Model)
	if len(m.Keys) != 0 {
		t.Fatalf("expected stale keys ignored, got %v", m.Keys)
	}
	if !m.Loading {
		t.Fatal("loading should stay true for stale scan")
	}

	next, _ = m.Update(keysLoadedMsg{keys: []string{"fresh"}, gen: 2})
	m = next.(*Model)
	if len(m.Keys) != 1 || m.Keys[0] != "fresh" {
		t.Fatalf("keys = %v, want [fresh]", m.Keys)
	}
}

func TestAutoRefreshSkippedWhenEditing(t *testing.T) {
	m := New()
	m.Screen = ScreenBrowser
	m.Client = &store.Client{}
	m.Config = &config.File{}
	m.Loading = false

	next, cmd := m.Update(autoRefreshMsg{})
	m = next.(*Model)
	if !m.Loading {
		t.Fatal("auto refresh should run on browser screen")
	}
	if cmd == nil {
		t.Fatal("expected reschedule command")
	}

	m.Screen = ScreenKeyEdit
	m.Loading = false
	next, _ = m.Update(autoRefreshMsg{})
	m = next.(*Model)
	if m.Loading {
		t.Fatal("auto refresh should not run on KeyEdit screen")
	}
}

func TestStartEditOpensKeyFormModal(t *testing.T) {
	m := New()
	m.Client = &store.Client{}
	m.SelectedKey = "demo:key"
	m.KeyDetail = &store.KeyDetail{
		Meta:   store.KeyMeta{Key: "demo:key", Type: "string", TTL: 300 * time.Second},
		String: "hello",
	}

	next, cmd := m.startEdit()
	m = next.(*Model)
	if m.EditMode != editExistingKey {
		t.Fatalf("edit mode = %v, want editExistingKey", m.EditMode)
	}
	if m.Screen != ScreenKeyEdit {
		t.Fatal("expected key edit screen")
	}
	if m.NewKeyName.Value() != "demo:key" {
		t.Fatalf("key = %q", m.NewKeyName.Value())
	}
	if m.NewKeyValue.Value() != "hello" {
		t.Fatalf("value = %q", m.NewKeyValue.Value())
	}
	if m.NewKeyTTL.Value() != "300" {
		t.Fatalf("ttl = %q", m.NewKeyTTL.Value())
	}
	if cmd == nil {
		t.Fatal("expected focus command")
	}
}

func TestOpenTTLModal(t *testing.T) {
	m := New()
	m.Client = &store.Client{}
	m.SelectedKey = "demo:key"
	m.KeyDetail = &store.KeyDetail{
		Meta: store.KeyMeta{Key: "demo:key", Type: "string", TTL: 300 * time.Second},
	}

	next, cmd := m.openTTLModal()
	m = next.(*Model)
	if m.EditMode != editTTL {
		t.Fatalf("edit mode = %v, want editTTL", m.EditMode)
	}
	if m.Screen != ScreenKeyEdit {
		t.Fatal("expected key edit screen")
	}
	if !m.editUsesModal() {
		t.Fatal("ttl edit should use modal")
	}
	if m.NewKeyTTL.Value() != "300" {
		t.Fatalf("ttl = %q", m.NewKeyTTL.Value())
	}
	if cmd == nil {
		t.Fatal("expected focus command")
	}
}

func TestKeysLoadedSetsScanCursorBeforeDetailLoad(t *testing.T) {
	m := New()
	m.Screen = ScreenBrowser
	m.Client = &store.Client{}
	m.scanGen = 1

	next, cmd := m.Update(keysLoadedMsg{
		keys:   []string{"a", "b"},
		cursor: 42,
		gen:    1,
	})
	m = next.(*Model)
	if m.ScanCursor != 42 {
		t.Fatalf("ScanCursor = %d, want 42", m.ScanCursor)
	}
	if cmd == nil {
		t.Fatal("expected loadKeyDetail command")
	}
}

func TestKeysLoadedAppendPreservesCursor(t *testing.T) {
	m := New()
	m.Screen = ScreenBrowser
	m.Client = &store.Client{}
	m.scanGen = 1
	m.Keys = []string{"a"}

	next, _ := m.Update(keysLoadedMsg{
		keys:   []string{"b"},
		cursor: 99,
		append: true,
		gen:    1,
	})
	m = next.(*Model)
	if len(m.Keys) != 2 {
		t.Fatalf("keys = %v", m.Keys)
	}
	if m.ScanCursor != 99 {
		t.Fatalf("ScanCursor = %d, want 99", m.ScanCursor)
	}
}

func TestKeysPanelMetaPagination(t *testing.T) {
	m := New()
	m.Info = &store.ServerInfo{TotalKeys: 210}
	m.Keys = make([]string, 101)
	m.ScanCursor = 1

	meta := m.keysPanelMeta()
	if !strings.Contains(meta, "101/210") {
		t.Fatalf("meta = %q, want loaded/total", meta)
	}
	if !strings.Contains(meta, "g") {
		t.Fatalf("meta = %q, want g hint", meta)
	}

	m.ScanCursor = 0
	m.Keys = make([]string, 210)
	meta = m.keysPanelMeta()
	if meta != "210/210 keys" {
		t.Fatalf("meta = %q, want 210/210 keys", meta)
	}
}

func TestWindowSizeMsg(t *testing.T) {
	m := New()

	next, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	m = next.(*Model)
	if m.Width != 100 || m.Height != 40 {
		t.Fatalf("unexpected size %dx%d", m.Width, m.Height)
	}
}

func TestCopyableValueText(t *testing.T) {
	m := New()
	m.KeyDetail = &store.KeyDetail{
		Meta:   store.KeyMeta{Type: "string"},
		String: "hello",
	}
	text, ok := m.copyableValueText()
	if !ok || text != "hello" {
		t.Fatalf("string copy = %q ok=%v", text, ok)
	}

	m.KeyDetail = &store.KeyDetail{
		Meta: store.KeyMeta{Type: "hash"},
		Hash: map[string]string{"name": "redis"},
	}
	text, ok = m.copyableValueText()
	if !ok || text != "redis" {
		t.Fatalf("hash copy = %q ok=%v", text, ok)
	}

	m.DetailCursor = 0
	m.KeyDetail.List = []string{"a", "b"}
	m.KeyDetail.Meta.Type = "list"
	text, ok = m.copyableValueText()
	if !ok || text != "a" {
		t.Fatalf("list copy = %q ok=%v", text, ok)
	}
}

func TestStatusClearAfterCopy(t *testing.T) {
	m := New()
	m.KeyDetail = &store.KeyDetail{
		Meta:   store.KeyMeta{Type: "string"},
		String: "hello",
	}
	m.SelectedKey = "demo:key"

	next, cmd := m.copyDetailValue()
	m = next.(*Model)
	if m.Status != copiedToClipboardStatus {
		t.Fatalf("status = %q", m.Status)
	}
	if cmd == nil {
		t.Fatal("expected clear status command")
	}

	next, _ = m.Update(statusClearMsg{gen: m.statusClearGen})
	m = next.(*Model)
	if m.Status != "" {
		t.Fatalf("status = %q, want empty", m.Status)
	}

	m.Status = "key saved"
	next, _ = m.Update(statusClearMsg{gen: 0})
	m = next.(*Model)
	if m.Status != "key saved" {
		t.Fatalf("status = %q, want key saved", m.Status)
	}
}

func TestStatusClearAfterFlush(t *testing.T) {
	m := New()
	m.Screen = ScreenBrowser
	m.Client = &store.Client{}

	next, _ := m.Update(actionDoneMsg{status: "database flushed"})
	m = next.(*Model)
	if m.Status != "database flushed" {
		t.Fatalf("status = %q", m.Status)
	}

	next, _ = m.Update(statusClearMsg{gen: m.statusClearGen})
	m = next.(*Model)
	if m.Status != "" {
		t.Fatalf("status = %q, want empty", m.Status)
	}
}

func TestBrowserCopyBind(t *testing.T) {
	m := New()
	m.Width = 120
	m.Height = 24
	m.Screen = ScreenBrowser
	m.SelectedKey = "demo:key"
	m.KeyDetail = &store.KeyDetail{
		Meta:   store.KeyMeta{Type: "string", Key: "demo:key"},
		String: "hello",
	}
	if !m.matchAction(actionBrowserCopy, "c") {
		t.Fatal("expected c to match copy action")
	}
	bar := m.renderKeybar()
	if !strings.Contains(bar, "copy value") {
		t.Fatalf("keybar = %q, want copy value bind", bar)
	}
}
