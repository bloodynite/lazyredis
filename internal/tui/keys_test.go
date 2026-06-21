package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/bloodynite/lazyredis/internal/config"
)

func TestSaveKeyUsesShortcutModifier(t *testing.T) {
	m := New()
	m.Config = &config.File{
		Settings: config.Settings{
			ShortcutModifier: "alt",
		},
	}
	if !m.matchAction(actionSave, "alt+s") {
		t.Fatal("expected alt+s to trigger save")
	}
	if m.matchAction(actionSave, "ctrl+s") {
		t.Fatal("ctrl+s should not trigger save when modifier is alt")
	}
}

func TestSaveCancelHintUsesShortcutModifier(t *testing.T) {
	m := New()
	m.Config = &config.File{
		Settings: config.Settings{
			ShortcutModifier: "alt",
		},
	}
	hint := m.saveCancelHint(actionSave)
	if !strings.Contains(hint, "alt+s save") {
		t.Fatalf("hint = %q, want alt save bind", hint)
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
