package tui

import (
	"strings"
	"testing"

	"github.com/bloodynite/lazyredis/internal/store"
)

func TestKeybarForBrowser(t *testing.T) {
	m := New()
	m.Width = 120
	m.Height = 24
	m.Screen = ScreenBrowser

	binds := m.keyBinds()
	if len(binds) < 6 {
		t.Fatalf("expected browser binds, got %d", len(binds))
	}

	bar := m.renderKeybar()
	if bar == "" {
		t.Fatal("expected non-empty keybar")
	}
}

func TestKeybarWhenEditing(t *testing.T) {
	m := New()
	m.Screen = ScreenKeyEdit
	m.EditMode = editElement
	m.EditInput.Focus()

	binds := m.keyBinds()
	if len(binds) != 4 {
		t.Fatalf("expected edit binds, got %d", len(binds))
	}

	m.EditMode = editNewKey
	binds = m.keyBinds()
	if len(binds) != 6 {
		t.Fatalf("expected new key binds, got %d", len(binds))
	}
	foundCtrlS := false
	for _, b := range binds {
		if strings.Contains(b.Key, "ctrl+s") {
			foundCtrlS = true
			break
		}
	}
	if !foundCtrlS {
		t.Fatal("expected ctrl+s save bind")
	}
}

func TestKeybarShowsEditOnListType(t *testing.T) {
	m := New()
	m.Screen = ScreenBrowser
	m.Client = &store.Client{}
	m.SelectedKey = "demo:list"
	m.KeyDetail = &store.KeyDetail{
		Meta: store.KeyMeta{Type: "list"},
		List: []string{"a"},
	}

	binds := m.keyBinds()
	found := false
	for _, b := range binds {
		if strings.Contains(b.Desc, "edit key") {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("edit key should be available for list type")
	}
}

func TestKeybarPinsContextActions(t *testing.T) {
	m := New()
	m.Width = 60
	m.Height = 24
	m.Screen = ScreenBrowser
	m.Client = &store.Client{}
	m.SelectedKey = "demo:key"
	m.KeyDetail = &store.KeyDetail{
		Meta: store.KeyMeta{Type: "string", Key: "demo:key"},
		String: "hello",
	}

	bar := m.renderKeybar()
	if !strings.Contains(bar, "edit key") {
		t.Fatal("expected pinned edit key bind in keybar")
	}
	if !strings.Contains(bar, "delete") {
		t.Fatal("expected pinned delete bind in keybar")
	}
}

func TestBrowserPanelWidths(t *testing.T) {
	m := New()
	m.Width = 80
	left, right := m.browserPanelWidths()
	if left < 16 {
		t.Fatalf("left too small: %d", left)
	}
	if left+right != 80 {
		t.Fatalf("panels should fill width: %d + %d", left, right)
	}
}

func TestKeybarWrapsToSecondLine(t *testing.T) {
	m := New()
	m.Width = 50
	m.Height = 24
	m.Screen = ScreenBrowser
	m.Client = &store.Client{}
	m.ScanCursor = 1
	m.SelectedKey = "demo:key"
	m.KeyDetail = &store.KeyDetail{
		Meta: store.KeyMeta{Type: "string", Key: "demo:key"},
		String: "hello",
	}

	if m.keybarLineCount() < 2 {
		t.Fatalf("expected wrapped keybar, got %d lines", m.keybarLineCount())
	}
	bar := m.renderKeybar()
	if !strings.Contains(bar, "\n") {
		t.Fatal("expected multiline keybar")
	}
	if !strings.Contains(bar, "load more keys") {
		t.Fatal("expected load more keys bind in keybar")
	}
}

func TestLayoutHeights(t *testing.T) {
	m := New()
	m.Height = 24

	content, status := m.layoutHeights()
	if content != m.panelAreaLines() {
		t.Fatalf("content = %d, want %d", content, m.panelAreaLines())
	}
	if status != gridStatusRows {
		t.Fatalf("status lines = %d", status)
	}
}

func TestStatusLineShowsMessageOnly(t *testing.T) {
	m := New()
	m.Width = 120
	m.Client = &store.Client{}
	m.Status = "copied to clipboard"

	line := m.renderStatusLine()
	if !strings.Contains(line, "copied to clipboard") {
		t.Fatalf("status line = %q, want copy message", line)
	}
	if strings.Contains(line, "keys") {
		t.Fatalf("status line should not include server stats: %q", line)
	}
}

func TestInfoLinesShowServerStats(t *testing.T) {
	m := New()
	m.Width = 120
	m.Client = &store.Client{}
	m.Info = &store.ServerInfo{
		Version:    "7.2",
		UsedMemory: "1M",
		TotalKeys:  10,
	}

	line2 := m.renderInfoLine2()
	if !strings.Contains(line2, "keys") {
		t.Fatalf("info line 2 = %q, want server stats", line2)
	}
}

// TestKeybarDetailFocusHidesKeysOnlyActions verifies that when the detail
// panel is focused, the keybar does not surface keys-panel-only bindings
// (refresh, auto-refresh, new key, load more keys). Those bindings stay
// reachable via the keys panel and the help modal.
func TestKeybarDetailFocusHidesKeysOnlyActions(t *testing.T) {
	m := New()
	m.Width = 120
	m.Height = 24
	m.Screen = ScreenBrowser
	m.Client = &store.Client{}
	m.PanelFocus = panelDetail
	m.SelectedKey = "demo:key"
	m.KeyDetail = &store.KeyDetail{
		Meta:   store.KeyMeta{Type: "string", Key: "demo:key"},
		String: "hello",
	}

	binds := m.keyBinds()
	for _, b := range binds {
		switch b.Desc {
		case "refresh", "auto refresh", "new key", "load more keys":
			t.Fatalf("detail focus must not show keys-panel-only action %q", b.Desc)
		}
	}

	bar := m.renderKeybar()
	for _, hidden := range []string{"refresh", "auto refresh", "new key", "load more keys"} {
		if strings.Contains(bar, hidden) {
			t.Fatalf("detail focus keybar must not mention %q; got %q", hidden, bar)
		}
	}
}

// TestKeybarDetailFocusShowsSingleFilter verifies the dedup invariant: when
// the detail panel is focused, the keybar shows exactly one binding for `/`
// (the detail-scoped "search value"), not the keys-panel-scoped "filter"
// alongside it.
func TestKeybarDetailFocusShowsSingleFilter(t *testing.T) {
	m := New()
	m.Width = 120
	m.Height = 24
	m.Screen = ScreenBrowser
	m.Client = &store.Client{}
	m.PanelFocus = panelDetail
	m.SelectedKey = "demo:key"
	m.KeyDetail = &store.KeyDetail{
		Meta:   store.KeyMeta{Type: "string", Key: "demo:key"},
		String: "hello",
	}

	binds := m.keyBinds()
	filterCount := 0
	searchValueCount := 0
	genericFilterCount := 0
	for _, b := range binds {
		if b.Key == "/" {
			genericFilterCount++
			switch b.Desc {
			case "filter":
				filterCount++
			case "search value":
				searchValueCount++
			}
		}
	}
	if genericFilterCount != 1 {
		t.Fatalf("expected exactly one '/' binding in detail focus, got %d (filter=%d, search value=%d)",
			genericFilterCount, filterCount, searchValueCount)
	}
	if searchValueCount != 1 {
		t.Fatalf("expected detail focus keybar to surface 'search value' for '/', got %d", searchValueCount)
	}
	if filterCount != 0 {
		t.Fatalf("detail focus must not surface the keys-panel 'filter' desc, got %d", filterCount)
	}

	bar := m.renderKeybar()
	if strings.Count(bar, "search value") != 1 {
		t.Fatalf("keybar should contain exactly one 'search value' mention; got %q", bar)
	}
	if strings.Contains(bar, " filter ") || strings.HasSuffix(bar, " filter") {
		t.Fatalf("keybar must not contain a bare 'filter' desc in detail focus; got %q", bar)
	}
}

// TestKeybarKeysFocusShowsFilterNotSearchValue verifies the inverse: when the
// keys panel is focused (the user is browsing the key list), the keybar shows
// the keys-panel-scoped "filter" binding, not the detail-panel "search value".
func TestKeybarKeysFocusShowsFilterNotSearchValue(t *testing.T) {
	m := New()
	m.Width = 120
	m.Height = 24
	m.Screen = ScreenBrowser
	m.Client = &store.Client{}
	m.PanelFocus = panelKeys
	m.SelectedKey = "demo:key"
	m.KeyDetail = &store.KeyDetail{
		Meta:   store.KeyMeta{Type: "string", Key: "demo:key"},
		String: "hello",
	}

	binds := m.keyBinds()
	for _, b := range binds {
		if b.Key == "/" && b.Desc != "filter" {
			t.Fatalf("keys focus '/' binding should describe 'filter', got %q", b.Desc)
		}
	}
}

// TestKeybarDetailFocusStillShowsHelpAndForceQuit ensures the always-pinned
// global bindings remain on line 2 even when the detail panel is focused.
// Users must always see how to dismiss help and force-quit.
func TestKeybarDetailFocusStillShowsHelpAndForceQuit(t *testing.T) {
	m := New()
	m.Width = 120
	m.Height = 24
	m.Screen = ScreenBrowser
	m.Client = &store.Client{}
	m.PanelFocus = panelDetail
	m.SelectedKey = "demo:key"
	m.KeyDetail = &store.KeyDetail{
		Meta:   store.KeyMeta{Type: "string", Key: "demo:key"},
		String: "hello",
	}

	bar := m.renderKeybar()
	if !strings.Contains(bar, "help") {
		t.Fatalf("keybar should always show help bind; got %q", bar)
	}
	if !strings.Contains(bar, "force quit") {
		t.Fatalf("keybar should always show force quit bind; got %q", bar)
	}
}

// TestKeybarAppliesToCompositeDetail verifies that switching the key type to a
// composite (list) exposes the composite-only detail actions (add/edit/delete
// item) in the keybar.
func TestKeybarAppliesToCompositeDetail(t *testing.T) {
	m := New()
	m.Width = 120
	m.Height = 24
	m.Screen = ScreenBrowser
	m.Client = &store.Client{}
	m.PanelFocus = panelDetail
	m.SelectedKey = "demo:list"
	m.KeyDetail = &store.KeyDetail{
		Meta: store.KeyMeta{Type: "list", Key: "demo:list"},
		List: []string{"a", "b"},
	}

	binds := m.keyBinds()
	descs := map[string]bool{}
	for _, b := range binds {
		descs[b.Desc] = true
	}
	for _, want := range []string{"add item", "edit item", "delete item", "search value"} {
		if !descs[want] {
			t.Fatalf("composite detail focus should expose %q; got %v", want, descs)
		}
	}
}

// TestApplicableHelpActionsDedupsAcrossScopes is a structural check: even
// though helpGroups produces both a Keys-panel and a Detail-panel group with
// the same binding id (e.g. actionBrowserCopy, actionBrowserTTL), the flat
// applicableHelpActions list — which drives the keybar — must contain each
// id at most once.
func TestApplicableHelpActionsDedupsAcrossScopes(t *testing.T) {
	m := New()
	m.Width = 120
	m.Height = 24
	m.Screen = ScreenBrowser
	m.Client = &store.Client{}
	m.SelectedKey = "demo:key"
	m.KeyDetail = &store.KeyDetail{
		Meta:   store.KeyMeta{Type: "string", Key: "demo:key"},
		String: "hello",
	}

	seen := map[string]int{}
	for _, d := range m.applicableHelpActions() {
		seen[d.id]++
	}
	for id, count := range seen {
		if count > 1 {
			t.Fatalf("applicableHelpActions duplicate id %q (%d entries); keybar would render duplicates", id, count)
		}
	}
}

// TestKeybarHidesTtlAndAutoRefreshEverywhere verifies the keybar never
// surfaces ttl or auto refresh entries on the browser screen, regardless of
// panel focus. Those shortcuts stay reachable via the key handlers and the
// help modal.
func TestKeybarHidesTtlAndAutoRefreshEverywhere(t *testing.T) {
	cases := []struct {
		name  string
		focus panelFocus
	}{
		{"keys focus", panelKeys},
		{"detail focus", panelDetail},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := New()
			m.Width = 120
			m.Height = 24
			m.Screen = ScreenBrowser
			m.Client = &store.Client{}
			m.PanelFocus = tc.focus
			m.SelectedKey = "demo:key"
			m.KeyDetail = &store.KeyDetail{
				Meta:   store.KeyMeta{Type: "string", Key: "demo:key"},
				String: "hello",
			}

			for _, b := range m.keyBinds() {
				if b.Desc == "ttl" || b.Desc == "auto refresh" {
					t.Fatalf("%s keybar must not show %q", tc.name, b.Desc)
				}
			}
			main, pinned := m.keybarBinds()
			for _, b := range append(append([]keyBind{}, main...), pinned...) {
				if b.Desc == "ttl" || b.Desc == "auto refresh" {
					t.Fatalf("%s keybar (split) must not show %q", tc.name, b.Desc)
				}
			}

			bar := m.renderKeybar()
			for _, hidden := range []string{"ttl", "auto refresh"} {
				if strings.Contains(bar, hidden) {
					t.Fatalf("%s keybar must not mention %q; got %q", tc.name, hidden, bar)
				}
			}
		})
	}
}

// TestKeybarNoLongerExposesTtlInApplicableHelpActions guards the structural
// invariant behind TestKeybarHidesTtlAndAutoRefreshEverywhere: the action
// list that feeds the keybar must no longer include actionBrowserTTL or
// actionBrowserAutoRefresh on the browser screen.
func TestKeybarNoLongerExposesTtlInApplicableHelpActions(t *testing.T) {
	m := New()
	m.Width = 120
	m.Height = 24
	m.Screen = ScreenBrowser
	m.Client = &store.Client{}
	m.SelectedKey = "demo:key"
	m.KeyDetail = &store.KeyDetail{
		Meta:   store.KeyMeta{Type: "string", Key: "demo:key"},
		String: "hello",
	}

	for _, d := range m.applicableHelpActions() {
		if d.id == actionBrowserTTL || d.id == actionBrowserAutoRefresh {
			t.Fatalf("applicableHelpActions must not include %q; keybar would surface it", d.id)
		}
	}
}
