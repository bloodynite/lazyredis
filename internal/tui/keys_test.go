package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/bloodynite/lazyredis/internal/config"
)

func TestParseKeyList(t *testing.T) {
	keys := parseKeyList(" ctrl+r , alt+f ,enter ")
	if len(keys) != 3 {
		t.Fatalf("expected 3 keys, got %d", len(keys))
	}
	if keys[0] != "ctrl+r" || keys[1] != "alt+f" || keys[2] != "enter" {
		t.Fatalf("unexpected keys %#v", keys)
	}
}

func TestCustomKeybindingOverride(t *testing.T) {
	m := New()
	m.Config = &config.File{
		Settings: config.Settings{
			Keybindings: map[string]string{
				actionBrowserRefresh: "ctrl+r",
			},
		},
	}
	if !m.matchAction(actionBrowserRefresh, "ctrl+r") {
		t.Fatal("expected ctrl+r to trigger refresh")
	}
	if m.matchAction(actionBrowserRefresh, "r") {
		t.Fatal("default refresh key should be overridden")
	}
}

func TestSaveCancelHintUsesCustomBinding(t *testing.T) {
	m := New()
	m.Config = &config.File{
		Settings: config.Settings{
			Keybindings: map[string]string{
				actionEditEnter: "ctrl+p",
			},
		},
	}
	hint := m.editEnterSaveCancelHint()
	if !strings.Contains(hint, "ctrl+p save") {
		t.Fatalf("hint = %q, want custom save bind", hint)
	}
	if strings.Contains(hint, "ctrl+s") {
		t.Fatalf("hint = %q, should not show default save bind", hint)
	}
}

func TestHelpToggle(t *testing.T) {
	m := New()
	m.Screen = ScreenProfiles
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	m = next.(*Model)
	if !m.HelpOpen {
		t.Fatal("expected help to open")
	}
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	m = next.(*Model)
	if m.HelpOpen {
		t.Fatal("expected help to close")
	}
}
