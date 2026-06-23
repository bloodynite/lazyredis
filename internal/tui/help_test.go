package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/bloodynite/lazyredis/internal/store"
)

// TestHelpModalGroupsBothBrowserSectionsInKeysFocus verifies the help modal
// always lists both the Keys-panel and Detail-panel groups when on the browser
// screen, even when the user is currently focused on the keys panel.
func TestHelpModalGroupsBothBrowserSectionsInKeysFocus(t *testing.T) {
	m := New()
	m.Width = 100
	m.Height = 30
	m.Screen = ScreenBrowser
	m.Client = &store.Client{}
	m.PanelFocus = panelKeys
	m.SelectedKey = "demo:key"
	m.KeyDetail = &store.KeyDetail{
		Meta:   store.KeyMeta{Type: "string", Key: "demo:key"},
		String: "hello",
	}

	out := m.renderHelpModal()
	if !strings.Contains(out, "Browser · Keys panel") {
		t.Fatalf("help modal should contain 'Browser · Keys panel' heading; got %q", out)
	}
	if !strings.Contains(out, "Browser · Detail panel") {
		t.Fatalf("help modal should contain 'Browser · Detail panel' heading; got %q", out)
	}
	if !strings.Contains(out, "Global") {
		t.Fatalf("help modal should contain 'Global' heading; got %q", out)
	}
	if !strings.Contains(out, "Help") {
		t.Fatalf("help modal should contain 'Help' (close) section; got %q", out)
	}
}

// TestHelpModalGroupsBothBrowserSectionsInDetailFocus verifies the help modal
// still lists both browser key groups when the detail panel is focused. The
// user can read what keys the other panel accepts from the same modal.
func TestHelpModalGroupsBothBrowserSectionsInDetailFocus(t *testing.T) {
	m := New()
	m.Width = 100
	m.Height = 30
	m.Screen = ScreenBrowser
	m.Client = &store.Client{}
	m.PanelFocus = panelDetail
	m.SelectedKey = "demo:list"
	m.KeyDetail = &store.KeyDetail{
		Meta: store.KeyMeta{Type: "list", Key: "demo:list"},
		List: []string{"a"},
	}

	out := m.renderHelpModal()
	if !strings.Contains(out, "Browser · Keys panel") {
		t.Fatalf("help modal in detail focus should still list 'Browser · Keys panel'; got %q", out)
	}
	if !strings.Contains(out, "Browser · Detail panel") {
		t.Fatalf("help modal in detail focus should list 'Browser · Detail panel'; got %q", out)
	}
}

// TestHelpModalShowsBothSlashDescriptions verifies that even though the keybar
// collapses `/` to a single entry based on focus, the help modal surfaces both
// contextual meanings of the same key under their respective group headings.
func TestHelpModalShowsBothSlashDescriptions(t *testing.T) {
	m := New()
	m.Width = 100
	m.Height = 30
	m.Screen = ScreenBrowser
	m.Client = &store.Client{}
	m.PanelFocus = panelDetail
	m.SelectedKey = "demo:key"
	m.KeyDetail = &store.KeyDetail{
		Meta:   store.KeyMeta{Type: "string", Key: "demo:key"},
		String: "hello",
	}

	out := m.renderHelpModal()
	// "filter" must appear under the Keys-panel section, "search value"
	// under the Detail-panel section.
	if !strings.Contains(out, "filter") {
		t.Fatalf("help modal should mention 'filter' under Keys panel; got %q", out)
	}
	if !strings.Contains(out, "search value") {
		t.Fatalf("help modal should mention 'search value' under Detail panel; got %q", out)
	}
	idxFilter := strings.Index(out, "filter")
	idxKeys := strings.Index(out, "Browser · Keys panel")
	idxDetail := strings.Index(out, "Browser · Detail panel")
	idxSearch := strings.Index(out, "search value")
	if !(idxKeys < idxFilter && idxFilter < idxDetail) {
		t.Fatalf("'filter' entry should be between Keys-panel and Detail-panel headings; keys=%d filter=%d detail=%d",
			idxKeys, idxFilter, idxDetail)
	}
	if !(idxDetail < idxSearch) {
		t.Fatalf("'search value' entry should appear after Detail-panel heading; detail=%d search=%d",
			idxDetail, idxSearch)
	}
}

// TestHelpModalOmitsKeysPanelOnlyActionsInDetailKeybarFocus is a regression
// guard for the keybar scoping: the keybar must not show refresh/auto refresh
// in detail focus even though the help modal still lists them (under the
// Keys-panel heading) for reference.
func TestHelpModalOmitsKeysPanelOnlyActionsInDetailKeybarFocus(t *testing.T) {
	m := New()
	m.Width = 100
	m.Height = 30
	m.Screen = ScreenBrowser
	m.Client = &store.Client{}
	m.PanelFocus = panelDetail
	m.SelectedKey = "demo:key"
	m.KeyDetail = &store.KeyDetail{
		Meta:   store.KeyMeta{Type: "string", Key: "demo:key"},
		String: "hello",
	}

	// Modal mentions refresh under Keys panel section.
	out := m.renderHelpModal()
	if !strings.Contains(out, "refresh") {
		t.Fatalf("help modal should reference refresh under Keys panel; got %q", out)
	}

	// But the keybar (which is focus-driven) must not surface it.
	bar := m.renderKeybar()
	if strings.Contains(bar, "refresh") || strings.Contains(bar, "auto refresh") {
		t.Fatalf("detail focus keybar must not surface refresh/auto refresh; got %q", bar)
	}
}

// TestHelpModalGroupOrderingInBrowserFocus asserts that in the browser
// screen the group headings appear in a stable order: Global, Browser ·
// Common, Browser · Keys panel, Browser · Detail panel.
func TestHelpModalGroupOrderingInBrowserFocus(t *testing.T) {
	m := New()
	m.Width = 100
	m.Height = 30
	m.Screen = ScreenBrowser
	m.Client = &store.Client{}
	m.PanelFocus = panelKeys

	out := m.renderHelpModal()
	order := []string{
		"Global",
		"Browser · Common",
		"Browser · Keys panel",
		"Browser · Detail panel",
	}
	last := -1
	for _, heading := range order {
		idx := strings.Index(out, heading)
		if idx < 0 {
			t.Fatalf("help modal missing heading %q; got %q", heading, out)
		}
		if idx <= last {
			t.Fatalf("help modal heading %q appears out of order; last=%d, idx=%d", heading, last, idx)
		}
		last = idx
	}
}

// TestHelpModalGroupsForProfilesScreen verifies the modal groups bind by the
// active screen so the modal reflects the current context (Profiles screen
// shows the Profiles group, no Browser groups).
func TestHelpModalGroupsForProfilesScreen(t *testing.T) {
	m := New()
	m.Width = 100
	m.Height = 30
	m.Screen = ScreenProfiles

	out := m.renderHelpModal()
	if !strings.Contains(out, "Profiles") {
		t.Fatalf("help modal on Profiles screen should include 'Profiles' group; got %q", out)
	}
	if strings.Contains(out, "Browser · Keys panel") || strings.Contains(out, "Browser · Detail panel") {
		t.Fatalf("help modal on Profiles screen should not include Browser groups; got %q", out)
	}
}

// TestHelpModalGroupsForDetailSearchFocus verifies the modal adds a Detail
// search group on top of the regular browser groups when the detail search
// input has the focus, so the user can see how to apply/close the search.
func TestHelpModalGroupsForDetailSearchFocus(t *testing.T) {
	m := New()
	m.Width = 100
	m.Height = 30
	m.Screen = ScreenBrowser
	m.Client = &store.Client{}
	m.PanelFocus = panelDetail
	m.SelectedKey = "demo:key"
	m.KeyDetail = &store.KeyDetail{
		Meta:   store.KeyMeta{Type: "string", Key: "demo:key"},
		String: "hello world",
	}
	m.DetailSearchFocus = true
	m.DetailSearchInput.SetValue("hello")

	out := m.renderHelpModal()
	if !strings.Contains(out, "Detail search") {
		t.Fatalf("help modal should show Detail search group when input is focused; got %q", out)
	}
	if !strings.Contains(out, "close search") {
		t.Fatalf("help modal should show 'close search' under Detail search; got %q", out)
	}
	if !strings.Contains(out, "apply search") {
		t.Fatalf("help modal should show 'apply search' under Detail search; got %q", out)
	}
}

// TestHelpModalOpensWithQuestionMark verifies the `?` key still toggles the
// help modal after the refactor — i.e. actionAppHelp still resolves on
// ScreenProfiles.
func TestHelpModalOpensWithQuestionMark(t *testing.T) {
	m := New()
	m.Width = 100
	m.Height = 30
	m.Screen = ScreenProfiles
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	m = next.(*Model)
	if !m.HelpOpen {
		t.Fatal("? should open help modal on Profiles screen")
	}
	// Render the full view: it must not panic and must contain the modal.
	out := m.View()
	if !strings.Contains(out, "Keyboard shortcuts") {
		t.Fatalf("view with help open should contain modal title; got %q", out)
	}
}

// TestHelpModalListsTtlAndAutoRefreshOnBrowserScreen verifies the help modal
// still surfaces ttl and auto refresh on the browser screen, even though
// the keybar hides those shortcuts. The defs come from browserHelpOnlyDefs
// under the "Browser · More" group.
func TestHelpModalListsTtlAndAutoRefreshOnBrowserScreen(t *testing.T) {
	m := New()
	m.Width = 100
	m.Height = 30
	m.Screen = ScreenBrowser
	m.Client = &store.Client{}
	m.SelectedKey = "demo:key"
	m.KeyDetail = &store.KeyDetail{
		Meta:   store.KeyMeta{Type: "string", Key: "demo:key"},
		String: "hello",
	}

	out := m.renderHelpModal()
	if !strings.Contains(out, "Browser · More") {
		t.Fatalf("help modal on browser screen should contain 'Browser · More' group; got %q", out)
	}
	idxGroup := strings.Index(out, "Browser · More")
	idxTTL := strings.Index(out, "ttl")
	if idxTTL < 0 {
		t.Fatalf("help modal on browser screen should list 'ttl'; got %q", out)
	}
	if idxTTL < idxGroup {
		t.Fatalf("'ttl' entry should appear inside the 'Browser · More' group; group=%d ttl=%d", idxGroup, idxTTL)
	}
	if !strings.Contains(out, "auto refresh") {
		t.Fatalf("help modal on browser screen should list 'auto refresh'; got %q", out)
	}
	idxAuto := strings.Index(out, "auto refresh")
	if idxAuto < idxGroup {
		t.Fatalf("'auto refresh' entry should appear inside the 'Browser · More' group; group=%d auto=%d", idxGroup, idxAuto)
	}
}

// TestHelpModalOmitsTtlAndAutoRefreshOnProfilesScreen verifies the new
// browser-only help defs do not leak into other screens. The Profiles
// help modal must mention neither ttl nor auto refresh.
func TestHelpModalOmitsTtlAndAutoRefreshOnProfilesScreen(t *testing.T) {
	m := New()
	m.Width = 100
	m.Height = 30
	m.Screen = ScreenProfiles

	out := m.renderHelpModal()
	if strings.Contains(out, "Browser · More") {
		t.Fatalf("help modal on Profiles screen must not include 'Browser · More' group; got %q", out)
	}
	if strings.Contains(out, "auto refresh") {
		t.Fatalf("help modal on Profiles screen must not mention 'auto refresh'; got %q", out)
	}
}
