package tui

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/bloodynite/lazyredis/internal/config"
	"github.com/bloodynite/lazyredis/internal/store"
)

func TestEditRefreshIntervalRejectsBelowMinimum(t *testing.T) {
	cfg := &config.File{}
	cases := []string{"1", "3", "4", "-1", "abc"}
	for _, v := range cases {
		m := New()
		m.EditMode = editRefreshInterval
		m.Config = cfg
		m.EditInput.SetValue(v)

		next, _ := m.updateEditInput(tea.KeyMsg{Type: tea.KeyCtrlS})
		nm := next.(*Model)
		if nm.ErrMsg == "" {
			t.Errorf("sec=%q: expected ErrMsg, got empty", v)
		}
	}
}

func TestEditRefreshIntervalAcceptsValidValues(t *testing.T) {
	cfg := &config.File{}
	cases := []string{"0", "5", "10", "60"}
	for _, v := range cases {
		m := New()
		m.EditMode = editRefreshInterval
		m.Config = cfg
		m.EditInput.SetValue(v)

		_, cmd := m.updateEditInput(tea.KeyMsg{Type: tea.KeyCtrlS})
		if cmd == nil {
			t.Errorf("sec=%q: expected save cmd", v)
		}
	}
}

func TestEditRefreshIntervalSaveResetsCountdownAndTriggersRefresh(t *testing.T) {
	m := New()
	m.Screen = ScreenKeyEdit
	m.EditMode = editRefreshInterval
	m.Config = &config.File{}
	m.Client = &store.Client{}
	m.SelectedKey = "demo:key"
	m.Loading = false
	m.RefreshStartedAt = time.Now().Add(-30 * time.Second)
	beforeSave := time.Now()

	next, cmd := m.Update(actionDoneMsg{status: "auto refresh 10s"})
	nm := next.(*Model)

	if nm.Screen != ScreenBrowser {
		t.Fatalf("screen = %v, want Browser", nm.Screen)
	}
	if !nm.Loading {
		t.Fatal("expected Loading=true after immediate refresh")
	}
	if !nm.RefreshStartedAt.After(beforeSave.Add(-time.Second)) {
		t.Fatalf("RefreshStartedAt not reset, got %v (before=%v)", nm.RefreshStartedAt, beforeSave)
	}
	if cmd == nil {
		t.Fatal("expected batch cmd (immediate refresh + reschedule + status clear)")
	}
}

func TestEditRefreshIntervalSaveOffTriggersImmediateRefreshWithoutReschedule(t *testing.T) {
	m := New()
	m.Screen = ScreenKeyEdit
	m.EditMode = editRefreshInterval
	m.Config = &config.File{}
	m.Client = &store.Client{}
	m.SelectedKey = "demo:key"
	m.Loading = false

	next, cmd := m.Update(actionDoneMsg{status: "auto refresh off"})
	nm := next.(*Model)

	if !nm.Loading {
		t.Fatal("off mode should still trigger immediate refresh")
	}
	if cmd == nil {
		t.Fatal("expected batch cmd (immediate refresh + status clear, no reschedule)")
	}
}