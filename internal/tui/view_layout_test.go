package tui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/bloodynite/lazyredis/internal/config"
	"github.com/bloodynite/lazyredis/internal/store"
	"github.com/charmbracelet/lipgloss"
)

func TestViewFitsTerminalHeight(t *testing.T) {
	m := New()
	m.Width = 100
	m.Height = 24
	m.Screen = ScreenBrowser
	m.Client = &store.Client{}
	m.Info = &store.ServerInfo{
		Version: "7.2", UsedMemory: "1M", TotalKeys: 10,
		Connected: "1", OpsPerSec: "0", Role: "master", Uptime: "100",
	}
	m.Config = &config.File{}
	m.Keys = []string{"demo:key"}
	m.KeyDetail = &store.KeyDetail{
		Meta: store.KeyMeta{Type: "string", Key: "demo:key"},
		String: "hello",
	}

	out := m.View()
	lines := strings.Split(out, "\n")
	if len(lines) != m.Height {
		t.Fatalf("view lines = %d, want %d", len(lines), m.Height)
	}
}

func TestViewShowsKeysPanelMeta(t *testing.T) {
	m := New()
	m.Width = 100
	m.Height = 24
	m.Screen = ScreenBrowser
	m.Client = &store.Client{}
	m.Config = &config.File{}
	m.Info = &store.ServerInfo{TotalKeys: 210}
	m.Keys = make([]string, 101)
	for i := range m.Keys {
		m.Keys[i] = fmt.Sprintf("key:%03d", i)
	}
	m.ScanCursor = 1

	out := m.View()
	if !strings.Contains(out, "101/210") {
		t.Fatalf("view should show keys pagination meta")
	}
	if !strings.Contains(out, " · g") {
		t.Fatalf("view should show pagination hint")
	}
}

func TestViewDoesNotOverflowWithLongString(t *testing.T) {
	m := New()
	m.Width = 100
	m.Height = 24
	m.Screen = ScreenBrowser
	m.Client = &store.Client{}
	m.Config = &config.File{}
	m.Keys = []string{"demo:key"}
	m.KeyDetail = &store.KeyDetail{
		Meta:   store.KeyMeta{Type: "string", Key: "demo:key"},
		String: strings.Repeat("x", 5000),
	}

	out := m.View()
	lines := strings.Split(out, "\n")
	if len(lines) != m.Height {
		t.Fatalf("view lines = %d, want %d", len(lines), m.Height)
	}
}

func TestViewKeepsHeaderVisibleWithLongValueAndSelection(t *testing.T) {
	m := New()
	m.Width = 100
	m.Height = 24
	m.Screen = ScreenBrowser
	m.Client = &store.Client{}
	m.Info = &store.ServerInfo{Version: "7.2", UsedMemory: "1M", TotalKeys: 3}
	m.Keys = []string{"key:1", "key:2", "key:3"}
	m.KeyCursor = 2
	m.SelectedKey = "key:3"
	m.PanelFocus = panelDetail
	m.KeyDetail = &store.KeyDetail{
		Meta:   store.KeyMeta{Type: "string", Key: "key:3"},
		String: strings.Repeat("x", 5000),
	}

	out := m.View()
	lines := strings.Split(out, "\n")
	if len(lines) != m.Height {
		t.Fatalf("view lines = %d, want %d", len(lines), m.Height)
	}
	if !strings.Contains(lines[0], "Lazyredis") {
		t.Fatalf("header missing: %q", lines[0])
	}
}

func TestStringDetailScrolls(t *testing.T) {
	m := New()
	m.Width = 80
	m.Height = 24
	m.Screen = ScreenBrowser
	m.Client = &store.Client{}
	m.PanelFocus = panelDetail
	m.KeyDetail = &store.KeyDetail{
		Meta:   store.KeyMeta{Type: "string", Key: "demo:key"},
		String: strings.Repeat("0123456789", 200),
	}

	_, rightW := m.browserPanelWidths()
	panelW := rightW - panelChromeCols
	visible := max(1, m.browserContentHeight()-4)
	if limit := stringDetailScrollLimit(m.KeyDetail.String, panelW, visible); limit <= 0 {
		t.Fatalf("expected scroll limit > 0, got %d", limit)
	}

	first := strings.Join(m.renderDetailBody(m.KeyDetail, panelW, visible, ""), "\n")
	m.detailMove(1)
	if m.DetailScroll == 0 {
		t.Fatal("expected detail scroll to move")
	}
	second := strings.Join(m.renderDetailBody(m.KeyDetail, panelW, visible, ""), "\n")
	if first == second {
		t.Fatal("expected scrolled value rendering to change")
	}
}

func TestViewBrowserPanelsFitWidth(t *testing.T) {
	m := New()
	m.Width = 100
	m.Height = 24
	m.Screen = ScreenBrowser
	m.Client = &store.Client{}
	m.Info = &store.ServerInfo{Version: "7.2", UsedMemory: "1M", TotalKeys: 10}
	m.Config = &config.File{}
	m.Keys = []string{"demo:key"}
	m.KeyDetail = &store.KeyDetail{
		Meta:   store.KeyMeta{Type: "string", Key: "demo:key"},
		String: "hello world",
	}

	out := m.View()
	lines := strings.Split(out, "\n")
	panelEnd := gridInfoRows + m.panelAreaLines()
	for i := gridInfoRows; i < panelEnd && i < len(lines); i++ {
		if w := lipgloss.Width(lines[i]); w != m.Width {
			t.Fatalf("panel line %d width=%d want %d", i, w, m.Width)
		}
	}
}

func TestViewShowsInfoRowsOnTop(t *testing.T) {
	m := New()
	m.Width = 100
	m.Height = 24
	m.Screen = ScreenBrowser
	m.Client = &store.Client{}
	m.Info = &store.ServerInfo{Version: "7.2", UsedMemory: "1M", TotalKeys: 10}
	m.Config = &config.File{}
	m.Keys = []string{"a"}

	out := m.View()
	lines := strings.Split(out, "\n")
	if len(lines) < 2 {
		t.Fatal("expected at least 2 lines")
	}
	if strings.TrimSpace(lines[0]) == "" {
		t.Fatalf("info line 1 empty: %q", lines[0])
	}
	if !strings.Contains(lines[1], "keys") {
		t.Fatalf("info line 2 = %q, want server stats", lines[1])
	}
}

// TestDetailPanelHeightFitsWithMultilineString proves that a string value
// containing embedded newlines never pushes the detail panel beyond its
// allocated height.
func TestDetailPanelHeightFitsWithMultilineString(t *testing.T) {
	m := New()
	m.Width = 100
	m.Height = 24
	m.Screen = ScreenBrowser
	m.Client = &store.Client{}
	m.Config = &config.File{}
	m.Keys = []string{"demo:key"}
	// A value rich in newlines: any chunk that kept the real "\n" would
	// render as multiple physical terminal rows, overflowing the panel.
	m.KeyDetail = &store.KeyDetail{
		Meta:   store.KeyMeta{Type: "string", Key: "demo:key"},
		String: "alpha\nbeta\ngamma\n\ndelta\nepsilon\r\nzeta",
	}

	out := m.View()
	lines := strings.Split(out, "\n")
	if len(lines) != m.Height {
		t.Fatalf("view lines = %d, want %d", len(lines), m.Height)
	}
	// Header must stay at the top even when the value contains newlines.
	if !strings.Contains(lines[0], "Lazyredis") {
		t.Fatalf("header missing in line 0: %q", lines[0])
	}
	// The visible marker should appear in the rendered body so the user
	// still sees the value had embedded newlines.
	if !strings.Contains(out, detailNewlineMarker) {
		t.Fatalf("expected newline marker %q in view output", detailNewlineMarker)
	}
}

// TestDetailPanelHeightFitsWithMultilineHash proves the same property for
// a hash whose field values contain embedded newlines.
func TestDetailPanelHeightFitsWithMultilineHash(t *testing.T) {
	m := New()
	m.Width = 100
	m.Height = 24
	m.Screen = ScreenBrowser
	m.Client = &store.Client{}
	m.Config = &config.File{}
	m.Keys = []string{"demo:hash"}
	m.KeyDetail = &store.KeyDetail{
		Meta: store.KeyMeta{Type: "hash", Key: "demo:hash"},
		Hash: map[string]string{
			"f1": "line1\nline2\nline3",
			"f2": "alpha\nbeta\ngamma\ndelta",
			"f3": "single\nline",
		},
	}

	out := m.View()
	lines := strings.Split(out, "\n")
	if len(lines) != m.Height {
		t.Fatalf("view lines = %d, want %d", len(lines), m.Height)
	}
	if !strings.Contains(out, detailNewlineMarker) {
		t.Fatalf("expected newline marker %q in view output", detailNewlineMarker)
	}
}

// TestDetailPanelHeightFitsWithMultilineList proves the same property for
// a list whose items contain embedded newlines.
func TestDetailPanelHeightFitsWithMultilineList(t *testing.T) {
	m := New()
	m.Width = 100
	m.Height = 24
	m.Screen = ScreenBrowser
	m.Client = &store.Client{}
	m.Config = &config.File{}
	m.Keys = []string{"demo:list"}
	m.KeyDetail = &store.KeyDetail{
		Meta:  store.KeyMeta{Type: "list", Key: "demo:list"},
		List:  []string{"a\nb\nc", "d\ne", "f\ng\nh\ni\nj"},
	}

	out := m.View()
	lines := strings.Split(out, "\n")
	if len(lines) != m.Height {
		t.Fatalf("view lines = %d, want %d", len(lines), m.Height)
	}
	if !strings.Contains(out, detailNewlineMarker) {
		t.Fatalf("expected newline marker %q in view output", detailNewlineMarker)
	}
}

// TestDetailPanelHeightFitsWithMultilineSet proves the same property for
// a set whose members contain embedded newlines.
func TestDetailPanelHeightFitsWithMultilineSet(t *testing.T) {
	m := New()
	m.Width = 100
	m.Height = 24
	m.Screen = ScreenBrowser
	m.Client = &store.Client{}
	m.Config = &config.File{}
	m.Keys = []string{"demo:set"}
	m.KeyDetail = &store.KeyDetail{
		Meta: store.KeyMeta{Type: "set", Key: "demo:set"},
		Set:  []string{"alpha\nbeta", "gamma\ndelta\nepsilon"},
	}

	out := m.View()
	lines := strings.Split(out, "\n")
	if len(lines) != m.Height {
		t.Fatalf("view lines = %d, want %d", len(lines), m.Height)
	}
	if !strings.Contains(out, detailNewlineMarker) {
		t.Fatalf("expected newline marker %q in view output", detailNewlineMarker)
	}
}

// TestRenderDetailPanelExactHeightMultiline proves the panel body itself
// (not just the full View) stays at its allocated height when a single
// composite row contains an embedded newline.
func TestRenderDetailPanelExactHeightMultiline(t *testing.T) {
	m := New()
	m.Width = 100
	m.Height = 24
	m.Screen = ScreenBrowser
	m.Client = &store.Client{}
	m.Keys = []string{"k"}
	m.SelectedKey = "k"
	_, rightW := m.browserPanelWidths()
	panelW := rightW - panelChromeCols
	height := m.browserContentHeight()

	m.KeyDetail = &store.KeyDetail{
		Meta: store.KeyMeta{Type: "list", Key: "k"},
		List: []string{"one\ntwo\nthree\nfour", "five", "six\nseven"},
	}

	panel := m.renderDetailPanel(panelW, height)
	lines := strings.Split(panel, "\n")
	if len(lines) != height {
		t.Fatalf("detail panel lines = %d, want %d", len(lines), height)
	}
	// The marker must appear in the rendered body so the user can see the
	// value had embedded newlines.
	if !strings.Contains(panel, detailNewlineMarker) {
		t.Fatalf("expected newline marker %q in detail panel", detailNewlineMarker)
	}
}

// TestSanitizeDetailRowFastPath is a small unit test on the helper itself
// so future regressions on the marker logic get caught fast.
func TestSanitizeDetailRowFastPath(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"empty", "", ""},
		{"no newline", "hello world", "hello world"},
		{"lf", "a\nb", "a" + detailNewlineMarker + "b"},
		{"crlf", "a\r\nb", "a" + detailNewlineMarker + "b"},
		{"bare cr", "a\rb", "a" + detailNewlineMarker + "b"},
		{"multiple", "a\nb\r\nc\rd", "a" + detailNewlineMarker + "b" + detailNewlineMarker + "c" + detailNewlineMarker + "d"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := sanitizeDetailRow(tc.in); got != tc.want {
				t.Fatalf("sanitizeDetailRow(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
