package tui

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/bloodynite/lazyredis/internal/config"
	"github.com/bloodynite/lazyredis/internal/store"
)

func TestEditRefreshIntervalAcceptsValidValues(t *testing.T) {
	cases := []int{0, 5, 10, 15, 30, 60}
	for _, v := range cases {
		idx := refreshIntervalCursor(v)
		m := New()
		m.EditMode = editRefreshInterval
		m.Config = &config.File{}
		m.RefreshIntervalCursor = idx

		_, cmd := m.updateRefreshIntervalModal(tea.KeyMsg{Type: tea.KeyEnter})
		if cmd == nil {
			t.Errorf("sec=%d: expected save cmd", v)
		}
	}
}

func TestEditRefreshIntervalCtrlSSaves(t *testing.T) {
	m := New()
	m.EditMode = editRefreshInterval
	m.Config = &config.File{}
	m.RefreshIntervalCursor = refreshIntervalCursor(10)

	_, cmd := m.updateRefreshIntervalModal(tea.KeyMsg{Type: tea.KeyCtrlS})
	if cmd == nil {
		t.Fatal("expected save cmd from ctrl+s")
	}
}

func TestEditRefreshIntervalNavigatesChoices(t *testing.T) {
	m := New()
	m.EditMode = editRefreshInterval
	m.RefreshIntervalCursor = 0

	m.updateRefreshIntervalModal(tea.KeyMsg{Type: tea.KeyDown})
	if m.RefreshIntervalCursor != 1 {
		t.Fatalf("down should move cursor to 1, got %d", m.RefreshIntervalCursor)
	}
	m.updateRefreshIntervalModal(tea.KeyMsg{Type: tea.KeyDown})
	if m.RefreshIntervalCursor != 2 {
		t.Fatalf("down should move cursor to 2, got %d", m.RefreshIntervalCursor)
	}
	m.updateRefreshIntervalModal(tea.KeyMsg{Type: tea.KeyUp})
	if m.RefreshIntervalCursor != 1 {
		t.Fatalf("up should move cursor to 1, got %d", m.RefreshIntervalCursor)
	}
	m.updateRefreshIntervalModal(tea.KeyMsg{Type: tea.KeyUp})
	m.updateRefreshIntervalModal(tea.KeyMsg{Type: tea.KeyUp})
	if m.RefreshIntervalCursor != len(refreshIntervalChoices)-1 {
		t.Fatalf("up from 0 should wrap to last, got %d", m.RefreshIntervalCursor)
	}
	m.updateRefreshIntervalModal(tea.KeyMsg{Type: tea.KeyDown})
	if m.RefreshIntervalCursor != 0 {
		t.Fatalf("down from last should wrap to 0, got %d", m.RefreshIntervalCursor)
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
