package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

type keyBind struct {
	Key  string
	Desc string
}

func (m *Model) keyBinds() []keyBind {
	defs := m.applicableHelpActions()
	binds := make([]keyBind, 0, len(defs)+2)
	for _, def := range defs {
		binds = append(binds, m.bindEntry(def.id, def.desc))
	}
	binds = m.appendHelpBind(binds)
	binds = append(binds, m.bindEntry(actionAppForceQuit, "force quit"))
	return binds
}

func (m *Model) keybarBinds() (main []keyBind, pinned []keyBind) {
	defs := m.applicableHelpActions()
	pinnedIDs := map[string]struct{}{}
	if m.Screen == ScreenBrowser && !m.SearchFocus {
		if m.ScanCursor != 0 {
			pinnedIDs[actionBrowserMoreKeys] = struct{}{}
		}
		if m.KeyDetail != nil && m.SelectedKey != "" {
			pinnedIDs[actionBrowserCopy] = struct{}{}
		}
		if m.PanelFocus == panelDetail && m.KeyDetail != nil && !m.DetailSearchFocus {
			pinnedIDs[actionBrowserFilter] = struct{}{}
			if compositeKeyType(m.KeyDetail.Meta.Type) {
				pinnedIDs[actionBrowserDetailAdd] = struct{}{}
				pinnedIDs[actionBrowserDetailEdit] = struct{}{}
				pinnedIDs[actionBrowserDetailDelete] = struct{}{}
			} else if m.KeyDetail.Meta.Type == "string" {
				pinnedIDs[actionBrowserDetailEdit] = struct{}{}
				pinnedIDs[actionBrowserDelete] = struct{}{}
			}
			if m.SelectedKey != "" {
				pinnedIDs[actionBrowserTTL] = struct{}{}
			}
		} else if m.SelectedKey != "" {
			pinnedIDs[actionBrowserTTL] = struct{}{}
			pinnedIDs[actionBrowserDelete] = struct{}{}
			if m.KeyDetail != nil && editableKeyType(m.KeyDetail.Meta.Type) {
				pinnedIDs[actionBrowserEdit] = struct{}{}
			}
		}
	}
	for _, def := range defs {
		b := m.bindEntry(def.id, def.desc)
		if _, ok := pinnedIDs[def.id]; ok {
			pinned = append(pinned, b)
			continue
		}
		main = append(main, b)
	}
	pinned = append(pinned, m.bindEntry(actionAppHelp, "help"))
	pinned = append(pinned, m.bindEntry(actionAppForceQuit, "force quit"))
	return main, pinned
}

func (m *Model) keybarLineCount() int {
	return gridKeybarRows
}

func (m *Model) keybarLayoutLines() []string {
	main, pinned := m.keybarBinds()
	maxWidth := max(1, m.Width-2)
	all := append(append([]keyBind{}, main...), pinned...)
	if renderBindLine(all) == "" {
		return nil
	}
	if lipgloss.Width(renderBindLine(all)) <= maxWidth {
		return []string{renderBindLine(all)}
	}

	line1 := packKeyBindsLine(main, maxWidth)
	remainingMain := main[len(line1):]
	row2Binds := append(append([]keyBind{}, pinned...), remainingMain...)
	line2 := renderBindLine(row2Binds)
	if lipgloss.Width(line2) > maxWidth {
		line2 = truncateKeybar(line2, maxWidth)
	}
	if line2 == "" {
		return []string{renderBindLine(line1)}
	}
	return []string{renderBindLine(line1), line2}
}

func packKeyBindsLine(binds []keyBind, maxWidth int) []keyBind {
	if len(binds) == 0 {
		return nil
	}
	sep := keySepStyle.Render(" │ ")
	sepW := lipgloss.Width(sep)
	var packed []keyBind
	width := 0
	for _, b := range binds {
		part := keyLabelStyle.Render(b.Key) + keyDescStyle.Render(" "+b.Desc)
		addW := lipgloss.Width(part)
		if len(packed) > 0 {
			addW += sepW
		}
		if len(packed) > 0 && width+addW > maxWidth {
			break
		}
		if len(packed) > 0 {
			width += sepW
		}
		width += lipgloss.Width(part)
		packed = append(packed, b)
	}
	return packed
}

func (m *Model) layoutHeights() (content, status int) {
	return m.panelAreaLines(), gridStatusRows
}

func (m *Model) renderStatusLine() string {
	msg := m.statusMessageText()
	maxW := max(1, m.Width-2)
	var line string
	if msg != "" {
		line = " " + truncateStyled(m.styleStatusMessage(msg), maxW)
	} else {
		line = " "
	}
	return m.renderFixedRow(line, statusBarStyle)
}

func (m *Model) styleStatusMessage(msg string) string {
	switch {
	case m.ErrMsg != "":
		return errorStyle.Render(msg)
	case m.Loading:
		return statusStyle.Render(msg)
	default:
		return statusStyle.Render(msg)
	}
}

func truncatePlain(s string, maxW int) string {
	if maxW <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= maxW {
		return s
	}
	runes := []rune(s)
	for len(runes) > 0 && lipgloss.Width(string(runes)) > maxW-1 {
		runes = runes[:len(runes)-1]
	}
	return string(runes) + "…"
}

func truncateStyled(s string, maxW int) string {
	if maxW <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= maxW {
		return s
	}
	return ansi.Truncate(s, maxW, "…")
}

func (m *Model) renderKeybar() string {
	lines := m.keybarLayoutLines()
	for len(lines) < gridKeybarRows {
		lines = append(lines, "")
	}
	if len(lines) > gridKeybarRows {
		lines = lines[:gridKeybarRows]
	}
	rendered := make([]string, len(lines))
	for i, line := range lines {
		rendered[i] = " " + line
	}
	inner := strings.Join(rendered, "\n")
	return keybarStyle.Width(max(1, m.Width)).Height(gridKeybarRows).MaxHeight(gridKeybarRows).Render(inner)
}

func renderBindLine(binds []keyBind) string {
	if len(binds) == 0 {
		return ""
	}
	parts := make([]string, 0, len(binds))
	for _, b := range binds {
		parts = append(parts, keyLabelStyle.Render(b.Key)+keyDescStyle.Render(" "+b.Desc))
	}
	return strings.Join(parts, keySepStyle.Render(" │ "))
}

func truncateKeybar(line string, maxWidth int) string {
	if maxWidth <= 0 {
		return ""
	}
	for len(line) > 0 && lipgloss.Width(line) > maxWidth {
		idx := strings.LastIndex(line, " │ ")
		if idx <= 0 {
			break
		}
		line = line[:idx]
	}
	if lipgloss.Width(line) > maxWidth {
		runes := []rune(line)
		for len(runes) > 0 && lipgloss.Width(string(runes)) > maxWidth-1 {
			runes = runes[:len(runes)-1]
		}
		line = string(runes) + "…"
	}
	return line
}
