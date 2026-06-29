package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestOverlayCenterPlacesDialog(t *testing.T) {
	base := strings.Repeat("x", 40) + "\n" + strings.Repeat("y", 40)
	dialog := lipgloss.NewStyle().Border(lipgloss.NormalBorder()).Render("hello")
	out := overlayCenter(base, dialog, 40, 2)
	if !strings.Contains(out, "hello") {
		t.Fatal("expected dialog in overlay output")
	}
}

func TestOverlayCenterAnchorsBottomWhenOverflowing(t *testing.T) {
	base := strings.Repeat(".", 20)
	dialog := "TOP\nA\nB\nC\nBOTTOM"
	out := overlayCenter(base, dialog, 20, 3)
	if !strings.Contains(out, "BOTTOM") {
		t.Fatal("expected bottom line of dialog to remain visible when overflowing")
	}
}

func TestNewKeyValueHeightShrinksWithTypeSelector(t *testing.T) {
	m := New()
	m.Width = 80
	m.Height = 30
	m.EditMode = editExistingKey
	editH := m.newKeyValueHeight()

	m.EditMode = editNewKey
	m.NewKeyFocus = newKeyFieldKey
	newH := m.newKeyValueHeight()
	if newH != editH {
		t.Fatalf("new key (selector closed) height = %d, want %d", newH, editH)
	}

	m.NewKeyFocus = newKeyFieldType
	selectorH := m.newKeyValueHeight()
	if selectorH >= newH {
		t.Fatalf("textarea should shrink when type selector is open: selector=%d new=%d", selectorH, newH)
	}
	if selectorH < 3 {
		t.Fatalf("textarea height must stay usable: got %d", selectorH)
	}
}

func TestRenderConfirmModal(t *testing.T) {
	m := New()
	m.ConfirmAction = confirmDeleteKey
	m.ConfirmTarget = "demo:key"
	modal := m.renderConfirmModal()
	if modal == "" {
		t.Fatal("expected modal content")
	}
	if !strings.Contains(modal, "demo:key") {
		t.Fatal("expected target key in modal")
	}
}

func TestRenderEditModal(t *testing.T) {
	m := New()
	m.EditMode = editNewKey
	m.NewKeyTTL.SetValue("300")
	m.NewKeyName.SetValue("demo:new")
	m.NewKeyValue.SetValue("hello")
	modal := m.renderEditModal()
	if !strings.Contains(modal, "demo:new") {
		t.Fatal("expected key in modal")
	}
	if !strings.Contains(modal, "hello") {
		t.Fatal("expected value in modal")
	}
	if !strings.Contains(modal, "300") {
		t.Fatal("expected ttl in modal")
	}

	m.EditMode = editRefreshInterval
	m.RefreshIntervalCursor = refreshIntervalCursor(5)
	modal = m.renderEditModal()
	if !strings.Contains(modal, "5s") {
		t.Fatal("expected refresh value 5s in modal")
	}

	m.EditMode = editTTL
	m.SelectedKey = "demo:key"
	m.NewKeyTTL.SetValue("300")
	modal = m.renderEditModal()
	if !strings.Contains(modal, "demo:key") {
		t.Fatal("expected key in ttl modal")
	}
	if !strings.Contains(modal, "300") {
		t.Fatal("expected ttl value in modal")
	}
}

func TestEditUsesModal(t *testing.T) {
	m := New()
	m.EditMode = editNewKey
	if !m.editUsesModal() {
		t.Fatal("new key should use modal")
	}
	m.EditMode = editExistingKey
	if !m.editUsesModal() {
		t.Fatal("existing key edit should use modal")
	}
	m.EditMode = editRefreshInterval
	if !m.editUsesModal() {
		t.Fatal("refresh interval should use modal")
	}
	m.EditMode = editTTL
	if !m.editUsesModal() {
		t.Fatal("ttl edit should use modal")
	}
	m.EditMode = editElement
	if m.editUsesModal() {
		t.Fatal("element edit should not use key form modal")
	}
}
