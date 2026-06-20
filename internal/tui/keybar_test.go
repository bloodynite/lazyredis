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
