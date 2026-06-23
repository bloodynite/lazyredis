package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/bloodynite/lazyredis/internal/config"
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