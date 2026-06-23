package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/bloodynite/lazyredis/internal/config"
)

func newModelWithModifier(t *testing.T, modifier string) *Model {
	t.Helper()
	m := New()
	m.Config = &config.File{
		Settings: config.Settings{
			ShortcutModifier: modifier,
		},
	}
	return m
}

func TestSaveKeyUsesShortcutModifier(t *testing.T) {
	m := newModelWithModifier(t, "alt")
	if !m.matchAction(actionSave, "alt+s") {
		t.Fatal("expected alt+s to trigger save")
	}
	if m.matchAction(actionSave, "ctrl+s") {
		t.Fatal("ctrl+s should not trigger save when modifier is alt")
	}
}

func TestSaveCancelHintUsesShortcutModifier(t *testing.T) {
	m := newModelWithModifier(t, "alt")
	hint := m.saveCancelHint(actionSave)
	if !strings.Contains(hint, "alt+s save") {
		t.Fatalf("hint = %q, want alt save bind", hint)
	}
	if strings.Contains(hint, "ctrl+s") {
		t.Fatalf("hint = %q, should not show default save bind", hint)
	}
}

// TestShortcutModifierAppliesToForceQuit proves the modifier is global: when
// shortcut_modifier=alt, ctrl+c no longer quits and alt+c becomes the force
// quit binding instead.
func TestShortcutModifierAppliesToForceQuit(t *testing.T) {
	m := newModelWithModifier(t, "alt")
	if !m.matchAction(actionAppForceQuit, "alt+c") {
		t.Fatal("expected alt+c to trigger force quit when modifier is alt")
	}
	if m.matchAction(actionAppForceQuit, "ctrl+c") {
		t.Fatal("ctrl+c should not trigger force quit when modifier is alt")
	}
}

// TestShortcutModifierAppliesToFlush proves the same global transform
// affects browser-scoped ctrl+ bindings (flush db).
func TestShortcutModifierAppliesToFlush(t *testing.T) {
	m := newModelWithModifier(t, "alt")
	if !m.matchAction(actionBrowserFlush, "alt+f") {
		t.Fatal("expected alt+f to trigger flush when modifier is alt")
	}
	if m.matchAction(actionBrowserFlush, "ctrl+f") {
		t.Fatal("ctrl+f should not trigger flush when modifier is alt")
	}
}

// TestShortcutModifierDefaultKeepsCtrl verifies the transform is a no-op
// when shortcut_modifier is unset or set to ctrl, so the default keymap
// keeps its existing behavior.
func TestShortcutModifierDefaultKeepsCtrl(t *testing.T) {
	cases := []*config.File{nil, {}, {Settings: config.Settings{}}, {Settings: config.Settings{ShortcutModifier: "ctrl"}}}
	for _, c := range cases {
		m := New()
		m.Config = c
		if !m.matchAction(actionSave, "ctrl+s") {
			t.Fatalf("expected ctrl+s save with default modifier (cfg=%+v)", c)
		}
		if !m.matchAction(actionAppForceQuit, "ctrl+c") {
			t.Fatalf("expected ctrl+c force quit with default modifier (cfg=%+v)", c)
		}
		if !m.matchAction(actionBrowserFlush, "ctrl+f") {
			t.Fatalf("expected ctrl+f flush with default modifier (cfg=%+v)", c)
		}
	}
}

// TestShortcutModifierLeavesNonCtrlKeysAlone proves the transform only
// rewrites the ctrl+ prefix; single keys, "tab", "shift+tab" and
// multi-binding rows stay intact so the keybar and matching logic don't
// regress.
func TestShortcutModifierLeavesNonCtrlKeysAlone(t *testing.T) {
	m := newModelWithModifier(t, "alt")
	cases := map[string][]string{
		actionBrowserFilter:           {"/"},
		actionBrowserTab:              {"tab"},
		actionFormShiftTab:            {"shift+tab"},
		actionBrowserCopy:             {"c"},
		actionProfilesQuit:            {"q"},
		actionConfirmNo:               {"n", "esc"},
		actionBrowserDetailSearchPrev: {"N"},
	}
	for action, want := range cases {
		got := m.bindKeys(action)
		if !equalSlices(got, want) {
			t.Fatalf("bindKeys(%q) = %v, want %v (modifier should not change non-ctrl binds)", action, got, want)
		}
	}
}

// TestShortcutModifierRewritesEveryCtrlBinding enumerates every action whose
// default key starts with ctrl+ and asserts the rewrite fires for each.
// This is the structural guarantee that nothing slips back to a save-only
// special case.
func TestShortcutModifierRewritesEveryCtrlBinding(t *testing.T) {
	for action, defaults := range defaultKeyMap {
		for _, key := range defaults {
			if !strings.HasPrefix(key, "ctrl+") {
				continue
			}
			defaultModel := New()
			if got := defaultModel.bindKeys(action); !containsString(got, key) {
				t.Fatalf("default bindKeys(%q) should contain %q, got %v", action, key, got)
			}
			altModel := newModelWithModifier(t, "alt")
			want := "alt+" + strings.TrimPrefix(key, "ctrl+")
			if got := altModel.bindKeys(action); !containsString(got, want) {
				t.Fatalf("with modifier=alt bindKeys(%q) should contain %q, got %v", action, want, got)
			}
			if got := altModel.bindKeys(action); containsString(got, key) {
				t.Fatalf("with modifier=alt bindKeys(%q) must not contain %q, got %v", action, key, got)
			}
		}
	}
}

func TestHelpModalDescribesModifierAsGlobal(t *testing.T) {
	m := New()
	m.Width = 100
	m.Height = 30
	out := m.renderHelpModal()
	if strings.Contains(out, "Customize save shortcuts") {
		t.Fatalf("help modal must not describe modifier as save-only; got %q", out)
	}
	if !strings.Contains(out, "shortcut_modifier") {
		t.Fatalf("help modal should still mention shortcut_modifier; got %q", out)
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

func containsString(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}

func equalSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
