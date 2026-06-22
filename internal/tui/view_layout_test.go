package tui

import (
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

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
		Meta:   store.KeyMeta{Type: "string", Key: "demo:key"},
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

func TestDetailPanelHeightFitsWithMultilineString(t *testing.T) {
	m := New()
	m.Width = 100
	m.Height = 24
	m.Screen = ScreenBrowser
	m.Client = &store.Client{}
	m.Config = &config.File{}
	m.Keys = []string{"demo:key"}
	m.KeyDetail = &store.KeyDetail{
		Meta:   store.KeyMeta{Type: "string", Key: "demo:key"},
		String: "alpha\nbeta\ngamma\n\ndelta\nepsilon\r\nzeta",
	}

	out := m.View()
	lines := strings.Split(out, "\n")
	if len(lines) != m.Height {
		t.Fatalf("view lines = %d, want %d", len(lines), m.Height)
	}
	if !strings.Contains(lines[0], "Lazyredis") {
		t.Fatalf("header missing in line 0: %q", lines[0])
	}
	if !strings.Contains(out, detailNewlineMarker) {
		t.Fatalf("expected newline marker %q in view output", detailNewlineMarker)
	}
}

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

func TestDetailPanelHeightFitsWithMultilineList(t *testing.T) {
	m := New()
	m.Width = 100
	m.Height = 24
	m.Screen = ScreenBrowser
	m.Client = &store.Client{}
	m.Config = &config.File{}
	m.Keys = []string{"demo:list"}
	m.KeyDetail = &store.KeyDetail{
		Meta: store.KeyMeta{Type: "list", Key: "demo:list"},
		List: []string{"a\nb\nc", "d\ne", "f\ng\nh\ni\nj"},
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
	if !strings.Contains(panel, detailNewlineMarker) {
		t.Fatalf("expected newline marker %q in detail panel", detailNewlineMarker)
	}
}

func TestChunkStringStripsTabsBeforeChunking(t *testing.T) {
	cases := []struct {
		name string
		in   string
		size int
	}{
		{"tabs and ascii", "a\tb\tc\td\te", 4},
		{"only tabs", strings.Repeat("\t", 30), 8},
		{"tabs with wide runes", "你\t好\t世\t界", 6},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			chunks := chunkString(tc.in, tc.size)
			joined := strings.Join(chunks, "")
			wantJoined := sanitizeDetailRow(tc.in)
			if joined != wantJoined {
				t.Fatalf("reassembled chunks != sanitized input: got %q want %q", joined, wantJoined)
			}
			if strings.ContainsRune(joined, '\t') {
				t.Fatalf("chunks leaked raw tab from input %q: %q", tc.in, joined)
			}
			for i, c := range chunks {
				if strings.ContainsRune(c, '\t') {
					t.Fatalf("chunk %d leaked raw tab from input %q: %q", i, tc.in, c)
				}
				if w := lipgloss.Width(c); w > tc.size {
					t.Fatalf("chunk %d width=%d exceeds budget %d (chunk=%q)", i, w, tc.size, c)
				}
			}
		})
	}
}

func TestChunkStringNeverSplitsRune(t *testing.T) {
	cases := []struct {
		name string
		in   string
		size int
	}{
		{"emoji short", "👋🌍🚀", 1},
		{"cjk short", "你好世界", 1},
		{"cjk split", "你好世界", 3},
		{"mixed short", "a你b好c世d界", 2},
		{"mixed odd", "a你b好c世d界", 3},
		{"ascii only", strings.Repeat("x", 100), 8},
		{"cjk heavy", strings.Repeat("你", 50), 8},
		{"empty", "", 8},
		{"fits in one", "hello", 10},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			chunks := chunkString(tc.in, tc.size)
			for i, c := range chunks {
				if !utf8.ValidString(c) {
					t.Fatalf("chunk %d (%q) is not valid UTF-8 (input=%q size=%d)", i, c, tc.in, tc.size)
				}
			}
			if joined := strings.Join(chunks, ""); joined != tc.in {
				t.Fatalf("reassembled chunks != input: got %q want %q", joined, tc.in)
			}
		})
	}
}

func TestChunkStringRespectsDisplayWidth(t *testing.T) {
	cases := []struct {
		name string
		in   string
		size int
	}{
		{"cjk tight", "你好世界你好世界", 4},
		{"emoji tight", "👋🌍🚀👋🌍🚀", 4},
		{"mixed", "a你b好c世d界e", 3},
		{"wide boundary", strings.Repeat("你", 30), 5},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			chunks := chunkString(tc.in, tc.size)
			for i, c := range chunks {
				if w := lipgloss.Width(c); w > tc.size {
					t.Fatalf("chunk %d width=%d, exceeds budget %d (chunk=%q)", i, w, tc.size, c)
				}
			}
			if joined := strings.Join(chunks, ""); joined != tc.in {
				t.Fatalf("reassembled chunks != input: got %q want %q", joined, tc.in)
			}
		})
	}
}

func TestChunkStringRuneWiderThanBudget(t *testing.T) {
	in := "你"
	chunks := chunkString(in, 1)
	if len(chunks) != 1 {
		t.Fatalf("chunks = %d, want 1", len(chunks))
	}
	if chunks[0] != in {
		t.Fatalf("chunk = %q, want %q", chunks[0], in)
	}
}

func TestChunkPositionForByteOffset(t *testing.T) {
	in := strings.Repeat("你", 100) + "needle" + strings.Repeat("好", 100)
	size := 8
	idx, off := chunkPositionForByteOffset(in, size, -1)
	if idx != -1 {
		t.Fatalf("negative offset: idx=%d, want -1", idx)
	}
	needlePos := strings.Index(in, "needle")
	idx, off = chunkPositionForByteOffset(in, size, needlePos)
	if idx < 0 {
		t.Fatalf("needle not found in any chunk: idx=%d", idx)
	}
	chunks := chunkString(in, size)
	if idx >= len(chunks) {
		t.Fatalf("idx=%d out of range (chunks=%d)", idx, len(chunks))
	}
	if !strings.Contains(chunks[idx], "needle") {
		t.Fatalf("chunk %d (%q) does not contain 'needle'", idx, chunks[idx])
	}
	if chunks[idx][off:off+len("needle")] != "needle" {
		t.Fatalf("offset %d in chunk %d does not point to 'needle' (chunk=%q)", off, idx, chunks[idx])
	}
}

func TestLargeUnicodeJsonStringFitsPanel(t *testing.T) {
	m := New()
	m.Width = 100
	m.Height = 24
	m.Screen = ScreenBrowser
	m.Client = &store.Client{}
	m.Info = &store.ServerInfo{Version: "7.2", UsedMemory: "1M", TotalKeys: 1}
	m.Keys = []string{"demo:json"}
	m.SelectedKey = "demo:json"
	m.PanelFocus = panelDetail
	jsonVal := `{"name":"José García","city":"São Paulo","note":"` +
		strings.Repeat("héllo世界🌍 ", 400) + `"}`
	m.KeyDetail = &store.KeyDetail{
		Meta:   store.KeyMeta{Type: "string", Key: "demo:json"},
		String: jsonVal,
	}

	out := m.View()
	if !utf8.ValidString(out) {
		t.Fatalf("rendered view is not valid UTF-8 — byte-based chunking split a rune")
	}
	lines := strings.Split(out, "\n")
	if len(lines) != m.Height {
		t.Fatalf("view lines = %d, want %d", len(lines), m.Height)
	}
	_, rightW := m.browserPanelWidths()
	panelInner := rightW - panelChromeCols
	panelStart := gridInfoRows
	panelEnd := panelStart + m.panelAreaLines()
	for i := panelStart; i < panelEnd && i < len(lines); i++ {
		if w := lipgloss.Width(lines[i]); w != m.Width {
			t.Fatalf("panel line %d width=%d, want %d (line=%q)", i, w, m.Width, lines[i])
		}
	}
	body := m.renderDetailPanel(panelInner, m.browserContentHeight())
	if !utf8.ValidString(body) {
		t.Fatalf("renderDetailPanel output is not valid UTF-8 — byte-based chunking split a rune")
	}
	for i, l := range strings.Split(body, "\n") {
		if w := lipgloss.Width(l); w > panelInner {
			t.Fatalf("detail body line %d width=%d exceeds panel width %d (line=%q)", i, w, panelInner, l)
		}
	}
}

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
		{"single tab", "a\tb", "a    b"},
		{"two tabs", "\t\t", "        "},
		{"tab + newline", "a\t\nb", "a    " + detailNewlineMarker + "b"},
		{"tab only", "\t", "    "},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := sanitizeDetailRow(tc.in)
			if got != tc.want {
				t.Fatalf("sanitizeDetailRow(%q) = %q, want %q", tc.in, got, tc.want)
			}
			if strings.ContainsRune(got, '\t') {
				t.Fatalf("sanitizeDetailRow(%q) leaked raw tab: %q", tc.in, got)
			}
		})
	}
}

func TestScrollJSONWithTabsKeepsBorders(t *testing.T) {
	jsonVal := `{"data":[`
	for i := 0; i < 50; i++ {
		if i > 0 {
			jsonVal += ","
		}
		jsonVal += fmt.Sprintf("{\t\"id\":%d,\t\"name\":\"item-%d\"}", i, i)
	}
	jsonVal += `]}`

	m := New()
	m.Width = 100
	m.Height = 24
	m.Screen = ScreenBrowser
	m.Client = &store.Client{}
	m.Info = &store.ServerInfo{Version: "7.2", UsedMemory: "1M", TotalKeys: 1}
	m.Config = &config.File{}
	m.Keys = []string{"demo:json"}
	m.SelectedKey = "demo:json"
	m.PanelFocus = panelDetail
	m.KeyDetail = &store.KeyDetail{
		Meta:   store.KeyMeta{Type: "string", Key: "demo:json"},
		String: jsonVal,
	}

	for {
		_, rightW := m.browserPanelWidths()
		pw := rightW - panelChromeCols
		vis := max(1, m.browserContentHeight()-4)
		limit := stringDetailScrollLimit(m.KeyDetail.String, pw, vis)
		if limit <= 0 || m.DetailScroll >= limit {
			break
		}
		m.detailMove(1)
	}

	out := m.View()
	if strings.ContainsRune(out, '\t') {
		t.Fatalf("rendered view leaked raw tab byte — terminal tab expansion would shift layout")
	}
	lines := strings.Split(out, "\n")
	if len(lines) != m.Height {
		t.Fatalf("view lines=%d want %d", len(lines), m.Height)
	}
	leftW, rightW := m.browserPanelWidths()
	panelStart := gridInfoRows
	panelEnd := panelStart + m.panelAreaLines()

	assertDetailCorner := func(lineIdx int, wantTopLeft, wantTopRight rune) {
		t.Helper()
		plain := stripANSI(lines[lineIdx])
		gotTL := runeAtDisplayCol(plain, leftW)
		gotTR := runeAtDisplayCol(plain, m.Width-1)
		if gotTL != wantTopLeft || gotTR != wantTopRight {
			t.Fatalf("detail panel corners on line %d: tl=%q want %q, tr=%q want %q (line=%q)",
				lineIdx, gotTL, wantTopLeft, gotTR, wantTopRight, plain)
		}
	}
	assertDetailCorner(panelStart, '┌', '┐')
	assertDetailCorner(panelEnd-1, '└', '┘')

	detailTop := stripANSI(lines[panelStart])[leftW+1 : m.Width-1]
	if !strings.Contains(detailTop, "─") {
		t.Fatalf("detail panel top edge missing horizontal border: %q", detailTop)
	}
	detailBottom := stripANSI(lines[panelEnd-1])[leftW+1 : m.Width-1]
	if !strings.Contains(detailBottom, "─") {
		t.Fatalf("detail panel bottom edge missing horizontal border: %q", detailBottom)
	}

	for i, l := range lines {
		if w := lipgloss.Width(l); w != m.Width {
			t.Fatalf("line %d width=%d want %d", i, w, m.Width)
		}
	}

	panelW := rightW - panelChromeCols
	body := m.renderDetailPanel(panelW, m.browserContentHeight())
	if strings.ContainsRune(body, '\t') {
		t.Fatalf("renderDetailPanel leaked raw tab byte")
	}
	for i, l := range strings.Split(body, "\n") {
		if w := lipgloss.Width(l); w > panelW {
			t.Fatalf("body line %d width=%d exceeds panelW=%d: %q", i, w, panelW, l)
		}
	}
}

func stripANSI(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	i := 0
	for i < len(s) {
		if s[i] == 0x1b && i+1 < len(s) && s[i+1] == '[' {
			j := i + 2
			for j < len(s) {
				c := s[j]
				j++
				if c >= 0x40 && c <= 0x7e {
					break
				}
			}
			i = j
			continue
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String()
}

func runeAtDisplayCol(s string, col int) rune {
	displayCol := 0
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		if displayCol == col {
			return r
		}
		displayCol++
		i += size
	}
	return 0
}
