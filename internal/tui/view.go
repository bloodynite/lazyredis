package tui

import (
	"fmt"
	"runtime/debug"
	"strings"
	"unicode/utf8"

	"github.com/bloodynite/lazyredis/internal/config"
	"github.com/bloodynite/lazyredis/internal/store"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
)

func (m *Model) View() string {
	if m.Width == 0 {
		return "Loading..."
	}

	var out string
	if m.Client != nil {
		switch m.Screen {
		case ScreenKeyEdit:
			if m.editUsesModal() {
				out = m.viewBrowserWithEditModal()
			} else {
				out = m.viewBrowserWithBodyOverlay(m.viewKeyEdit())
			}
		case ScreenConfirm:
			out = m.viewBrowserWithConfirmModal()
		default:
			out = m.viewBrowserLayout()
		}
	} else if m.Screen == ScreenConfirm {
		out = m.viewDisconnectedWithConfirmModal()
	} else {
		out = m.viewDisconnectedLayout()
	}

	if m.HelpOpen {
		out = m.applyHelpOverlay(out)
	}
	return fitViewHeight(out, m.Height)
}

func (m *Model) viewDisconnectedLayout() string {
	return m.renderAppChrome(m.renderContent())
}

func (m *Model) viewDisconnectedWithConfirmModal() string {
	contentHeight := m.panelAreaLines()
	body := m.viewProfiles(contentHeight)
	if m.PrevScreen == ScreenProfileForm {
		body = m.viewProfileForm()
	}
	modal := m.renderConfirmModal()
	overlay := overlayCenter(dimContent(body), modal, m.Width, contentHeight)
	return m.renderAppChrome(lipgloss.NewStyle().Height(contentHeight).Render(overlay))
}

func (m *Model) viewBrowserWithConfirmModal() string {
	return m.viewBrowserWithModal(m.renderConfirmModal())
}

func (m *Model) viewBrowserWithEditModal() string {
	return m.viewBrowserWithModal(m.renderEditModal())
}

func (m *Model) viewBrowserWithModal(modal string) string {
	areaH := m.panelAreaLines()
	panels := m.renderBrowserPanels()
	body := overlayCenter(dimContent(panels), modal, m.Width, areaH)
	return m.renderAppChrome(body)
}

func (m *Model) viewBrowserWithBodyOverlay(overlay string) string {
	return m.renderAppChrome(lipgloss.NewStyle().Height(m.panelAreaLines()).Render(overlay))
}

func (m *Model) viewBrowserLayout() string {
	return m.renderAppChrome(m.renderBrowserPanels())
}

const (
	gridInfoRows           = 2
	gridStatusRows         = 1
	gridKeybarRows         = 2
	gridPanelBorderRows    = 2
	gridFixedRows          = gridInfoRows + gridStatusRows + gridKeybarRows
	panelHorizontalPadding = 2
	panelChromeCols        = 4
)

func (m *Model) panelAreaLines() int {
	h := m.Height - gridFixedRows
	if h < 4 {
		h = 4
	}
	return h
}

func (m *Model) panelInnerHeight() int {
	h := m.panelAreaLines() - gridPanelBorderRows
	if h < 2 {
		h = 2
	}
	return h
}

func (m *Model) browserPanelAreaHeight() int {
	return m.panelAreaLines()
}

func (m *Model) browserContentHeight() int {
	return m.panelInnerHeight()
}

func (m *Model) mainContentHeight(extraRows int) int {
	h := m.panelAreaLines() - extraRows
	if h < 4 {
		h = 4
	}
	return h
}

func (m *Model) renderFixedRow(content string, style lipgloss.Style) string {
	w := max(1, m.Width)
	return style.Width(w).Height(1).MaxHeight(1).Render(content)
}

func (m *Model) renderAppChrome(content string) string {
	return lipgloss.JoinVertical(lipgloss.Top,
		m.renderInfoLine1(),
		m.renderInfoLine2(),
		content,
		m.renderStatusLine(),
		m.renderKeybar(),
	)
}

func (m *Model) renderInfoLine1() string {
	prefix := appHeaderPrefix()
	var line string
	if m.Client != nil {
		p := m.Client.Profile()
		line = fmt.Sprintf("%s - %s  %s  db %d", prefix, p.Name, p.Addr, p.DB)
	} else {
		screen := m.Screen
		if m.Screen == ScreenConfirm {
			screen = m.PrevScreen
		}
		line = fmt.Sprintf("%s - %s", prefix, screenName(screen))
	}
	return m.renderFixedRow(" "+truncatePlain(line, max(1, m.Width-2)), headerBarStyle)
}

func (m *Model) renderInfoLine2() string {
	var line string
	if m.Client != nil {
		if m.Info == nil {
			line = "loading info…"
		} else {
			line = strings.Join([]string{
				"v" + m.Info.Version,
				"mem " + m.Info.UsedMemory,
				fmt.Sprintf("keys %d", m.Info.TotalKeys),
				"clients " + m.Info.Connected,
				"ops/s " + m.Info.OpsPerSec,
				m.Info.Role,
				"uptime " + m.Info.Uptime + "s",
				"auto " + m.autoRefreshLabel(),
			}, "  ·  ")
		}
	}
	text := " " + subtitleStyle.Render(truncatePlain(line, max(1, m.Width-2)))
	return m.renderFixedRow(text, infoBarStyle)
}

func (m *Model) statusMessageText() string {
	switch {
	case m.ErrMsg != "":
		return m.ErrMsg
	case m.Loading:
		return m.Spinner.View() + " loading…"
	default:
		return m.Status
	}
}

func (m *Model) autoRefreshLabel() string {
	sec := config.DefaultRefreshIntervalSec
	if m.Config != nil {
		sec = m.Config.GetRefreshIntervalSec()
	}
	if sec <= 0 {
		return "off"
	}
	return fmt.Sprintf("%ds", sec)
}

func (m *Model) browserPanelWidths() (left, right int) {
	left = max(16, m.Width/5)
	if left > m.Width-20 {
		left = max(16, m.Width/4)
	}
	right = m.Width - left
	if right < 20 {
		right = 20
		left = max(16, m.Width-right)
	}
	return left, right
}

func (m *Model) renderBrowserPanels() string {
	leftW, rightW := m.browserPanelWidths()
	height := m.browserContentHeight()

	leftStyle := panelStyle
	if m.PanelFocus == panelKeys {
		leftStyle = panelFocusedStyle
	}
	rightStyle := panelStyle
	if m.PanelFocus == panelDetail {
		rightStyle = panelFocusedStyle
	}

	left := renderTitledPanel(leftStyle, leftW, height, "Keys", m.renderKeysPanel(leftW-panelChromeCols, height))
	right := renderTitledPanel(rightStyle, rightW, height, "Detail", m.renderDetailPanel(rightW-panelChromeCols, height))
	return lipgloss.JoinHorizontal(lipgloss.Top, left, right)
}

func renderTitledPanel(style lipgloss.Style, outerWidth, height int, title, body string) string {
	panel := style.Width(outerWidth - panelHorizontalPadding).Height(height).Render(body)
	return injectPanelTitle(panel, title, style)
}

func injectPanelTitle(panel, title string, style lipgloss.Style) string {
	if title == "" {
		return panel
	}

	lines := strings.Split(panel, "\n")
	if len(lines) == 0 {
		return panel
	}

	topWidth := lipgloss.Width(lines[0])
	if topWidth < 4 {
		return panel
	}

	title = truncatePlain(title, topWidth-4)
	label := " " + panelTitleStyle.Render(title) + " "
	fillerWidth := topWidth - 2 - lipgloss.Width(label)
	if fillerWidth < 0 {
		fillerWidth = 0
	}

	borderStyle := lipgloss.NewStyle().
		Foreground(style.GetBorderTopForeground()).
		Background(style.GetBorderTopBackground())
	lines[0] = borderStyle.Render("┌") + label + borderStyle.Render(strings.Repeat("─", fillerWidth)) + borderStyle.Render("┐")
	return strings.Join(lines, "\n")
}

func clipPanelLines(lines []string, maxLines int) string {
	if maxLines < 1 {
		maxLines = 1
	}
	if len(lines) > maxLines {
		lines = lines[:maxLines]
	}
	for len(lines) < maxLines {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

func clipPanelLinesKeepFooter(lines []string, maxLines int) string {
	if maxLines < 1 {
		maxLines = 1
	}
	if len(lines) <= maxLines {
		return clipPanelLines(lines, maxLines)
	}
	footer := lines[len(lines)-1]
	head := lines[:len(lines)-1]
	if len(head) > maxLines-1 {
		head = head[:maxLines-1]
	}
	for len(head) < maxLines-1 {
		head = append(head, "")
	}
	head = append(head, footer)
	return strings.Join(head, "\n")
}

func (m *Model) renderKeysPanel(panelW, height int) string {
	const titleRows = 1
	const metaRows = 1

	title := panelTitleStyle.Render("Keys")
	pattern := m.ScanPattern
	if pattern == "" {
		pattern = "*"
	}
	if m.SearchFocus {
		m.SearchInput.Width = max(4, panelW-6)
		title += "  " + m.SearchInput.View()
	} else {
		title += "  " + helpStyle.Render(pattern)
	}

	listH := max(1, height-titleRows-metaRows)
	var lines []string
	lines = append(lines, title)

	var contentLines []string
	if len(m.Keys) == 0 {
		contentLines = append(contentLines, normalStyle.Render("  no keys"))
	} else {
		end := min(len(m.Keys), m.KeyScroll+listH)
		for i := m.KeyScroll; i < end; i++ {
			key := m.Keys[i]
			if lipgloss.Width(key) > panelW-4 {
				key = truncate(key, panelW-4)
			}
			prefix := "  "
			if i == m.KeyCursor && m.PanelFocus == panelKeys {
				prefix = "▸ "
			}
			line := prefix + key
			if i == m.KeyCursor {
				contentLines = append(contentLines, selectedStyle.Render(line))
			} else {
				contentLines = append(contentLines, normalStyle.Render(line))
			}
		}
	}

	bodySlots := listH
	for len(contentLines) < bodySlots {
		contentLines = append(contentLines, "")
	}
	if len(contentLines) > bodySlots {
		contentLines = contentLines[:bodySlots]
	}
	lines = append(lines, contentLines...)

	meta := truncatePlain(m.keysPanelMeta(), max(8, panelW-4))
	lines = append(lines, helpStyle.Render("  "+meta))
	return clipPanelLinesKeepFooter(lines, height)
}

func (m *Model) keysPanelMeta() string {
	loaded := len(m.Keys)
	if loaded == 0 && m.ScanCursor == 0 {
		return "0 keys"
	}
	if m.ScanCursor != 0 {
		if m.ScanPattern == "*" && m.Info != nil && m.Info.TotalKeys > int64(loaded) {
			return fmt.Sprintf("%d/%d · g", loaded, m.Info.TotalKeys)
		}
		return fmt.Sprintf("%d · g", loaded)
	}
	if m.ScanPattern == "*" && m.Info != nil {
		return fmt.Sprintf("%d/%d keys", loaded, m.Info.TotalKeys)
	}
	return fmt.Sprintf("%d keys loaded", loaded)
}

func (m *Model) renderDetailPanel(panelW, height int) string {
	title := panelTitleStyle.Render("Detail")
	if m.SelectedKey != "" {
		title = panelTitleStyle.Render(truncate(m.SelectedKey, panelW-4))
	}

	var lines []string
	lines = append(lines, title)
	if m.KeyDetail == nil {
		if m.Loading && m.SelectedKey != "" {
			lines = append(lines, normalStyle.Render("  loading…"))
		} else {
			lines = append(lines, normalStyle.Render("  select a key"))
		}
		return clipPanelLines(lines, height)
	}

	d := m.KeyDetail
	extraRows := 0
	if m.DetailSearchFocus {
		m.DetailSearchInput.Width = max(4, panelW-6)
		lines = append(lines, m.DetailSearchInput.View())
		extraRows = 1
	}
	lines = append(lines,
		fmt.Sprintf("  %s  %s",
			typeStyle(d.Meta.Type).Render(d.Meta.Type),
			subtitleStyle.Render("ttl "+store.FormatTTL(d.Meta.TTL)),
		),
	)

	listH := max(1, height-2-extraRows)
	query := m.detailSearchQuery()
	body := m.renderDetailBody(d, panelW, listH, query)
	lines = append(lines, body...)
	return clipPanelLines(lines, height)
}

func (m *Model) detailSearchQuery() string {
	if m.DetailSearchInput.Value() == "" {
		return ""
	}
	if m.DetailSearchFocus {
		return ""
	}
	return m.DetailSearchInput.Value()
}

func (m *Model) isActiveDetailMatch(idx int) bool {
	if m.DetailSearchCursor < 0 || m.DetailSearchCursor >= len(m.DetailSearchMatches) {
		return false
	}
	return m.DetailSearchMatches[m.DetailSearchCursor] == idx
}

func (m *Model) renderDetailBody(d *store.KeyDetail, panelW, listH int, query string) []string {
	var lines []string
	inDetail := m.PanelFocus == panelDetail

	switch d.Meta.Type {
	case "string":
		activeChunk, activeOffset := -1, -1
		if m.DetailSearchCursor >= 0 && m.DetailSearchCursor < len(m.DetailSearchMatches) {
			maxW := max(8, panelW-4)
			pos := m.DetailSearchMatches[m.DetailSearchCursor]
			activeChunk, activeOffset = chunkPositionForByteOffset(d.String, maxW, pos)
		}
		lines = append(lines, wrapValueWithQuery("value", d.String, query, panelW, listH, m.DetailScroll, activeChunk, activeOffset)...)
	case "hash":
		fields := hashFields(d.Hash)
		end := min(len(fields), m.DetailScroll+listH)
		for i := m.DetailScroll; i < end; i++ {
			lines = append(lines, m.renderCompositeRow(query, inDetail, i, func() string {
				f := fields[i]
				val := truncate(d.Hash[f], max(8, panelW-lipgloss.Width(f)-6))
				return fmt.Sprintf("%s%s = %s", compositeRowPrefix(i, inDetail, m.DetailCursor), f, val)
			})...)
		}
	case "list":
		end := min(len(d.List), m.DetailScroll+listH)
		for i := m.DetailScroll; i < end; i++ {
			lines = append(lines, m.renderCompositeRow(query, inDetail, i, func() string {
				val := truncate(d.List[i], max(8, panelW-8))
				return fmt.Sprintf("%s[%d] %s", compositeRowPrefix(i, inDetail, m.DetailCursor), i, val)
			})...)
		}
	case "set":
		end := min(len(d.Set), m.DetailScroll+listH)
		for i := m.DetailScroll; i < end; i++ {
			lines = append(lines, m.renderCompositeRow(query, inDetail, i, func() string {
				val := truncate(d.Set[i], max(8, panelW-4))
				return compositeRowPrefix(i, inDetail, m.DetailCursor) + val
			})...)
		}
	case "zset":
		end := min(len(d.ZSet), m.DetailScroll+listH)
		for i := m.DetailScroll; i < end; i++ {
			z := d.ZSet[i]
			lines = append(lines, m.renderCompositeRow(query, inDetail, i, func() string {
				return fmt.Sprintf("%s%s (%.2f)", compositeRowPrefix(i, inDetail, m.DetailCursor), z.Member, z.Score)
			})...)
		}
	case "stream":
		end := min(len(d.Stream), m.DetailScroll+listH)
		for i := m.DetailScroll; i < end; i++ {
			entry := d.Stream[i]
			lines = append(lines, m.renderCompositeRow(query, inDetail, i, func() string {
				body := truncate(formatStreamEntry(entry), max(8, panelW-12))
				return fmt.Sprintf("%s%s  %s", compositeRowPrefix(i, inDetail, m.DetailCursor), entry.ID, body)
			})...)
		}
	}
	return lines
}

func compositeRowPrefix(i int, inDetail bool, cursor int) string {
	if inDetail && i == cursor {
		return "▸ "
	}
	return "  "
}

const detailNewlineMarker = "↵"
const detailTabSpaces = "    "

func sanitizeDetailRow(s string) string {
	if !strings.ContainsAny(s, "\r\n\t") {
		return s
	}
	s = strings.ReplaceAll(s, "\r\n", detailNewlineMarker)
	s = strings.ReplaceAll(s, "\n", detailNewlineMarker)
	s = strings.ReplaceAll(s, "\r", detailNewlineMarker)
	s = strings.ReplaceAll(s, "\t", detailTabSpaces)
	return s
}

func (m *Model) renderCompositeRow(query string, inDetail bool, idx int, lineFn func() string) []string {
	line := sanitizeDetailRow(lineFn())
	isCursor := inDetail && idx == m.DetailCursor
	isActive := isCursor && m.isActiveDetailMatch(idx)
	switch {
	case isActive && query != "":
		rendered := normalStyle.Render(line)
		rendered = highlightAllWithStyle(rendered, query, activeSearchMatchStyle)
		return []string{rendered}
	case isCursor:
		return []string{selectedStyle.Render(line)}
	default:
		rendered := normalStyle.Render(line)
		if query != "" {
			rendered = highlightSubstring(rendered, query)
		}
		return []string{rendered}
	}
}

func formatStreamEntry(entry store.StreamEntry) string {
	names := hashFields(entry.Fields)
	parts := make([]string, 0, len(names))
	for _, name := range names {
		parts = append(parts, name+"="+entry.Fields[name])
	}
	return strings.Join(parts, " ")
}

func (m *Model) renderContent() string {
	contentHeight, _ := m.layoutHeights()
	var body string
	switch m.Screen {
	case ScreenProfiles:
		body = m.viewProfiles(contentHeight)
	case ScreenProfileForm:
		body = m.viewProfileForm()
	case ScreenKeyEdit:
		body = m.viewKeyEdit()
	default:
		body = ""
	}
	return lipgloss.NewStyle().Height(contentHeight).Render(body)
}

func screenName(s Screen) string {
	switch s {
	case ScreenProfiles:
		return "Profiles"
	case ScreenProfileForm:
		return "Profile"
	case ScreenBrowser:
		return "Redis"
	case ScreenKeyEdit:
		return "Edit"
	case ScreenConfirm:
		return "Confirm"
	default:
		return "lazyredis"
	}
}

func (m *Model) viewProfiles(maxLines int) string {
	if len(m.Profiles) == 0 {
		return normalStyle.Render("No profiles.")
	}
	visible := max(1, maxLines)
	scroll := 0
	if m.ProfileCursor >= visible {
		scroll = m.ProfileCursor - visible + 1
	}
	var lines []string
	end := min(len(m.Profiles), scroll+visible)
	for i := scroll; i < end; i++ {
		p := m.Profiles[i]
		line := fmt.Sprintf("%-16s  %s  db=%d  mode=%s", p.Name, p.Addr, p.DB, p.Mode)
		if hint := profileConnectionHint(p); hint != "" {
			line += "  " + hint
		}
		if i == m.ProfileCursor {
			lines = append(lines, selectedStyle.Render("▸ "+line))
		} else {
			lines = append(lines, normalStyle.Render("  "+line))
		}
	}
	return strings.Join(lines, "\n")
}

func profileConnectionHint(p config.Profile) string {
	var parts []string
	if p.TLS != nil && p.TLS.Enabled {
		parts = append(parts, "tls")
	}
	if p.SSHTunnel != nil && p.SSHTunnel.Enabled {
		parts = append(parts, "ssh")
	}
	if p.Proxy != nil && p.Proxy.Type != "" {
		parts = append(parts, p.Proxy.Type)
	}
	if len(parts) == 0 {
		return ""
	}
	return "[" + strings.Join(parts, "+") + "]"
}

func (m *Model) viewProfileForm() string {
	title := "New profile"
	if m.FormEditing {
		title = "Edit profile"
	}
	var lines []string
	lines = append(lines, subtitleStyle.Render(title))
	for i, label := range profileFormLabels {
		prefix := "  "
		if i == m.FormFocus {
			prefix = "▸ "
		}
		lines = append(lines, fmt.Sprintf("%s%s: %s", prefix, label, profileFormInputView(m.FormInputs, i)))
	}
	return strings.Join(lines, "\n")
}

func (m *Model) viewKeyEdit() string {
	var title string
	switch m.EditMode {
	case editNewKey:
		title = "New key"
	case editElement:
		title = "Edit item"
	case editElementAdd:
		title = "Add item"
	case editTTL:
		title = "TTL for: " + m.SelectedKey
	case editRefreshInterval:
		title = "Auto refresh interval (seconds, 0=off)"
	}
	if (m.EditMode == editElement || m.EditMode == editElementAdd) && m.elementEditUsesTextarea() {
		m.syncNewKeyLayout()
		return subtitleStyle.Render(title) + "\n" + m.NewKeyValue.View() + "\n" + confirmHintStyle.Render(m.editCtrlEnterSaveCancelHint())
	}
	hint := confirmHintStyle.Render(m.editEnterSaveCancelHint())
	if m.EditMode == editElement || m.EditMode == editElementAdd {
		return subtitleStyle.Render(title) + "\n" + m.EditInput.View() + "\n" + hint
	}
	return subtitleStyle.Render(title) + "\n" + m.EditInput.View() + "\n" + hint
}

func (m *Model) editUsesModal() bool {
	return m.EditMode == editNewKey || m.EditMode == editExistingKey || m.EditMode == editRefreshInterval || m.EditMode == editTTL
}

func (m *Model) renderEditModal() string {
	if m.EditMode == editNewKey || m.EditMode == editExistingKey {
		return m.renderKeyFormModal()
	}
	if m.EditMode == editTTL {
		return m.renderTTLModal()
	}
	inner := panelTitleStyle.Render("Auto refresh") + "\n\n" +
		m.EditInput.View() + "\n\n" +
		confirmHintStyle.Render(m.editEnterSaveCancelHint())
	width := min(56, max(36, lipgloss.Width(m.EditInput.View())+8))
	return confirmModalStyle.Width(width).Render(inner)
}

func (m *Model) renderTTLModal() string {
	m.syncNewKeyLayout()
	keyLine := subtitleStyle.Render(truncate(m.SelectedKey, 48))
	inner := panelTitleStyle.Render("TTL") + "\n\n" +
		keyLine + "\n\n" +
		fmt.Sprintf("TTL: %s", m.NewKeyTTL.View()) + "\n\n" +
		confirmHintStyle.Render(m.editEnterSaveCancelHint())
	width := min(56, max(36, lipgloss.Width(m.NewKeyTTL.View())+8))
	return confirmModalStyle.Width(width).Render(inner)
}

func (m *Model) renderKeyFormModal() string {
	m.syncNewKeyLayout()
	title := "New key"
	if m.EditMode == editExistingKey {
		title = "Edit key"
	}
	var lines []string
	lines = append(lines, panelTitleStyle.Render(title))
	lines = append(lines, "")

	ttlPrefix := "  "
	if m.NewKeyFocus == newKeyFieldTTL {
		ttlPrefix = "▸ "
	}
	lines = append(lines, fmt.Sprintf("%sTTL: %s", ttlPrefix, m.NewKeyTTL.View()))

	typePrefix := "  "
	if m.NewKeyFocus == newKeyFieldType {
		typePrefix = "▸ "
	}
	if m.EditMode == editNewKey && m.NewKeyFocus == newKeyFieldType {
		lines = append(lines, typePrefix+"Type:")
		lines = append(lines, m.renderKeyTypeSelector()...)
	} else {
		lines = append(lines, fmt.Sprintf("%sType: %s", typePrefix, m.KeyFormType))
	}

	keyPrefix := "  "
	if m.NewKeyFocus == newKeyFieldKey {
		keyPrefix = "▸ "
	}
	lines = append(lines, fmt.Sprintf("%sKey: %s", keyPrefix, m.NewKeyName.View()))

	valuePrefix := "  "
	if m.NewKeyFocus == newKeyFieldValue {
		valuePrefix = "▸ "
	}
	valueLabel := keyFormValueLabel(m.KeyFormType)
	lines = append(lines, valuePrefix+valueLabel+":")
	lines = append(lines, m.NewKeyValue.View())

	lines = append(lines, "", confirmHintStyle.Render(m.keyFormModalHint()))
	inner := strings.Join(lines, "\n")
	width := min(70, max(48, m.Width*2/3))
	return confirmModalStyle.Width(width).Render(inner)
}

func (m *Model) renderKeyTypeSelector() []string {
	lines := make([]string, 0, len(keyFormTypes))
	for i, t := range keyFormTypes {
		prefix := "    "
		if i == m.NewKeyTypeCursor {
			prefix = "  ▸ "
		}
		line := prefix + t
		if i == m.NewKeyTypeCursor {
			lines = append(lines, selectedStyle.Render(line))
		} else {
			lines = append(lines, normalStyle.Render(line))
		}
	}
	return lines
}

func keyFormValueLabel(keyType string) string {
	switch keyType {
	case "hash":
		return "Fields"
	case "list":
		return "Items"
	case "set":
		return "Members"
	case "zset":
		return "Scores"
	case "stream":
		return "Entries"
	default:
		return "Value"
	}
}

func (m *Model) syncNewKeyLayout() {
	if m.Width == 0 {
		return
	}
	inputW := min(62, max(36, m.Width*2/3-8))
	m.NewKeyTTL.Width = inputW
	m.NewKeyName.Width = inputW
	m.NewKeyValue.SetWidth(inputW)
	m.NewKeyValue.SetHeight(m.newKeyValueHeight())
}

func (m *Model) newKeyValueHeight() int {
	h := m.browserContentHeight() / 2
	if h < 4 {
		h = 4
	}
	if h > 12 {
		h = 12
	}
	return h
}

func (m *Model) renderConfirmModal() string {
	var msg string
	switch m.ConfirmAction {
	case confirmDeleteKey:
		msg = fmt.Sprintf("Delete key %q?", m.ConfirmTarget)
	case confirmFlushDB:
		msg = "Flush the entire current database?"
	case confirmDeleteProfile:
		msg = fmt.Sprintf("Delete profile %q?", m.ConfirmTarget)
	}
	inner := panelTitleStyle.Render("Confirm") + "\n\n" +
		confirmMsgStyle.Render(msg) + "\n\n" +
		confirmHintStyle.Render("y yes   n no")
	width := min(56, max(36, lipgloss.Width(msg)+6))
	return confirmModalStyle.Width(width).Render(inner)
}

func (m *Model) applyHelpOverlay(base string) string {
	height := max(lipgloss.Height(base), m.Height)
	if height < 1 {
		height = m.Height
	}
	padded := lipgloss.NewStyle().Width(max(1, m.Width)).Height(height).Render(base)
	modal := m.renderHelpModal()
	return overlayCenter(dimContent(padded), modal, m.Width, height)
}

func (m *Model) renderHelpModal() string {
	groups := m.helpGroups()
	var lines []string
	lines = append(lines, panelTitleStyle.Render("Keyboard shortcuts"))
	lines = append(lines, "")
	for _, g := range groups {
		if g.Title != "" {
			lines = append(lines, helpGroupTitleStyle.Render(g.Title))
		}
		for _, def := range g.Defs {
			lines = append(lines, fmt.Sprintf("  %-16s %s", formatBindKeys(m.bindKeys(def.id)), def.desc))
		}
		lines = append(lines, "")
	}
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	lines = append(lines, "",
		confirmHintStyle.Render("Customize shortcut modifier with settings.shortcut_modifier"),
		confirmHintStyle.Render("Use ctrl or alt"),
		"",
		confirmHintStyle.Render("Press ? or esc to close"),
	)
	inner := strings.Join(lines, "\n")
	width := min(72, max(48, m.Width*2/3))
	return confirmModalStyle.Width(width).Render(inner)
}

func appVersion() string {
	if bi, ok := debug.ReadBuildInfo(); ok && bi.Main.Version != "" && bi.Main.Version != "(devel)" {
		return bi.Main.Version
	}
	return "dev"
}

func appHeaderPrefix() string {
	return "Lazyredis " + appVersion()
}

func fitViewHeight(out string, height int) string {
	if height <= 0 {
		return out
	}
	lines := strings.Split(out, "\n")
	if len(lines) > height {
		lines = lines[:height]
	}
	for len(lines) < height {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

func wrapValue(label, value string, width, maxLines, scroll int) []string {
	return wrapValueWithQuery(label, value, "", width, maxLines, scroll, -1, -1)
}

func wrapValueWithQuery(label, value, query string, width, maxLines, scroll, activeChunk, activeOffset int) []string {
	if maxLines < 1 {
		maxLines = 1
	}
	maxW := max(8, width-4)
	chunks := chunkString(value, maxW)
	if scroll < 0 {
		scroll = 0
	}
	if scroll > len(chunks) {
		scroll = len(chunks)
	}
	var lines []string
	lines = append(lines, "  "+label+":")
	bodyVisible := maxLines - 1
	for i, chunk := range chunks[scroll:min(len(chunks), scroll+bodyVisible)] {
		actualChunkIdx := scroll + i
		var rendered string
		if actualChunkIdx == activeChunk && query != "" {
			rendered = highlightChunkActive(chunk, query, activeOffset)
		} else {
			rendered = highlightSubstring(chunk, query)
		}
		lines = append(lines, normalStyle.Render("  "+rendered))
	}
	return lines
}

func highlightSubstring(s, query string) string {
	if query == "" {
		return s
	}
	if !strings.Contains(strings.ToLower(s), strings.ToLower(query)) {
		return s
	}
	return highlightAllWithStyle(s, query, searchMatchStyle)
}

func highlightChunkActive(chunk, query string, activeOffset int) string {
	q := strings.ToLower(query)
	if query == "" || !strings.Contains(strings.ToLower(chunk), q) {
		return chunk
	}
	if activeOffset < 0 {
		return highlightAllWithStyle(chunk, query, searchMatchStyle)
	}
	var out strings.Builder
	cursor := 0
	lower := strings.ToLower(chunk)
	for {
		idx := strings.Index(lower[cursor:], q)
		if idx < 0 {
			out.WriteString(chunk[cursor:])
			break
		}
		start := cursor + idx
		out.WriteString(chunk[cursor:start])
		style := searchMatchStyle
		if start == activeOffset {
			style = activeSearchMatchStyle
		}
		out.WriteString(style.Render(chunk[start : start+len(query)]))
		cursor = start + len(query)
	}
	return out.String()
}

func highlightAllWithStyle(s, query string, style lipgloss.Style) string {
	q := strings.ToLower(query)
	lower := strings.ToLower(s)
	var out strings.Builder
	cursor := 0
	for {
		idx := strings.Index(lower[cursor:], q)
		if idx < 0 {
			out.WriteString(s[cursor:])
			break
		}
		start := cursor + idx
		out.WriteString(s[cursor:start])
		out.WriteString(style.Render(s[start : start+len(query)]))
		cursor = start + len(query)
	}
	return out.String()
}

func chunkString(s string, size int) []string {
	s = sanitizeDetailRow(s)
	if size < 1 {
		return []string{s}
	}
	if chunkDisplayWidth(s) <= size {
		return []string{s}
	}
	var out []string
	for s != "" {
		_, n := chunkBoundary(s, size)
		if n <= 0 {
			break
		}
		out = append(out, s[:n])
		s = s[n:]
	}
	if len(out) == 0 {
		return []string{s}
	}
	return out
}

func chunkBoundary(s string, size int) (width, bytes int) {
	w := 0
	for i := 0; i < len(s); {
		r, rn := utf8.DecodeRuneInString(s[i:])
		if r == utf8.RuneError && rn == 1 {
			rn = 1
		}
		rw := runewidth.RuneWidth(r)
		if w+rw > size && i > 0 {
			return w, i
		}
		w += rw
		i += rn
	}
	return w, len(s)
}

func chunkDisplayWidth(s string) int {
	w := 0
	for _, r := range s {
		w += runewidth.RuneWidth(r)
	}
	return w
}

func chunkPositionForByteOffset(s string, size, byteOffset int) (chunkIdx, offsetInChunk int) {
	chunks := chunkString(s, size)
	offset := 0
	for i, c := range chunks {
		end := offset + len(c)
		if byteOffset >= offset && byteOffset < end {
			return i, byteOffset - offset
		}
		offset = end
	}
	return -1, -1
}

// truncate walks runes once (O(n) vs O(n²) in naive impl).
func truncate(s string, n int) string {
	if n <= 3 {
		return s
	}
	if lipgloss.Width(s) <= n {
		return s
	}
	runes := []rune(s)
	budget := n - 1
	width := 0
	for i, r := range runes {
		w := runewidth.RuneWidth(r)
		if width+w > budget {
			return string(runes[:i]) + "…"
		}
		width += w
	}
	return s
}
