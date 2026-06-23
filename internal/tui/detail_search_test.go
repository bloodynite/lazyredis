package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"github.com/bloodynite/lazyredis/internal/config"
	"github.com/bloodynite/lazyredis/internal/store"
)

func init() {
	// Force ANSI output so highlight tests can assert on escape codes.
	lipgloss.SetColorProfile(termenv.TrueColor)
}

func keyRune(r rune) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}
}

func typeQuery(m *Model, q string) *Model {
	for _, r := range q {
		next, _ := m.Update(keyRune(r))
		m = next.(*Model)
	}
	return m
}

func TestDetailSearchOpensOnSlashWhenDetailFocused(t *testing.T) {
	m := New()
	m.Width = 100
	m.Height = 24
	m.Screen = ScreenBrowser
	m.Client = &store.Client{}
	m.PanelFocus = panelDetail
	m.SelectedKey = "k"
	m.KeyDetail = &store.KeyDetail{Meta: store.KeyMeta{Type: "string", Key: "k"}, String: "hello world"}

	next, _ := m.Update(keyRune('/'))
	m = next.(*Model)
	if !m.DetailSearchFocus {
		t.Fatal("expected detail search focus when / pressed on detail panel")
	}
	if m.SearchFocus {
		t.Fatal("key filter must not open when detail panel focused")
	}
}

func TestDetailSearchKeepsKeyFilterOnKeysPanel(t *testing.T) {
	m := New()
	m.Width = 100
	m.Height = 24
	m.Screen = ScreenBrowser
	m.Client = &store.Client{}
	m.PanelFocus = panelKeys
	m.SelectedKey = "k"

	next, _ := m.Update(keyRune('/'))
	m = next.(*Model)
	if !m.SearchFocus {
		t.Fatal("key filter should open on / when keys panel focused")
	}
	if m.DetailSearchFocus {
		t.Fatal("detail search must not open when keys panel focused")
	}
}

func TestDetailSearchScrollsStringToMatch(t *testing.T) {
	m := New()
	m.Width = 80
	m.Height = 24
	m.Screen = ScreenBrowser
	m.Client = &store.Client{}
	m.PanelFocus = panelDetail
	m.SelectedKey = "k"
	m.KeyDetail = &store.KeyDetail{
		Meta: store.KeyMeta{Type: "string", Key: "k"},
		// Place a unique marker deep enough that the chunk containing it
		// is past the visible window midpoint.
		String: strings.Repeat("0", 500) + "findme" + strings.Repeat("0", 1500),
	}

	next, _ := m.Update(keyRune('/'))
	m = next.(*Model)
	m = typeQuery(m, "findme")
	if m.DetailScroll != 0 {
		t.Fatalf("typing alone must not advance scroll (live search disabled), got %d", m.DetailScroll)
	}

	// Enter applies the search.
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(*Model)
	if m.DetailScroll <= 0 {
		t.Fatalf("expected scroll to advance past first chunk on Enter, got %d", m.DetailScroll)
	}
	if m.Status != "1 match" {
		t.Fatalf("status = %q, want \"1 match\"", m.Status)
	}

	// Reopening the search then backspacing to empty must not regress
	// scroll: applyDetailSearch no-ops on empty query.
	scrollAfterApply := m.DetailScroll
	next, _ = m.Update(keyRune('/'))
	m = next.(*Model)
	m = typeQuery(m, "findme")
	for i := 0; i < len("findme"); i++ {
		next, _ = m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
		m = next.(*Model)
	}
	if m.DetailSearchInput.Value() != "" {
		t.Fatalf("expected empty query, got %q", m.DetailSearchInput.Value())
	}
	if m.DetailScroll != scrollAfterApply {
		t.Fatalf("scroll regressed on clear: was %d, now %d", scrollAfterApply, m.DetailScroll)
	}
}

func TestDetailSearchMovesCursorForHash(t *testing.T) {
	m := New()
	m.Width = 100
	m.Height = 24
	m.Screen = ScreenBrowser
	m.Client = &store.Client{}
	m.PanelFocus = panelDetail
	m.SelectedKey = "k"
	m.KeyDetail = &store.KeyDetail{
		Meta: store.KeyMeta{Type: "hash", Key: "k"},
		Hash: map[string]string{
			"alpha":  "1",
			"target": "found",
			"gamma":  "3",
		},
	}

	next, _ := m.Update(keyRune('/'))
	m = next.(*Model)
	m = typeQuery(m, "target")
	if m.DetailCursor != 0 {
		t.Fatalf("typing alone must not move cursor, got %d", m.DetailCursor)
	}
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(*Model)
	fields := hashFields(m.KeyDetail.Hash)
	targetIdx := -1
	for i, f := range fields {
		if f == "target" {
			targetIdx = i
			break
		}
	}
	if targetIdx < 0 {
		t.Fatal("setup: target field missing")
	}
	if m.DetailCursor != targetIdx {
		t.Fatalf("cursor = %d, want %d", m.DetailCursor, targetIdx)
	}
	if m.Status != "1 match" {
		t.Fatalf("status = %q, want \"1 match\"", m.Status)
	}
}

func TestDetailSearchMovesCursorForList(t *testing.T) {
	m := New()
	m.Width = 100
	m.Height = 24
	m.Screen = ScreenBrowser
	m.Client = &store.Client{}
	m.PanelFocus = panelDetail
	m.SelectedKey = "k"
	m.KeyDetail = &store.KeyDetail{
		Meta: store.KeyMeta{Type: "list", Key: "k"},
		List: []string{"alpha", "needle-here", "gamma"},
	}

	next, _ := m.Update(keyRune('/'))
	m = next.(*Model)
	m = typeQuery(m, "needle")
	if m.DetailCursor != 0 {
		t.Fatalf("typing alone must not move cursor, got %d", m.DetailCursor)
	}
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(*Model)
	if m.DetailCursor != 1 {
		t.Fatalf("cursor = %d, want 1", m.DetailCursor)
	}
	if m.Status != "1 match" {
		t.Fatalf("status = %q, want \"1 match\"", m.Status)
	}
}

// TestDetailSearchMultilineStringScrollsToMatch proves that the search
// math and the scroll math are both expressed in the same sanitized text
// the renderer uses. A string with many embedded newlines would otherwise
// get its raw byte offset indexed against the chunked sanitized render,
// landing the scroll on a chunk that has no "needle" at all.
func TestDetailSearchMultilineStringScrollsToMatch(t *testing.T) {
	m := New()
	m.Width = 100
	m.Height = 24
	m.Screen = ScreenBrowser
	m.Client = &store.Client{}
	m.Info = &store.ServerInfo{Version: "7.2", UsedMemory: "1M", TotalKeys: 1}
	m.PanelFocus = panelDetail
	m.SelectedKey = "k"
	m.KeyDetail = &store.KeyDetail{
		Meta:   store.KeyMeta{Type: "string", Key: "k"},
		String: strings.Repeat("\n", 1200) + "needle",
	}

	next, _ := m.Update(keyRune('/'))
	m = next.(*Model)
	m = typeQuery(m, "needle")
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(*Model)
	if m.Status != "1 match" {
		t.Fatalf("status = %q, want \"1 match\"", m.Status)
	}
	if m.DetailScroll <= 0 {
		t.Fatalf("scroll must advance into the body to reveal the match, got %d", m.DetailScroll)
	}

	// The match must be visible in the rendered body and the panel must
	// stay at its allocated height. This is the symptom from the blocker:
	// raw byte offset would land the scroll on a chunk full of newline
	// markers, never revealing "needle".
	_, rightW := m.browserPanelWidths()
	panelW := rightW - panelChromeCols
	height := m.browserContentHeight()
	panel := m.renderDetailPanel(panelW, height)
	lines := strings.Split(panel, "\n")
	if len(lines) != height {
		t.Fatalf("panel lines = %d, want %d (scroll=%d)", len(lines), height, m.DetailScroll)
	}
	if !strings.Contains(panel, "needle") {
		t.Fatalf("rendered body does not contain 'needle' after search; scroll=%d panel=%q", m.DetailScroll, panel)
	}
	// The chunk that holds the active match must carry the active highlight.
	activeMarker := activeSearchMatchStyle.Render("needle")
	if !strings.Contains(panel, activeMarker) {
		t.Fatalf("expected active highlight around 'needle' in rendered body; got %q", panel)
	}
}

func TestDetailSearchEscExits(t *testing.T) {
	m := New()
	m.Width = 100
	m.Height = 24
	m.Screen = ScreenBrowser
	m.Client = &store.Client{}
	m.PanelFocus = panelDetail
	m.SelectedKey = "k"
	m.KeyDetail = &store.KeyDetail{Meta: store.KeyMeta{Type: "string", Key: "k"}, String: "hello"}

	next, _ := m.Update(keyRune('/'))
	m = next.(*Model)
	if !m.DetailSearchFocus {
		t.Fatal("setup")
	}
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	m = next.(*Model)
	if m.DetailSearchFocus {
		t.Fatal("esc must exit detail search")
	}
}

func TestDetailSearchEnterExits(t *testing.T) {
	m := New()
	m.Width = 100
	m.Height = 24
	m.Screen = ScreenBrowser
	m.Client = &store.Client{}
	m.PanelFocus = panelDetail
	m.SelectedKey = "k"
	m.KeyDetail = &store.KeyDetail{Meta: store.KeyMeta{Type: "string", Key: "k"}, String: "hello"}

	next, _ := m.Update(keyRune('/'))
	m = next.(*Model)
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(*Model)
	if m.DetailSearchFocus {
		t.Fatal("enter must exit detail search")
	}
}

func TestDetailSearchKeepsHeightAndHeaderVisible(t *testing.T) {
	m := New()
	m.Width = 100
	m.Height = 24
	m.Screen = ScreenBrowser
	m.Client = &store.Client{}
	m.Info = &store.ServerInfo{Version: "7.2", UsedMemory: "1M", TotalKeys: 3}
	m.PanelFocus = panelDetail
	m.SelectedKey = "k"
	m.KeyDetail = &store.KeyDetail{
		Meta:   store.KeyMeta{Type: "string", Key: "k"},
		String: strings.Repeat("x", 5000),
	}
	m.DetailSearchFocus = true
	m.DetailSearchInput.Focus()
	m.DetailSearchInput.SetValue("xxx")

	out := m.View()
	lines := strings.Split(out, "\n")
	if len(lines) != m.Height {
		t.Fatalf("view lines = %d, want %d", len(lines), m.Height)
	}
	if !strings.Contains(lines[0], "Lazyredis") {
		t.Fatalf("header missing: %q", lines[0])
	}
}

func TestDetailSearchSurvivesKeyChange(t *testing.T) {
	m := New()
	m.Screen = ScreenBrowser
	m.Client = &store.Client{}
	m.PanelFocus = panelDetail
	m.SelectedKey = "old"
	m.KeyDetail = &store.KeyDetail{Meta: store.KeyMeta{Type: "string", Key: "old"}, String: "old"}
	m.DetailSearchFocus = true
	m.DetailSearchInput.SetValue("old-query")

	m.SelectedKey = "new"
	m.detailGen++
	next, _ := m.Update(keyDetailMsg{
		detail: &store.KeyDetail{Meta: store.KeyMeta{Type: "string", Key: "new"}, String: "new"},
		key:    "new",
		gen:    m.detailGen,
	})
	m = next.(*Model)
	if m.DetailSearchFocus {
		t.Fatal("detail search focus should stay off after key change")
	}
	if m.DetailSearchInput.Value() != "old-query" {
		t.Fatalf("query = %q, want preserved query", m.DetailSearchInput.Value())
	}
}

func TestDetailSearchAdversarialPanelToggle(t *testing.T) {
	// After closing detail search, pressing / on detail panel must reopen
	// detail search; pressing / on keys panel must reopen key filter.
	m := New()
	m.Width = 100
	m.Height = 24
	m.Screen = ScreenBrowser
	m.Client = &store.Client{}
	m.SelectedKey = "k"
	m.KeyDetail = &store.KeyDetail{Meta: store.KeyMeta{Type: "string", Key: "k"}, String: "hello"}

	// Tab to detail panel.
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = next.(*Model)
	if m.PanelFocus != panelDetail {
		t.Fatalf("panel focus = %v, want detail", m.PanelFocus)
	}

	// / opens detail search.
	next, _ = m.Update(keyRune('/'))
	m = next.(*Model)
	if !m.DetailSearchFocus {
		t.Fatal("expected detail search on /")
	}

	// Enter closes detail search.
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(*Model)
	if m.DetailSearchFocus {
		t.Fatal("enter must close detail search")
	}

	// Tab to keys panel.
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = next.(*Model)
	if m.PanelFocus != panelKeys {
		t.Fatalf("panel focus = %v, want keys", m.PanelFocus)
	}

	// / opens key filter.
	next, _ = m.Update(keyRune('/'))
	m = next.(*Model)
	if !m.SearchFocus {
		t.Fatal("/ on keys panel must open key filter")
	}
}

func TestDetailSearchAutoRefreshSkipped(t *testing.T) {
	sec := 5
	m := New()
	m.Screen = ScreenBrowser
	m.Client = &store.Client{}
	m.Config = &config.File{Settings: config.Settings{RefreshIntervalSec: &sec}}
	m.Loading = false
	m.DetailSearchFocus = true

	next, cmd := m.Update(autoRefreshMsg{})
	m = next.(*Model)
	if m.Loading {
		t.Fatal("auto refresh must not fire while detail search active")
	}
	if cmd == nil {
		t.Fatal("expected reschedule command")
	}
}

func TestDetailSearchNoMatchStatus(t *testing.T) {
	m := New()
	m.Width = 100
	m.Height = 24
	m.Screen = ScreenBrowser
	m.Client = &store.Client{}
	m.PanelFocus = panelDetail
	m.SelectedKey = "k"
	m.KeyDetail = &store.KeyDetail{
		Meta:   store.KeyMeta{Type: "string", Key: "k"},
		String: "hello world",
	}

	next, _ := m.Update(keyRune('/'))
	m = next.(*Model)
	m = typeQuery(m, "missing")
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(*Model)
	if m.Status != "no matches" {
		t.Fatalf("status = %q, want \"no matches\"", m.Status)
	}
	if m.DetailSearchFocus {
		t.Fatal("detail search should close after Enter")
	}
	if m.DetailScroll != 0 {
		t.Fatalf("scroll should not change on no match, got %d", m.DetailScroll)
	}
}

func TestDetailSearchMatchCountStatus(t *testing.T) {
	m := New()
	m.Width = 100
	m.Height = 24
	m.Screen = ScreenBrowser
	m.Client = &store.Client{}
	m.PanelFocus = panelDetail
	m.SelectedKey = "k"
	m.KeyDetail = &store.KeyDetail{
		Meta:   store.KeyMeta{Type: "string", Key: "k"},
		String: "foo foo foo",
	}

	next, _ := m.Update(keyRune('/'))
	m = next.(*Model)
	m = typeQuery(m, "foo")
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(*Model)
	if m.Status != "3 matches" {
		t.Fatalf("status = %q, want \"3 matches\"", m.Status)
	}
}

func TestDetailSearchHighlightVisibleInRender(t *testing.T) {
	m := New()
	m.Width = 100
	m.Height = 24
	m.Screen = ScreenBrowser
	m.Client = &store.Client{}
	m.Info = &store.ServerInfo{Version: "7.2", UsedMemory: "1M", TotalKeys: 1}
	m.PanelFocus = panelDetail
	m.SelectedKey = "k"
	m.KeyDetail = &store.KeyDetail{
		Meta:   store.KeyMeta{Type: "string", Key: "k"},
		String: "findme here",
	}

	// The single match in this detail is the active match, so the active
	// style (not the regular search-match style) is what wraps it. We assert
	// the active highlight is visible, not which exact style fires.
	activeMarker := activeSearchMatchStyle.Render("findme")

	next, _ := m.Update(keyRune('/'))
	m = next.(*Model)
	m = typeQuery(m, "findme")
	// While DetailSearchFocus is true, no highlight applied (active typing).
	typingBody := m.renderDetailPanel(50, 20)
	if strings.Contains(typingBody, activeMarker) {
		t.Fatal("highlight should not render while typing")
	}

	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(*Model)
	body := m.renderDetailPanel(50, 20)
	if !strings.Contains(body, activeMarker) {
		t.Fatalf("expected active search-match style codes in detail panel; got %q", body)
	}
	if !strings.Contains(body, "findme") {
		t.Fatal("matched substring should be visible in detail body")
	}
}

func TestDetailSearchHighlightHashField(t *testing.T) {
	m := New()
	m.Width = 100
	m.Height = 24
	m.Screen = ScreenBrowser
	m.Client = &store.Client{}
	m.Info = &store.ServerInfo{Version: "7.2", UsedMemory: "1M", TotalKeys: 1}
	m.PanelFocus = panelDetail
	m.SelectedKey = "k"
	m.KeyDetail = &store.KeyDetail{
		Meta: store.KeyMeta{Type: "hash", Key: "k"},
		Hash: map[string]string{
			"alpha":  "1",
			"target": "found",
			"gamma":  "3",
		},
	}

	next, _ := m.Update(keyRune('/'))
	m = next.(*Model)
	m = typeQuery(m, "target")
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(*Model)
	body := m.renderDetailPanel(60, 20)
	if !strings.Contains(body, "\x1b[") {
		t.Fatal("expected ANSI highlight codes in detail panel for hash match")
	}
}

func TestDetailSearchNextCyclesStringMatches(t *testing.T) {
	// String with three "foo" matches, each far enough apart to land in a
	// distinct chunk so scrolling actually moves on each n press.
	m := New()
	m.Width = 40
	m.Height = 24
	m.Screen = ScreenBrowser
	m.Client = &store.Client{}
	m.PanelFocus = panelDetail
	m.SelectedKey = "k"
	m.KeyDetail = &store.KeyDetail{
		Meta: store.KeyMeta{Type: "string", Key: "k"},
		String: "foo" + strings.Repeat("0", 200) +
			"foo" + strings.Repeat("0", 200) +
			"foo" + strings.Repeat("0", 50),
	}

	next, _ := m.Update(keyRune('/'))
	m = next.(*Model)
	m = typeQuery(m, "foo")
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(*Model)
	if m.Status != "3 matches" {
		t.Fatalf("status after apply = %q, want \"3 matches\"", m.Status)
	}
	if len(m.DetailSearchMatches) != 3 {
		t.Fatalf("matches stored = %d, want 3", len(m.DetailSearchMatches))
	}
	if m.DetailSearchCursor != 0 {
		t.Fatalf("cursor after apply = %d, want 0", m.DetailSearchCursor)
	}
	scrollFirst := m.DetailScroll

	// n moves forward.
	next, _ = m.Update(keyRune('n'))
	m = next.(*Model)
	if m.DetailSearchCursor != 1 {
		t.Fatalf("cursor after n = %d, want 1", m.DetailSearchCursor)
	}
	if m.Status != "match 2/3" {
		t.Fatalf("status after n = %q, want \"match 2/3\"", m.Status)
	}
	if m.DetailScroll <= scrollFirst {
		t.Fatalf("scroll did not advance on n: was %d, now %d", scrollFirst, m.DetailScroll)
	}
	scrollSecond := m.DetailScroll

	// n again to the third match.
	next, _ = m.Update(keyRune('n'))
	m = next.(*Model)
	if m.DetailSearchCursor != 2 {
		t.Fatalf("cursor after second n = %d, want 2", m.DetailSearchCursor)
	}
	if m.Status != "match 3/3" {
		t.Fatalf("status after second n = %q, want \"match 3/3\"", m.Status)
	}
	if m.DetailScroll <= scrollSecond {
		t.Fatalf("scroll did not advance on second n: was %d, now %d", scrollSecond, m.DetailScroll)
	}

	// n wraps back to the first match.
	next, _ = m.Update(keyRune('n'))
	m = next.(*Model)
	if m.DetailSearchCursor != 0 {
		t.Fatalf("cursor after wrap n = %d, want 0", m.DetailSearchCursor)
	}
	if m.Status != "match 1/3" {
		t.Fatalf("status after wrap = %q, want \"match 1/3\"", m.Status)
	}
	if m.DetailScroll != scrollFirst {
		t.Fatalf("scroll after wrap = %d, want %d", m.DetailScroll, scrollFirst)
	}

	// N from the first match wraps back to the last.
	next, _ = m.Update(keyRune('N'))
	m = next.(*Model)
	if m.DetailSearchCursor != 2 {
		t.Fatalf("cursor after N wrap = %d, want 2", m.DetailSearchCursor)
	}
	if m.Status != "match 3/3" {
		t.Fatalf("status after N wrap = %q, want \"match 3/3\"", m.Status)
	}

	// N from the third match goes back to the second.
	next, _ = m.Update(keyRune('N'))
	m = next.(*Model)
	if m.DetailSearchCursor != 1 {
		t.Fatalf("cursor after N = %d, want 1", m.DetailSearchCursor)
	}
	if m.Status != "match 2/3" {
		t.Fatalf("status after N = %q, want \"match 2/3\"", m.Status)
	}
}

func TestDetailSearchNextNoopOnKeysPanel(t *testing.T) {
	// On the keys panel, "n" must keep its existing "new key" behavior; it
	// must not trigger detail-search navigation.
	m := New()
	m.Width = 100
	m.Height = 24
	m.Screen = ScreenBrowser
	m.Client = &store.Client{}
	m.PanelFocus = panelKeys
	m.SelectedKey = "k"
	m.KeyDetail = &store.KeyDetail{Meta: store.KeyMeta{Type: "string", Key: "k"}, String: "x"}

	next, _ := m.Update(keyRune('/'))
	m = next.(*Model)
	m = typeQuery(m, "x")
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(*Model)
	// Apply happened while keys panel was focused (key filter path), so
	// DetailSearchMatches stays empty — detail search was never opened.
	if len(m.DetailSearchMatches) != 0 {
		t.Fatalf("expected no detail search matches, got %d", len(m.DetailSearchMatches))
	}
}

func TestDetailSearchNextNoopWithoutQuery(t *testing.T) {
	// On the detail panel with no applied query, "n" must NOT cycle or
	// otherwise interfere with the existing "new key" binding. We can't
	// fully assert the new-key side effect here (no client), but we can
	// confirm the navigation state is untouched and the screen transitioned
	// to the new-key form modal — the pre-existing action.
	m := New()
	m.Width = 100
	m.Height = 24
	m.Screen = ScreenBrowser
	m.Client = &store.Client{}
	m.PanelFocus = panelDetail
	m.SelectedKey = "k"
	m.KeyDetail = &store.KeyDetail{Meta: store.KeyMeta{Type: "string", Key: "k"}, String: "x"}

	next, _ := m.Update(keyRune('n'))
	m = next.(*Model)
	if m.Screen != ScreenKeyEdit {
		t.Fatalf("expected new-key form to open on detail-panel n without query, screen = %v", m.Screen)
	}
}

func TestDetailSearchKeyChangeResetsMatches(t *testing.T) {
	// After applying a search and navigating to match 2, switching keys must
	// re-apply the search on the new value and reset the cursor to 0 so
	// stale positions do not leak across key changes.
	m := New()
	m.Width = 80
	m.Height = 24
	m.Screen = ScreenBrowser
	m.Client = &store.Client{}
	m.PanelFocus = panelDetail
	m.SelectedKey = "old"
	m.KeyDetail = &store.KeyDetail{
		Meta:   store.KeyMeta{Type: "string", Key: "old"},
		String: "foo bar foo baz foo",
	}
	m.DetailSearchInput.SetValue("foo")
	m.applyDetailSearch(0, false)
	m.cycleDetailMatch(1)
	if m.DetailSearchCursor != 1 {
		t.Fatalf("setup: cursor = %d, want 1", m.DetailSearchCursor)
	}

	m.SelectedKey = "new"
	m.detailGen++
	next, _ := m.Update(keyDetailMsg{
		detail: &store.KeyDetail{
			Meta:   store.KeyMeta{Type: "string", Key: "new"},
			String: "foo x foo y foo",
		},
		key: "new",
		gen: m.detailGen,
	})
	m = next.(*Model)
	if m.DetailSearchCursor != 0 {
		t.Fatalf("cursor after key change = %d, want 0", m.DetailSearchCursor)
	}
	if len(m.DetailSearchMatches) != 3 {
		t.Fatalf("matches after key change = %d, want 3", len(m.DetailSearchMatches))
	}
}

func TestDetailSearchRefreshPreservesCursor(t *testing.T) {
	// A same-key refresh (auto-refresh of the cached value) must keep the
	// user on their current match index, not reset to the first match.
	m := New()
	m.Width = 80
	m.Height = 24
	m.Screen = ScreenBrowser
	m.Client = &store.Client{}
	m.PanelFocus = panelDetail
	m.SelectedKey = "k"
	m.KeyDetail = &store.KeyDetail{
		Meta:   store.KeyMeta{Type: "string", Key: "k"},
		String: "foo bar foo baz foo",
	}
	m.DetailSearchInput.SetValue("foo")
	m.applyDetailSearch(0, false)
	m.cycleDetailMatch(1)
	if m.DetailSearchCursor != 1 {
		t.Fatalf("setup: cursor = %d, want 1", m.DetailSearchCursor)
	}

	next, _ := m.Update(keyDetailMsg{
		detail: &store.KeyDetail{
			Meta:   store.KeyMeta{Type: "string", Key: "k"},
			String: "foo bar foo baz foo qux",
		},
		key: "k",
		gen: m.detailGen,
	})
	m = next.(*Model)
	if m.DetailSearchCursor != 1 {
		t.Fatalf("cursor after refresh = %d, want 1 (preserved)", m.DetailSearchCursor)
	}
	if len(m.DetailSearchMatches) != 3 {
		t.Fatalf("matches after refresh = %d, want 3", len(m.DetailSearchMatches))
	}
	if m.DetailSearchInput.Value() != "foo" {
		t.Fatalf("query after refresh = %q, want \"foo\" (preserved)", m.DetailSearchInput.Value())
	}
}

func TestDetailSearchRefreshClampsCursorWhenFewerMatches(t *testing.T) {
	// If the refresh shrinks the match count below the active cursor, the
	// cursor must clamp to the last available match instead of going out
	// of range or jumping to 0.
	m := New()
	m.Width = 80
	m.Height = 24
	m.Screen = ScreenBrowser
	m.Client = &store.Client{}
	m.PanelFocus = panelDetail
	m.SelectedKey = "k"
	m.KeyDetail = &store.KeyDetail{
		Meta:   store.KeyMeta{Type: "string", Key: "k"},
		String: "foo x foo y foo z foo",
	}
	m.DetailSearchInput.SetValue("foo")
	m.applyDetailSearch(0, false)
	m.cycleDetailMatch(1)
	m.cycleDetailMatch(1)
	if m.DetailSearchCursor != 2 {
		t.Fatalf("setup: cursor = %d, want 2", m.DetailSearchCursor)
	}

	// Refresh reduces matches from 4 to 2; active cursor was 2, must clamp to 1.
	next, _ := m.Update(keyDetailMsg{
		detail: &store.KeyDetail{
			Meta:   store.KeyMeta{Type: "string", Key: "k"},
			String: "foo x foo y",
		},
		key: "k",
		gen: m.detailGen,
	})
	m = next.(*Model)
	if len(m.DetailSearchMatches) != 2 {
		t.Fatalf("matches after refresh = %d, want 2", len(m.DetailSearchMatches))
	}
	if m.DetailSearchCursor != 1 {
		t.Fatalf("cursor after refresh = %d, want 1 (clamped to last match)", m.DetailSearchCursor)
	}
}

func TestDetailSearchRefreshClearsCursorWhenNoMatches(t *testing.T) {
	// If the new value no longer contains the query, the active cursor
	// must reset to -1 and matches must be empty, while the query itself
	// stays so subsequent refreshes can re-match.
	m := New()
	m.Width = 80
	m.Height = 24
	m.Screen = ScreenBrowser
	m.Client = &store.Client{}
	m.PanelFocus = panelDetail
	m.SelectedKey = "k"
	m.KeyDetail = &store.KeyDetail{
		Meta:   store.KeyMeta{Type: "string", Key: "k"},
		String: "foo bar foo",
	}
	m.DetailSearchInput.SetValue("foo")
	m.applyDetailSearch(0, false)
	m.cycleDetailMatch(1)
	if m.DetailSearchCursor != 1 {
		t.Fatalf("setup: cursor = %d, want 1", m.DetailSearchCursor)
	}

	next, _ := m.Update(keyDetailMsg{
		detail: &store.KeyDetail{
			Meta:   store.KeyMeta{Type: "string", Key: "k"},
			String: "plain value",
		},
		key: "k",
		gen: m.detailGen,
	})
	m = next.(*Model)
	if len(m.DetailSearchMatches) != 0 {
		t.Fatalf("matches after refresh = %d, want 0", len(m.DetailSearchMatches))
	}
	if m.DetailSearchCursor != -1 {
		t.Fatalf("cursor after refresh = %d, want -1 (no matches)", m.DetailSearchCursor)
	}
	if m.DetailSearchInput.Value() != "foo" {
		t.Fatalf("query after refresh = %q, want \"foo\" (preserved)", m.DetailSearchInput.Value())
	}
}

func TestDetailSearchRefreshPreservesCursorForHash(t *testing.T) {
	// Same fix must apply to composite types, where the match index is the
	// row index in the sorted fields list.
	m := New()
	m.Width = 80
	m.Height = 24
	m.Screen = ScreenBrowser
	m.Client = &store.Client{}
	m.PanelFocus = panelDetail
	m.SelectedKey = "k"
	m.KeyDetail = &store.KeyDetail{
		Meta: store.KeyMeta{Type: "hash", Key: "k"},
		Hash: map[string]string{
			"alpha":  "1",
			"target": "found",
			"beta":   "found",
			"gamma":  "3",
		},
	}
	m.DetailSearchInput.SetValue("found")
	m.applyDetailSearch(0, false)
	m.cycleDetailMatch(1)
	if m.DetailSearchCursor != 1 {
		t.Fatalf("setup: cursor = %d, want 1", m.DetailSearchCursor)
	}

	// Same key refresh with a different field still containing "found".
	next, _ := m.Update(keyDetailMsg{
		detail: &store.KeyDetail{
			Meta: store.KeyMeta{Type: "hash", Key: "k"},
			Hash: map[string]string{
				"alpha":  "1",
				"target": "found",
				"beta":   "found",
				"gamma":  "3",
				"delta":  "found",
			},
		},
		key: "k",
		gen: m.detailGen,
	})
	m = next.(*Model)
	if len(m.DetailSearchMatches) != 3 {
		t.Fatalf("matches after refresh = %d, want 3", len(m.DetailSearchMatches))
	}
	if m.DetailSearchCursor != 1 {
		t.Fatalf("cursor after refresh = %d, want 1 (preserved)", m.DetailSearchCursor)
	}
}

func TestDetailSearchClearQueryClearsMatches(t *testing.T) {
	// Reopening the search and pressing Enter with an empty query must
	// clear stored matches so n becomes a no-op again.
	m := New()
	m.Width = 80
	m.Height = 24
	m.Screen = ScreenBrowser
	m.Client = &store.Client{}
	m.PanelFocus = panelDetail
	m.SelectedKey = "k"
	m.KeyDetail = &store.KeyDetail{
		Meta:   store.KeyMeta{Type: "string", Key: "k"},
		String: "foo bar foo",
	}

	next, _ := m.Update(keyRune('/'))
	m = next.(*Model)
	m = typeQuery(m, "foo")
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(*Model)
	if len(m.DetailSearchMatches) == 0 {
		t.Fatal("setup: matches should be stored after apply")
	}

	next, _ = m.Update(keyRune('/'))
	m = next.(*Model)
	for i := 0; i < len("foo"); i++ {
		next, _ = m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
		m = next.(*Model)
	}
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(*Model)
	if len(m.DetailSearchMatches) != 0 {
		t.Fatalf("expected matches cleared, got %d", len(m.DetailSearchMatches))
	}
	if m.DetailSearchCursor != -1 {
		t.Fatalf("expected cursor reset, got %d", m.DetailSearchCursor)
	}
}

func TestDetailSearchActiveMatchStyleDistinctString(t *testing.T) {
	// String with three "foo" matches spaced far enough apart to land in
	// distinct chunks at Width=40. After applying, the first chunk holds
	// the active match; the second chunk holds a non-active match. Both
	// styles must appear in the rendered body.
	m := New()
	m.Width = 40
	m.Height = 24
	m.Screen = ScreenBrowser
	m.Client = &store.Client{}
	m.Info = &store.ServerInfo{Version: "7.2", UsedMemory: "1M", TotalKeys: 1}
	m.PanelFocus = panelDetail
	m.SelectedKey = "k"
	m.KeyDetail = &store.KeyDetail{
		Meta: store.KeyMeta{Type: "string", Key: "k"},
		String: "foo" + strings.Repeat("0", 100) +
			"foo" + strings.Repeat("0", 100) +
			"foo",
	}

	activeMarker := activeSearchMatchStyle.Render("foo")
	regularMarker := searchMatchStyle.Render("foo")

	next, _ := m.Update(keyRune('/'))
	m = next.(*Model)
	m = typeQuery(m, "foo")
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(*Model)
	body := m.renderDetailPanel(40, 24)
	if !strings.Contains(body, activeMarker) {
		t.Fatalf("expected activeSearchMatchStyle codes in body after apply; got %q", body)
	}
	if !strings.Contains(body, regularMarker) {
		t.Fatalf("expected searchMatchStyle codes for non-active matches; got %q", body)
	}

	// n moves to match 2; active style must still be present (now in a
	// different chunk) and regular style must still cover other matches.
	next, _ = m.Update(keyRune('n'))
	m = next.(*Model)
	if m.DetailSearchCursor != 1 {
		t.Fatalf("cursor after n = %d, want 1", m.DetailSearchCursor)
	}
	body = m.renderDetailPanel(40, 24)
	if !strings.Contains(body, activeMarker) {
		t.Fatalf("expected activeSearchMatchStyle after n; got %q", body)
	}
	if !strings.Contains(body, regularMarker) {
		t.Fatalf("expected searchMatchStyle after n; got %q", body)
	}
}

func TestDetailSearchActiveMatchStyleDistinctHash(t *testing.T) {
	// Hash with three matching fields. The cursor lands on the active match
	// (sorted fields: alpha, beta, gamma). Only one row should carry the
	// activeSearchMatchStyle codes; the others must use searchMatchStyle.
	m := New()
	m.Width = 100
	m.Height = 24
	m.Screen = ScreenBrowser
	m.Client = &store.Client{}
	m.Info = &store.ServerInfo{Version: "7.2", UsedMemory: "1M", TotalKeys: 1}
	m.PanelFocus = panelDetail
	m.SelectedKey = "k"
	m.KeyDetail = &store.KeyDetail{
		Meta: store.KeyMeta{Type: "hash", Key: "k"},
		Hash: map[string]string{
			"alpha": "found",
			"beta":  "found",
			"gamma": "found",
		},
	}

	activeMarker := activeSearchMatchStyle.Render("found")
	regularMarker := searchMatchStyle.Render("found")

	next, _ := m.Update(keyRune('/'))
	m = next.(*Model)
	m = typeQuery(m, "found")
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(*Model)
	body := m.renderDetailPanel(60, 20)

	activeCount := strings.Count(body, activeMarker)
	regularCount := strings.Count(body, regularMarker)
	if activeCount != 1 {
		t.Fatalf("expected exactly 1 active highlight in body, got %d; body=%q", activeCount, body)
	}
	if regularCount < 2 {
		t.Fatalf("expected at least 2 regular highlights, got %d; body=%q", regularCount, body)
	}
}

func TestDetailSearchActiveMatchStyleClearedOnKeyChange(t *testing.T) {
	// After switching to a different key (no matches), the active style must
	// not bleed into the new detail render.
	m := New()
	m.Width = 100
	m.Height = 24
	m.Screen = ScreenBrowser
	m.Client = &store.Client{}
	m.Info = &store.ServerInfo{Version: "7.2", UsedMemory: "1M", TotalKeys: 1}
	m.PanelFocus = panelDetail
	m.SelectedKey = "k"
	m.KeyDetail = &store.KeyDetail{Meta: store.KeyMeta{Type: "string", Key: "k"}, String: "foo bar foo"}

	next, _ := m.Update(keyRune('/'))
	m = next.(*Model)
	m = typeQuery(m, "foo")
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(*Model)

	activeMarker := activeSearchMatchStyle.Render("foo")
	if !strings.Contains(m.renderDetailPanel(50, 20), activeMarker) {
		t.Fatal("setup: active style should be present after apply")
	}

	// Switch to a key with no matches: DetailSearchInput keeps its value, so
	// the new detail panel would still attempt to render highlights. With no
	// matches, activeSearchMatchStyle must NOT appear.
	m.SelectedKey = "new"
	m.detailGen++
	next, _ = m.Update(keyDetailMsg{
		detail: &store.KeyDetail{Meta: store.KeyMeta{Type: "string", Key: "new"}, String: "plain"},
		key:    "new",
		gen:    m.detailGen,
	})
	m = next.(*Model)
	if len(m.DetailSearchMatches) != 0 {
		t.Fatalf("expected no matches on new key, got %d", len(m.DetailSearchMatches))
	}
	body := m.renderDetailPanel(50, 20)
	if strings.Contains(body, activeMarker) {
		t.Fatalf("active style must not render when there are no matches; body=%q", body)
	}
}
