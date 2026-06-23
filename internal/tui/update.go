package tui

import (
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	"github.com/aymanbagabas/go-osc52/v2"
	"github.com/bloodynite/lazyredis/internal/config"
	"github.com/bloodynite/lazyredis/internal/store"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height
		leftW, _ := m.browserPanelWidths()
		m.SearchInput.Width = max(4, leftW-panelChromeCols-6)
		initStyles()
		if m.Screen == ScreenKeyEdit && (m.EditMode == editNewKey || m.EditMode == editExistingKey || m.EditMode == editTTL) {
			m.syncNewKeyLayout()
		}
		return m, nil

	case tea.KeyMsg:
		key := msg.String()
		if m.matchAction(actionAppForceQuit, key) {
			if m.Client != nil {
				_ = m.Client.Close()
			}
			return m, tea.Quit
		}
		if m.handleHelpToggle(key) {
			return m, nil
		}
		if m.HelpOpen {
			if m.matchAction(actionHelpClose, key) {
				m.HelpOpen = false
			}
			return m, nil
		}
		if m.inputFocused() {
			return m.handleInputKeys(msg)
		}
		return m.handleKeyPress(key, msg)

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.Spinner, cmd = m.Spinner.Update(msg)
		return m, cmd

	case statusClearMsg:
		if msg.gen == m.statusClearGen {
			m.Status = ""
			m.ErrMsg = ""
		}
		return m, nil

	case profilesLoadedMsg:
		m.Loading = false
		if msg.err != nil {
			m.ErrMsg = msg.err.Error()
			m.statusClearGen++
			gen := m.statusClearGen
			return m, clearStatusAfter(statusMessageDuration, gen)
		}
		if msg.cfg == nil {
			m.ErrMsg = "config not loaded"
			m.statusClearGen++
			gen := m.statusClearGen
			return m, clearStatusAfter(statusMessageDuration, gen)
		}
		m.Config = msg.cfg
		m.Profiles = msg.cfg.Profiles
		if m.ProfileCursor >= len(m.Profiles) {
			m.ProfileCursor = max(0, len(m.Profiles)-1)
		}
		if m.Screen == ScreenProfileForm {
			m.Screen = ScreenProfiles
			m.FormEditing = false
			return m, m.statusClearCmd("profile saved")
		}
		if m.Screen == ScreenConfirm && m.ConfirmAction == confirmDeleteProfile {
			m.Screen = ScreenProfiles
			m.ConfirmAction = confirmNone
			return m, m.statusClearCmd("profile deleted")
		}
		return m, nil

	case connectedMsg:
		m.Loading = false
		if msg.err != nil {
			m.ErrMsg = msg.err.Error()
			m.statusClearGen++
			gen := m.statusClearGen
			return m, clearStatusAfter(statusMessageDuration, gen)
		}
		m.Client = msg.client
		m.ErrMsg = ""
		m.Screen = ScreenBrowser
		m.PanelFocus = panelKeys
		m.PrevScreen = ScreenProfiles
		m.scanGen = 1
		m.refreshGen++
		m.detailGen = 0
		statusCmd := m.statusClearCmd(fmt.Sprintf("connected to %s", msg.client.Profile().Name))
		m.RefreshStartedAt = time.Now()
		return m, tea.Batch(loadInfo(m.Client), scanKeys(m.Client, 0, m.ScanPattern, false, m.scanGen), m.Spinner.Tick, m.scheduleAutoRefreshCmd(), statusCmd)

	case autoRefreshMsg:
		if msg.gen != 0 && msg.gen != m.refreshGen {
			return m, m.scheduleAutoRefreshAt(msg.gen)
		}
		m.RefreshStartedAt = time.Now()
		cmds := []tea.Cmd{m.scheduleAutoRefreshCmd()}
		if m.canAutoRefresh() {
			m.Loading = true
			cmds = append(cmds, m.refreshDataCmd()...)
		}
		return m, tea.Batch(cmds...)

	case infoLoadedMsg:
		m.Loading = false
		if msg.err != nil {
			m.ErrMsg = msg.err.Error()
			m.statusClearGen++
			gen := m.statusClearGen
			return m, clearStatusAfter(statusMessageDuration, gen)
		}
		m.Info = msg.info
		return m, nil

	case keysLoadedMsg:
		if msg.gen != m.scanGen {
			return m, nil
		}
		m.Loading = false
		if msg.err != nil {
			m.ErrMsg = msg.err.Error()
			m.statusClearGen++
			gen := m.statusClearGen
			return m, clearStatusAfter(statusMessageDuration, gen)
		}
		m.ScanCursor = msg.cursor
		if msg.append {
			m.Keys = append(m.Keys, msg.keys...)
			m.sortKeys()
			return m, nil
		}
		m.Keys = msg.keys
		m.sortKeys()
		cursor := 0
		foundSelected := false
		if m.SelectedNodePath != "" {
			for i, node := range m.VisibleNodes {
				if node.fullPath == m.SelectedNodePath {
					cursor = i
					foundSelected = true
					break
				}
			}
		}
		if !foundSelected {
			if m.KeyCursor >= len(m.VisibleNodes) {
				cursor = max(0, len(m.VisibleNodes)-1)
			} else {
				cursor = m.KeyCursor
			}
		}
		m.KeyCursor = cursor
		m.adjustKeyScroll()
		if len(m.VisibleNodes) > 0 {
			node := m.VisibleNodes[m.KeyCursor]
			m.SelectedNodePath = node.fullPath
			if !node.isFolder {
				m.SelectedKey = node.fullPath
				m.detailGen++
				m.detailRetryCount = 0
				m.DetailTotal = -1
				m.DetailLoaded = 0
				m.Loading = true
				return m, loadKeySummaryFn(m.Client, m.SelectedKey, m.detailGen)
			}
		}
		m.SelectedKey = ""
		m.KeyDetail = nil
		return m, nil

	case keySummaryMsg:
		if msg.gen != m.detailGen || msg.key != m.SelectedKey {
			return m, nil
		}
		if msg.err != nil {
			m.Loading = false
			return m, m.handleDetailError(msg.err.Error())
		}
		m.Loading = false
		if m.detailRetryCount > 0 {
			m.ErrMsg = ""
		}
		m.DetailTotal = msg.summary.Total
		if msg.summary.Meta.Type == "string" ||
			msg.summary.Total <= 0 ||
			int(msg.summary.Total) <= detailChunkSize {
			m.DetailLoaded = int(detailTotalOrZero(m.DetailTotal, msg.summary.Meta.Type))
			m.detailChunkPending = false
			m.Loading = true
			return m, loadKeyDetailFn(m.Client, msg.summary.Meta.Key, -1, 0, m.detailGen, false)
		}
		m.DetailLoaded = 0
		m.detailChunkPending = false
		m.Loading = true
		return m, loadKeyDetailFn(m.Client, msg.summary.Meta.Key, 0, detailChunkSize, m.detailGen, false)

	case keyDetailMsg:
		if msg.gen != m.detailGen || msg.key != m.SelectedKey {
			return m, nil
		}
		if msg.err != nil {
			m.Loading = false
			m.detailChunkPending = false
			return m, m.handleDetailError(msg.err.Error())
		}
		m.Loading = false
		m.detailChunkPending = false
		if m.detailRetryCount > 0 {
			m.ErrMsg = ""
		}
		if !msg.chunk {
			prevCursor := m.DetailSearchCursor
			sameKey := m.KeyDetail != nil &&
				m.KeyDetail.Meta.Key == msg.detail.Meta.Key
			m.KeyDetail = msg.detail
			m.DetailCursor = 0
			m.DetailScroll = 0
			m.DetailSearchFocus = false
			m.DetailSearchInput.Blur()
			m.DetailLoaded = compositeLoadedCount(msg.detail)
			if m.DetailSearchInput.Value() != "" {
				m.applyDetailSearch(prevCursor, sameKey)
			}
			return m, nil
		}
		mergeChunkIntoDetail(m.KeyDetail, msg.detail, msg.appendOff)
		m.DetailLoaded = compositeLoadedCount(m.KeyDetail)
		return m, nil

	case detailDebounceMsg:
		if msg.gen != m.detailGen || msg.key != m.SelectedKey {
			return m, nil
		}
		if m.Client == nil {
			return m, nil
		}
		m.Loading = true
		return m, loadKeySummaryFn(m.Client, msg.key, msg.gen)

	case actionDoneMsg:
		m.Loading = false
		if msg.err != nil {
			m.ErrMsg = msg.err.Error()
			m.statusClearGen++
			gen := m.statusClearGen
			return m, clearStatusAfter(statusMessageDuration, gen)
		}
		m.ErrMsg = ""
		statusCmd := m.statusClearCmd(msg.status)
		if m.Screen == ScreenConfirm {
			switch m.ConfirmAction {
			case confirmDeleteKey:
				m.Screen = ScreenBrowser
				m.SelectedKey = ""
				m.KeyDetail = nil
				m.ConfirmAction = confirmNone
				return m, tea.Batch(loadInfo(m.Client), m.rescanKeysCmd(), statusCmd)
			case confirmFlushDB:
				m.Screen = ScreenBrowser
				m.ConfirmAction = confirmNone
				m.KeyDetail = nil
				m.SelectedKey = ""
				return m, tea.Batch(loadInfo(m.Client), m.rescanKeysCmd(), statusCmd)
			case confirmDeleteProfile:
				m.ConfirmAction = confirmNone
				return m, statusCmd
			}
		}
		if m.Screen == ScreenKeyEdit {
			m.Screen = ScreenBrowser
			m.blurEditInputs()
			if m.EditMode == editRefreshInterval {
				m.RefreshStartedAt = time.Now()
				cmds := []tea.Cmd{}
				if m.Client != nil {
					m.Loading = true
					cmds = append(cmds, m.refreshDataCmd()...)
				}
				if sched := m.scheduleAutoRefreshCmd(); sched != nil {
					cmds = append(cmds, sched)
				}
				cmds = append(cmds, statusCmd)
				return m, tea.Batch(cmds...)
			}
			if (m.EditMode == editNewKey || m.EditMode == editExistingKey) && msg.reload && m.Client != nil {
				return m, tea.Batch(loadInfo(m.Client), m.rescanKeysCmd(), statusCmd)
			}
			if msg.reload && m.Client != nil && m.SelectedKey != "" {
				m.detailGen++
				m.detailRetryCount = 0
				m.DetailTotal = -1
				m.DetailLoaded = 0
				return m, tea.Batch(loadKeySummaryFn(m.Client, m.SelectedKey, m.detailGen), statusCmd)
			}
			return m, statusCmd
		}
		if m.Screen == ScreenProfileForm {
			m.Screen = ScreenProfiles
			m.FormEditing = false
		}
		if msg.reload && m.Screen == ScreenBrowser && m.Client != nil && m.SelectedKey != "" {
			m.Loading = true
			m.detailGen++
			m.detailRetryCount = 0
			m.DetailTotal = -1
			m.DetailLoaded = 0
			return m, tea.Batch(loadKeySummaryFn(m.Client, m.SelectedKey, m.detailGen), statusCmd)
		}
		return m, statusCmd
	}

	return m, nil
}

func (m *Model) inputFocused() bool {
	if m.Screen == ScreenProfileForm {
		return true
	}
	if m.Screen == ScreenKeyEdit {
		return true
	}
	if m.Screen == ScreenBrowser && m.SearchFocus {
		return true
	}
	if m.Screen == ScreenBrowser && m.DetailSearchFocus {
		return true
	}
	if m.Screen == ScreenConfirm && m.ConfirmAction == confirmFlushDB {
		return true
	}
	return false
}

func (m *Model) handleHelpToggle(key string) bool {
	if m.matchAction(actionAppHelp, key) {
		m.HelpOpen = !m.HelpOpen
		return true
	}
	return false
}

func (m *Model) handleInputKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	switch m.Screen {
	case ScreenProfileForm:
		if m.matchAction(actionFormEsc, key) {
			m.Screen = ScreenProfiles
			m.FormEditing = false
			return m, nil
		}
	case ScreenKeyEdit:
		if m.matchAction(actionEditEsc, key) {
			m.Screen = ScreenBrowser
			if m.Client == nil {
				m.Screen = ScreenProfiles
			}
			m.blurEditInputs()
			return m, nil
		}
	case ScreenConfirm:
		if m.ConfirmAction == confirmFlushDB && m.matchAction(actionConfirmNo, key) {
			m.Screen = m.PrevScreen
			m.ConfirmAction = confirmNone
			return m, nil
		}
	case ScreenBrowser:
		if m.DetailSearchFocus && m.matchAction(actionBrowserFilterCancel, key) {
			m.DetailSearchFocus = false
			m.DetailSearchInput.Blur()
			return m, nil
		}
		if m.SearchFocus && m.matchAction(actionBrowserFilterCancel, key) {
			m.SearchFocus = false
			m.SearchInput.Blur()
			return m, nil
		}
	}

	switch m.Screen {
	case ScreenProfileForm:
		return m.updateFormInputs(msg)
	case ScreenKeyEdit:
		if m.EditMode == editNewKey || m.EditMode == editExistingKey {
			return m.updateKeyFormInputs(msg)
		}
		if m.EditMode == editTTL {
			return m.updateTTLModalInputs(msg)
		}
		if m.EditMode == editRefreshInterval {
			return m.updateRefreshIntervalModal(msg)
		}
		if (m.EditMode == editElement || m.EditMode == editElementAdd) && m.elementEditUsesTextarea() {
			return m.updateElementTextareaInput(msg)
		}
		return m.updateEditInput(msg)
	case ScreenConfirm:
		if m.ConfirmAction == confirmFlushDB {
			var cmd tea.Cmd
			m.ConfirmInput, cmd = m.ConfirmInput.Update(msg)
			if m.matchAction(actionBrowserFilterApply, key) {
				profileName := m.Client.Profile().Name
				if m.ConfirmInput.Value() == profileName {
					m.Loading = true
					m.Screen = ScreenBrowser
					m.ConfirmAction = confirmNone
					return m, tea.Batch(cmd, flushDB(m.Client))
				}
				m.ErrMsg = "profile name does not match"
				m.statusClearGen++
				gen := m.statusClearGen
				return m, tea.Batch(cmd, clearStatusAfter(statusMessageDuration, gen))
			}
			return m, cmd
		}
	case ScreenBrowser:
		if m.DetailSearchFocus {
			var cmd tea.Cmd
			m.DetailSearchInput, cmd = m.DetailSearchInput.Update(msg)
			if m.matchAction(actionBrowserFilterApply, key) {
				m.DetailSearchFocus = false
				m.DetailSearchInput.Blur()
				count := m.applyDetailSearch(0, false)
				msg := detailSearchStatus(count)
				m.Status = msg
				m.statusClearGen++
				gen := m.statusClearGen
				return m, tea.Batch(cmd, clearStatusAfter(statusMessageDuration, gen))
			}
			return m, cmd
		}
		var cmd tea.Cmd
		m.SearchInput, cmd = m.SearchInput.Update(msg)
		if m.matchAction(actionBrowserFilterApply, key) {
			next, scan := m.applySearchPattern()
			next.SearchFocus = false
			next.SearchInput.Blur()
			if scan != nil {
				return next, tea.Batch(cmd, scan)
			}
			return next, cmd
		}
		next, scan := m.applySearchPattern()
		if scan != nil {
			return next, tea.Batch(cmd, scan)
		}
		return next, cmd
	}
	return m, nil
}

func (m *Model) applySearchPattern() (*Model, tea.Cmd) {
	pattern := store.NormalizeScanPattern(m.SearchInput.Value())
	if pattern == m.ScanPattern {
		return m, nil
	}
	m.ScanPattern = pattern
	m.ScanCursor = 0
	m.Loading = true
	m.scanGen++
	return m, scanKeys(m.Client, 0, m.ScanPattern, false, m.scanGen)
}

func (m *Model) applyDetailSearch(prevCursor int, preserveCursor bool) int {
	if m.KeyDetail == nil {
		m.DetailSearchMatches = nil
		m.DetailSearchCursor = -1
		return 0
	}
	query := m.DetailSearchInput.Value()
	if query == "" {
		m.DetailSearchMatches = nil
		m.DetailSearchCursor = -1
		return 0
	}
	matches, count := computeDetailSearchMatches(m.KeyDetail, query)
	m.DetailSearchMatches = matches
	if len(matches) > 0 {
		target := 0
		switch {
		case preserveCursor && prevCursor >= 0 && prevCursor < len(matches):
			target = prevCursor
		case preserveCursor && prevCursor >= len(matches):
			target = len(matches) - 1
		}
		m.DetailSearchCursor = target
		m.jumpToDetailSearchMatch(target)
	} else {
		m.DetailSearchCursor = -1
	}
	return count
}

func computeDetailSearchMatches(d *store.KeyDetail, query string) (matches []int, count int) {
	if query == "" {
		return nil, 0
	}
	q := strings.ToLower(query)
	switch d.Meta.Type {
	case "string":
		haystack := strings.ToLower(sanitizeDetailRow(d.String))
		count = strings.Count(haystack, q)
		if count == 0 {
			return nil, 0
		}
		matches = make([]int, 0, count)
		cursor := 0
		for {
			idx := strings.Index(haystack[cursor:], q)
			if idx < 0 {
				break
			}
			start := cursor + idx
			matches = append(matches, start)
			cursor = start + len(q)
		}
		return matches, count
	case "hash":
		fields := hashFields(d.Hash)
		for i, f := range fields {
			if strings.Contains(strings.ToLower(f), q) || strings.Contains(strings.ToLower(d.Hash[f]), q) {
				count++
				matches = append(matches, i)
			}
		}
	case "list":
		for i, v := range d.List {
			if strings.Contains(strings.ToLower(v), q) {
				count++
				matches = append(matches, i)
			}
		}
	case "set":
		for i, v := range d.Set {
			if strings.Contains(strings.ToLower(v), q) {
				count++
				matches = append(matches, i)
			}
		}
	case "zset":
		for i, z := range d.ZSet {
			member, _ := z.Member.(string)
			if strings.Contains(strings.ToLower(member), q) ||
				strings.Contains(strings.ToLower(strconv.FormatFloat(z.Score, 'f', -1, 64)), q) {
				count++
				matches = append(matches, i)
			}
		}
	case "stream":
		for i, e := range d.Stream {
			if strings.Contains(strings.ToLower(e.ID), q) || strings.Contains(strings.ToLower(formatStreamEntry(e)), q) {
				count++
				matches = append(matches, i)
			}
		}
	}
	return matches, count
}

func (m *Model) jumpToDetailSearchMatch(idxInMatches int) {
	if m.KeyDetail == nil || idxInMatches < 0 || idxInMatches >= len(m.DetailSearchMatches) {
		return
	}
	d := m.KeyDetail
	pos := m.DetailSearchMatches[idxInMatches]
	if d.Meta.Type == "string" {
		_, rightW := m.browserPanelWidths()
		panelW := rightW - panelChromeCols
		if panelW < 1 {
			panelW = 1
		}
		visible := max(1, m.browserContentHeight()-4)
		maxW := max(8, panelW-4)
		chunkIdx, _ := chunkPositionForByteOffset(d.String, maxW, pos)
		bodyVisible := max(1, visible-1)
		limit := stringDetailScrollLimit(d.String, panelW, visible)
		target := chunkIdx - bodyVisible/2
		if target < 0 {
			target = 0
		}
		if target > limit {
			target = limit
		}
		m.DetailScroll = target
		return
	}
	m.DetailCursor = pos
	m.adjustDetailScroll()
}

func (m *Model) cycleDetailMatch(delta int) {
	n := len(m.DetailSearchMatches)
	if n == 0 {
		return
	}
	next := (m.DetailSearchCursor + delta) % n
	if next < 0 {
		next += n
	}
	m.DetailSearchCursor = next
	m.jumpToDetailSearchMatch(next)
}

func (m *Model) detailSearchNavStatus() string {
	n := len(m.DetailSearchMatches)
	if n == 0 {
		return "no matches"
	}
	return fmt.Sprintf("match %d/%d", m.DetailSearchCursor+1, n)
}

func detailSearchStatus(count int) string {
	switch count {
	case 0:
		return "no matches"
	case 1:
		return "1 match"
	default:
		return fmt.Sprintf("%d matches", count)
	}
}

func (m *Model) handleKeyPress(key string, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.Screen {
	case ScreenProfiles:
		return m.handleProfilesKeys(key)
	case ScreenBrowser:
		return m.handleBrowserKeys(key, msg)
	case ScreenKeyEdit:
		return m, nil
	case ScreenConfirm:
		return m.handleConfirmKeys(key)
	}
	return m, nil
}

func (m *Model) handleProfilesKeys(key string) (tea.Model, tea.Cmd) {
	switch {
	case m.matchAction(actionProfilesQuit, key):
		return m, tea.Quit
	case m.matchAction(actionProfilesDown, key):
		if len(m.Profiles) > 0 && m.ProfileCursor < len(m.Profiles)-1 {
			m.ProfileCursor++
		}
	case m.matchAction(actionProfilesUp, key):
		if m.ProfileCursor > 0 {
			m.ProfileCursor--
		}
	case m.matchAction(actionProfilesConnect, key):
		if len(m.Profiles) == 0 {
			return m, nil
		}
		m.Loading = true
		m.ErrMsg = ""
		return m, connectProfile(m.Profiles[m.ProfileCursor])
	case m.matchAction(actionProfilesNew, key):
		m.resetForm("")
		m.Screen = ScreenProfileForm
		m.FormEditing = false
	case m.matchAction(actionProfilesEdit, key):
		if len(m.Profiles) == 0 {
			return m, nil
		}
		m.resetForm(m.Profiles[m.ProfileCursor].Name)
		m.Screen = ScreenProfileForm
		m.FormEditing = true
	case m.matchAction(actionProfilesDelete, key):
		if len(m.Profiles) == 0 {
			return m, nil
		}
		m.ConfirmAction = confirmDeleteProfile
		m.ConfirmTarget = m.Profiles[m.ProfileCursor].Name
		m.PrevScreen = ScreenProfiles
		m.Screen = ScreenConfirm
	}
	return m, nil
}

func (m *Model) handleBrowserKeys(key string, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if key == "/" && !m.SearchFocus && !m.DetailSearchFocus &&
		m.PanelFocus == panelDetail && m.KeyDetail != nil {
		m.DetailSearchFocus = true
		m.DetailSearchInput.SetValue("")
		m.DetailSearchInput.Focus()
		return m, nil
	}
	if m.PanelFocus == panelDetail && m.KeyDetail != nil &&
		!m.DetailSearchFocus && m.DetailSearchInput.Value() != "" &&
		len(m.DetailSearchMatches) > 0 &&
		msg.Type == tea.KeyRunes && len(msg.Runes) == 1 {
		switch msg.Runes[0] {
		case 'n':
			m.cycleDetailMatch(1)
			return m, m.statusClearCmd(m.detailSearchNavStatus())
		case 'N':
			m.cycleDetailMatch(-1)
			return m, m.statusClearCmd(m.detailSearchNavStatus())
		}
	}
	if m.PanelFocus == panelKeys && !m.SearchFocus {
		switch key {
		case "enter":
			if len(m.VisibleNodes) > 0 {
				node := m.VisibleNodes[m.KeyCursor]
				m.SelectedNodePath = node.fullPath
				if node.isFolder {
					m.toggleFolder(m.KeyCursor)
					m.adjustKeyScroll()
					return m, nil
				}
			}
		case "right":
			if len(m.VisibleNodes) > 0 {
				node := m.VisibleNodes[m.KeyCursor]
				m.SelectedNodePath = node.fullPath
				if node.isFolder && !m.ExpandedFolders[node.fullPath] {
					m.toggleFolder(m.KeyCursor)
					m.adjustKeyScroll()
					return m, nil
				}
			}
		case "left":
			if len(m.VisibleNodes) > 0 {
				node := m.VisibleNodes[m.KeyCursor]
				m.SelectedNodePath = node.fullPath
				if node.isFolder && m.ExpandedFolders[node.fullPath] {
					m.toggleFolder(m.KeyCursor)
					m.adjustKeyScroll()
					return m, nil
				}
				if node.depth > 0 {
					for i := m.KeyCursor - 1; i >= 0; i-- {
						if m.VisibleNodes[i].depth < node.depth {
							m.KeyCursor = i
							m.SelectedNodePath = m.VisibleNodes[i].fullPath
							m.adjustKeyScroll()
							return m, nil
						}
					}
				}
			}
		}
	}
	switch {
	case m.matchAction(actionBrowserDisconnect, key):
		if m.Client != nil {
			_ = m.Client.Close()
		}
		m.Client = nil
		m.KeyDetail = nil
		m.Keys = nil
		m.Screen = ScreenProfiles
		return m, nil
	case m.matchAction(actionBrowserTab, key):
		if m.PanelFocus == panelKeys {
			m.PanelFocus = panelDetail
		} else {
			m.PanelFocus = panelKeys
		}
	case m.matchAction(actionBrowserDown, key):
		if m.PanelFocus == panelKeys {
			return m.moveKeyCursor(1)
		}
		return m.detailMove(1)
	case m.matchAction(actionBrowserUp, key):
		if m.PanelFocus == panelKeys {
			return m.moveKeyCursor(-1)
		}
		return m.detailMove(-1)
	case m.matchAction(actionBrowserFilter, key):
		m.PanelFocus = panelKeys
		m.SearchFocus = true
		if m.ScanPattern != "*" {
			m.SearchInput.SetValue(m.ScanPattern)
		} else {
			m.SearchInput.SetValue("")
		}
		m.SearchInput.Focus()
	case m.matchAction(actionBrowserNewKey, key):
		m.openKeyFormModal(true)
		return m, m.focusNewKeyField(newKeyFieldTTL)
	case m.matchAction(actionBrowserMoreKeys, key):
		if m.ScanCursor != 0 {
			m.Loading = true
			return m, scanKeys(m.Client, m.ScanCursor, m.ScanPattern, true, m.scanGen)
		}
	case m.matchAction(actionBrowserRefresh, key):
		m.Loading = true
		return m, tea.Batch(m.refreshDataCmd()...)
	case m.matchAction(actionBrowserAutoRefresh, key):
		sec := config.DefaultRefreshIntervalSec
		if m.Config != nil {
			sec = m.Config.GetRefreshIntervalSec()
		}
		m.EditMode = editRefreshInterval
		m.RefreshIntervalCursor = refreshIntervalCursor(sec)
		m.blurEditInputs()
		m.PrevScreen = ScreenBrowser
		m.Screen = ScreenKeyEdit
	case m.matchAction(actionBrowserSortOrder, key):
		m.SortOrder = (m.SortOrder + 1) % 3
		m.sortKeys()
		return m, m.statusClearCmd(m.sortOrderLabel())
	case m.matchAction(actionBrowserEdit, key) || m.matchAction(actionBrowserDetailEdit, key):
		if m.PanelFocus == panelDetail && m.KeyDetail != nil {
			return m.startDetailEdit()
		}
		return m.startEdit()
	case m.matchAction(actionBrowserDetailAdd, key):
		if m.PanelFocus == panelDetail && m.KeyDetail != nil && compositeKeyType(m.KeyDetail.Meta.Type) {
			return m.startDetailAdd()
		}
	case m.matchAction(actionBrowserDelete, key):
		if m.PanelFocus == panelDetail && m.KeyDetail != nil && compositeKeyType(m.KeyDetail.Meta.Type) {
			return m.deleteDetailElement()
		}
		if m.SelectedKey == "" {
			return m, nil
		}
		m.ConfirmAction = confirmDeleteKey
		m.ConfirmTarget = m.SelectedKey
		m.PrevScreen = ScreenBrowser
		m.Screen = ScreenConfirm
	case m.matchAction(actionBrowserDetailDelete, key):
		if m.PanelFocus == panelDetail && m.KeyDetail != nil && compositeKeyType(m.KeyDetail.Meta.Type) {
			return m.deleteDetailElement()
		}
	case m.matchAction(actionBrowserCopy, key):
		return m.copyDetailValue()
	case m.matchAction(actionBrowserTTL, key):
		if m.SelectedKey == "" {
			return m, nil
		}
		return m.openTTLModal()
	case m.matchAction(actionBrowserFlush, key):
		m.ConfirmAction = confirmFlushDB
		m.ConfirmTarget = ""
		m.PrevScreen = ScreenBrowser
		m.Screen = ScreenConfirm
		m.ConfirmInput.SetValue("")
		m.ConfirmInput.Focus()
		return m, textinput.Blink
	}
	return m, nil
}

func (m *Model) moveKeyCursor(delta int) (tea.Model, tea.Cmd) {
	count := len(m.VisibleNodes)
	if count == 0 {
		count = len(m.Keys)
	}
	if count == 0 {
		return m, nil
	}
	next := clamp(m.KeyCursor+delta, 0, count-1)
	if next == m.KeyCursor {
		return m, nil
	}
	m.KeyCursor = next
	m.adjustKeyScroll()
	m.PanelFocus = panelKeys
	if len(m.VisibleNodes) > 0 {
		node := m.VisibleNodes[m.KeyCursor]
		m.SelectedNodePath = node.fullPath
		if node.isFolder {
			return m, nil
		}
		m.SelectedKey = node.fullPath
	} else {
		m.SelectedKey = m.Keys[m.KeyCursor]
		m.SelectedNodePath = m.SelectedKey
	}
	m.detailGen++
	m.detailRetryCount = 0
	m.Loading = true
	return m, scheduleDetailDebounce(m.SelectedKey, m.detailGen)
}

func (m *Model) handleConfirmKeys(key string) (tea.Model, tea.Cmd) {
	switch {
	case m.matchAction(actionConfirmYes, key):
		if m.ConfirmAction == confirmFlushDB {
			return m, nil
		}
		m.Loading = true
		switch m.ConfirmAction {
		case confirmDeleteKey:
			return m, deleteKey(m.Client, m.ConfirmTarget)
		case confirmDeleteProfile:
			return m, deleteProfile(m.Config, m.ConfirmTarget)
		}
	case m.matchAction(actionConfirmNo, key):
		m.Screen = m.PrevScreen
		m.ConfirmAction = confirmNone
	}
	return m, nil
}

func (m *Model) startEdit() (tea.Model, tea.Cmd) {
	if m.KeyDetail == nil {
		return m, nil
	}
	switch m.KeyDetail.Meta.Type {
	case "string", "hash", "list", "set", "zset", "stream":
		m.openKeyFormModal(false)
		return m, m.focusNewKeyField(newKeyFieldTTL)
	default:
		m.ErrMsg = fmt.Sprintf("type %s is not editable", m.KeyDetail.Meta.Type)
		m.statusClearGen++
		gen := m.statusClearGen
		return m, clearStatusAfter(statusMessageDuration, gen)
	}
}

func (m *Model) updateEditInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	if m.matchAction(actionSave, key) {
		value := strings.TrimSpace(m.EditInput.Value())
		switch m.EditMode {
		case editRefreshInterval:
			sec, err := strconv.Atoi(value)
			if err != nil || sec < 0 || (sec > 0 && sec < 5) {
				m.ErrMsg = "seconds must be 0 (off) or >= 5"
				m.statusClearGen++
				gen := m.statusClearGen
				return m, clearStatusAfter(statusMessageDuration, gen)
			}
			m.Loading = true
			return m, saveRefreshInterval(m.Config, sec)
		case editElement, editElementAdd:
			return m.submitElementEdit()
		}
	}

	var cmd tea.Cmd
	m.EditInput, cmd = m.EditInput.Update(msg)
	return m, cmd
}

func ttlInputValue(ttl time.Duration) string {
	switch {
	case ttl < 0:
		return "persist"
	case ttl == 0:
		return "0"
	default:
		if ttl%time.Second == 0 {
			return strconv.Itoa(int(ttl / time.Second))
		}
		return ttl.Round(time.Second).String()
	}
}

func (m *Model) setKeyFormType(keyType string) {
	m.KeyFormType = store.NormalizeKeyType(keyType)
	m.NewKeyTypeCursor = 0
	for i, t := range keyFormTypes {
		if t == m.KeyFormType {
			m.NewKeyTypeCursor = i
			break
		}
	}
	m.NewKeyValue.Placeholder = keyFormValuePlaceholder(m.KeyFormType)
}

func (m *Model) keyFormFieldOrder() []int {
	if m.EditMode == editExistingKey {
		return []int{newKeyFieldTTL, newKeyFieldKey, newKeyFieldValue}
	}
	return []int{newKeyFieldTTL, newKeyFieldType, newKeyFieldKey, newKeyFieldValue}
}

func (m *Model) nextKeyFormField(current, delta int) int {
	order := m.keyFormFieldOrder()
	idx := 0
	for i, field := range order {
		if field == current {
			idx = i
			break
		}
	}
	idx = (idx + delta + len(order)) % len(order)
	return order[idx]
}

func (m *Model) moveKeyFormType(delta int) {
	if m.EditMode != editNewKey {
		return
	}
	m.NewKeyTypeCursor = (m.NewKeyTypeCursor + delta + len(keyFormTypes)) % len(keyFormTypes)
	m.KeyFormType = keyFormTypes[m.NewKeyTypeCursor]
	m.NewKeyValue.Placeholder = keyFormValuePlaceholder(m.KeyFormType)
}

func (m *Model) openTTLModal() (tea.Model, tea.Cmd) {
	if m.KeyDetail == nil || m.SelectedKey == "" {
		return m, nil
	}
	m.EditMode = editTTL
	m.NewKeyTTL.SetValue(ttlInputValue(m.KeyDetail.Meta.TTL))
	m.syncNewKeyLayout()
	m.blurEditInputs()
	m.PrevScreen = ScreenBrowser
	m.Screen = ScreenKeyEdit
	m.NewKeyFocus = newKeyFieldTTL
	return m, m.NewKeyTTL.Focus()
}

func (m *Model) updateTTLModalInputs(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	if m.matchAction(actionSave, key) {
		return m.submitTTLModal()
	}
	var cmd tea.Cmd
	m.NewKeyTTL, cmd = m.NewKeyTTL.Update(msg)
	return m, cmd
}

func (m *Model) updateRefreshIntervalModal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	switch {
	case m.matchAction(actionSave, key) || key == "enter":
		return m.submitRefreshInterval()
	case m.matchAction(actionBrowserUp, key):
		m.RefreshIntervalCursor = (m.RefreshIntervalCursor - 1 + len(refreshIntervalChoices)) % len(refreshIntervalChoices)
		return m, nil
	case m.matchAction(actionBrowserDown, key):
		m.RefreshIntervalCursor = (m.RefreshIntervalCursor + 1) % len(refreshIntervalChoices)
		return m, nil
	}
	return m, nil
}

func (m *Model) submitRefreshInterval() (tea.Model, tea.Cmd) {
	sec := refreshIntervalChoices[m.RefreshIntervalCursor]
	m.Loading = true
	return m, saveRefreshInterval(m.Config, sec)
}

func (m *Model) submitTTLModal() (tea.Model, tea.Cmd) {
	ttl, err := store.ParseTTLInput(m.NewKeyTTL.Value())
	if err != nil {
		m.ErrMsg = err.Error()
		m.statusClearGen++
		gen := m.statusClearGen
		return m, clearStatusAfter(statusMessageDuration, gen)
	}
	m.Loading = true
	return m, setTTL(m.Client, m.SelectedKey, ttl)
}

func (m *Model) openKeyFormModal(isNew bool) {
	if isNew {
		m.EditMode = editNewKey
		m.setKeyFormType("string")
		m.NewKeyTTL.SetValue("")
		m.NewKeyName.SetValue("")
		m.NewKeyValue.Reset()
		m.NewKeyFocus = newKeyFieldTTL
	} else {
		m.EditMode = editExistingKey
		m.setKeyFormType(m.KeyDetail.Meta.Type)
		m.NewKeyTTL.SetValue(ttlInputValue(m.KeyDetail.Meta.TTL))
		m.NewKeyName.SetValue(m.SelectedKey)
		m.NewKeyValue.SetValue(store.EncodeKeyBody(m.KeyDetail))
		m.NewKeyFocus = newKeyFieldTTL
	}
	m.NewKeyValue.Placeholder = keyFormValuePlaceholder(m.KeyFormType)
	configureNewKeyTextarea(&m.NewKeyValue)
	m.syncNewKeyLayout()
	m.blurEditInputs()
	m.PrevScreen = ScreenBrowser
	m.Screen = ScreenKeyEdit
}

func keyFormValuePlaceholder(keyType string) string {
	switch keyType {
	case "hash":
		return "field=value (one per line)"
	case "list":
		return "one item per line"
	case "set":
		return "one member per line"
	case "zset":
		return "score<TAB>member per line"
	case "stream":
		return "id\\tfield=value per line (* for auto id)"
	default:
		return "value"
	}
}

func (m *Model) blurEditInputs() {
	m.EditInput.Blur()
	m.NewKeyTTL.Blur()
	m.NewKeyName.Blur()
	m.NewKeyValue.Blur()
}

func (m *Model) focusNewKeyField(field int) tea.Cmd {
	m.NewKeyTTL.Blur()
	m.NewKeyName.Blur()
	m.NewKeyValue.Blur()
	m.NewKeyFocus = field
	switch field {
	case newKeyFieldTTL:
		return m.NewKeyTTL.Focus()
	case newKeyFieldType:
		return nil
	case newKeyFieldKey:
		return m.NewKeyName.Focus()
	case newKeyFieldValue:
		return m.NewKeyValue.Focus()
	}
	return nil
}

func (m *Model) submitKeyForm() (tea.Model, tea.Cmd) {
	key := strings.TrimSpace(m.NewKeyName.Value())
	if key == "" {
		m.ErrMsg = "key required"
		m.statusClearGen++
		gen := m.statusClearGen
		return m, clearStatusAfter(statusMessageDuration, gen)
	}
	ttl, err := store.ParseTTLInput(m.NewKeyTTL.Value())
	if err != nil {
		m.ErrMsg = err.Error()
		m.statusClearGen++
		gen := m.statusClearGen
		return m, clearStatusAfter(statusMessageDuration, gen)
	}
	renameFrom := ""
	if m.EditMode == editExistingKey {
		renameFrom = m.SelectedKey
		body, err := store.ParseKeyBody(m.KeyFormType, m.NewKeyValue.Value())
		if err != nil {
			m.ErrMsg = err.Error()
			m.statusClearGen++
			gen := m.statusClearGen
			return m, clearStatusAfter(statusMessageDuration, gen)
		}
		m.SelectedKey = key
		m.Loading = true
		return m, saveKeyBody(m.Client, key, m.KeyFormType, body, ttl, renameFrom)
	}
	keyType := m.KeyFormType
	body, err := store.ParseKeyBody(keyType, m.NewKeyValue.Value())
	if err != nil {
		m.ErrMsg = err.Error()
		m.statusClearGen++
		gen := m.statusClearGen
		return m, clearStatusAfter(statusMessageDuration, gen)
	}
	m.KeyFormType = keyType
	m.SelectedKey = key
	m.Loading = true
	return m, saveKeyBody(m.Client, key, keyType, body, ttl, "")
}

func (m *Model) updateKeyFormInputs(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	if m.matchAction(actionSave, key) {
		return m.submitKeyForm()
	}
	if m.matchAction(actionEditTab, key) || m.matchAction(actionEditShiftTab, key) {
		delta := 1
		if m.matchAction(actionEditShiftTab, key) {
			delta = -1
		}
		next := m.nextKeyFormField(m.NewKeyFocus, delta)
		return m, m.focusNewKeyField(next)
	}
	if m.NewKeyFocus == newKeyFieldType && m.EditMode == editNewKey {
		switch key {
		case "j", "down":
			m.moveKeyFormType(1)
			return m, nil
		case "k", "up":
			m.moveKeyFormType(-1)
			return m, nil
		}
	}
	if key == "enter" && m.NewKeyFocus == newKeyFieldTTL {
		next := m.nextKeyFormField(newKeyFieldTTL, 1)
		return m, m.focusNewKeyField(next)
	}
	if key == "enter" && m.NewKeyFocus == newKeyFieldType {
		next := m.nextKeyFormField(newKeyFieldType, 1)
		return m, m.focusNewKeyField(next)
	}
	if key == "enter" && m.NewKeyFocus == newKeyFieldKey {
		return m, m.focusNewKeyField(newKeyFieldValue)
	}

	var cmd tea.Cmd
	switch m.NewKeyFocus {
	case newKeyFieldTTL:
		m.NewKeyTTL, cmd = m.NewKeyTTL.Update(msg)
	case newKeyFieldKey:
		m.NewKeyName, cmd = m.NewKeyName.Update(msg)
	case newKeyFieldValue:
		m.NewKeyValue, cmd = m.NewKeyValue.Update(msg)
	}
	return m, cmd
}

func (m *Model) updateFormInputs(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	if m.matchAction(actionFormTab, key) || m.matchAction(actionFormShiftTab, key) {
		m.FormInputs[m.FormFocus].Blur()
		if m.matchAction(actionFormTab, key) {
			m.FormFocus = (m.FormFocus + 1) % len(m.FormInputs)
		} else {
			m.FormFocus = (m.FormFocus - 1 + len(m.FormInputs)) % len(m.FormInputs)
		}
		m.FormInputs[m.FormFocus].Focus()
		return m, nil
	}

	var cmd tea.Cmd
	if m.matchAction(actionSave, key) {
		p, err := profileFromForm(m.FormInputs)
		if err != nil {
			m.ErrMsg = err.Error()
			m.statusClearGen++
			gen := m.statusClearGen
			return m, clearStatusAfter(statusMessageDuration, gen)
		}
		if m.FormEditing && m.Config != nil {
			if existing, _ := m.Config.Find(m.FormOriginal); existing != nil {
				p = config.MergeProfile(*existing, p)
			}
		}
		m.Loading = true
		return m, saveProfile(m.Config, p)
	}

	m.FormInputs[m.FormFocus], cmd = m.FormInputs[m.FormFocus].Update(msg)
	return m, cmd
}

func (m *Model) resetForm(name string) {
	values := []string{"", "127.0.0.1:6379", "", "0", "standalone", "", "", "", "off", "", "", ""}
	if name != "" && m.Config != nil {
		if p, _ := m.Config.Find(name); p != nil {
			values = profileToFormValues(*p)
		}
	}
	for i := range m.FormInputs {
		m.FormInputs[i].SetValue(values[i])
	}
	m.FormFocus = 0
	for i := range m.FormInputs {
		m.FormInputs[i].Blur()
	}
	m.FormInputs[0].Focus()
	m.FormOriginal = name
	m.FormEditing = name != ""
}

func (m *Model) adjustKeyScroll() {
	visible := max(1, m.browserContentHeight()-3)
	if m.KeyCursor < m.KeyScroll {
		m.KeyScroll = m.KeyCursor
	}
	if m.KeyCursor >= m.KeyScroll+visible {
		m.KeyScroll = m.KeyCursor - visible + 1
	}
}

func (m *Model) detailMove(delta int) (tea.Model, tea.Cmd) {
	if m.KeyDetail == nil {
		return m, nil
	}
	if m.KeyDetail.Meta.Type == "string" {
		_, rightW := m.browserPanelWidths()
		panelW := rightW - panelChromeCols
		if panelW < 1 {
			panelW = 1
		}
		visible := max(1, m.browserContentHeight()-4)
		limit := stringDetailScrollLimit(m.KeyDetail.String, panelW, visible)
		m.DetailScroll = clamp(m.DetailScroll+delta, 0, limit)
		return m, nil
	}
	max := detailItemCount(m.KeyDetail) - 1
	if max < 0 {
		return m, nil
	}
	m.DetailCursor = clamp(m.DetailCursor+delta, 0, max)
	m.adjustDetailScroll()
	return m, m.maybeLoadMoreDetail()
}

func (m *Model) adjustDetailScroll() {
	visible := max(1, m.browserContentHeight()-4)
	if m.DetailCursor < m.DetailScroll {
		m.DetailScroll = m.DetailCursor
	}
	if m.DetailCursor >= m.DetailScroll+visible {
		m.DetailScroll = m.DetailCursor - visible + 1
	}
}

func detailItemCount(d *store.KeyDetail) int {
	switch d.Meta.Type {
	case "hash":
		return len(d.Hash)
	case "list":
		return len(d.List)
	case "set":
		return len(d.Set)
	case "zset":
		return len(d.ZSet)
	case "stream":
		return len(d.Stream)
	default:
		return 1
	}
}

func detailTotalOrZero(total int64, keyType string) int64 {
	if keyType == "string" || keyType == "" {
		return 0
	}
	if total < 0 {
		return 0
	}
	return total
}

func compositeLoadedCount(d *store.KeyDetail) int {
	if d == nil {
		return 0
	}
	return detailItemCount(d)
}

func mergeChunkIntoDetail(dst, src *store.KeyDetail, offset int) {
	if dst == nil || src == nil {
		return
	}
	switch dst.Meta.Type {
	case "hash":
		if dst.Hash == nil {
			dst.Hash = make(map[string]string, len(src.Hash))
		}
		for k, v := range src.Hash {
			dst.Hash[k] = v
		}
	case "list":
		dst.List = append(dst.List, src.List...)
	case "set":
		dst.Set = append(dst.Set, src.Set...)
	case "zset":
		dst.ZSet = append(dst.ZSet, src.ZSet...)
	case "stream":
		dst.Stream = append(dst.Stream, src.Stream...)
	}
}

func (m *Model) handleDetailError(errMsg string) tea.Cmd {
	if m.Client == nil || m.SelectedKey == "" {
		m.ErrMsg = errMsg
		m.statusClearGen++
		gen := m.statusClearGen
		return clearStatusAfter(statusMessageDuration, gen)
	}
	if m.detailRetryCount >= 1 || !looksLikeRetriableDetailError(errMsg) {
		m.ErrMsg = errMsg
		m.detailRetryCount = 0
		m.statusClearGen++
		gen := m.statusClearGen
		return clearStatusAfter(statusMessageDuration, gen)
	}
	m.detailRetryCount++
	m.ErrMsg = errMsg + " (retrying)"
	m.statusClearGen++
	gen := m.statusClearGen
	m.detailGen++
	m.detailChunkPending = false
	m.DetailTotal = -1
	m.DetailLoaded = 0
	m.Loading = true
	return tea.Batch(loadKeySummaryFn(m.Client, m.SelectedKey, m.detailGen), clearStatusAfter(statusMessageDuration, gen))
}

func looksLikeRetriableDetailError(msg string) bool {
	return strings.Contains(msg, "WRONGTYPE") ||
		strings.Contains(msg, "LOADING")
}

func (m *Model) maybeLoadMoreDetail() tea.Cmd {
	if m.Client == nil || m.KeyDetail == nil {
		return nil
	}
	if m.detailChunkPending {
		return nil
	}
	if !compositeKeyType(m.KeyDetail.Meta.Type) {
		return nil
	}
	loaded := compositeLoadedCount(m.KeyDetail)
	if int64(loaded) >= m.DetailTotal {
		return nil
	}
	if m.DetailCursor < loaded-detailChunkLookahead {
		return nil
	}
	m.detailChunkPending = true
	m.Loading = true
	return loadKeyDetailFn(m.Client, m.SelectedKey, loaded, detailChunkSize, m.detailGen, true)
}

func stringDetailScrollLimit(value string, panelW, listH int) int {
	if listH < 1 {
		listH = 1
	}
	maxW := max(8, panelW-4)
	chunks := chunkString(value, maxW)
	bodyVisible := max(1, listH-1)
	limit := len(chunks) - bodyVisible
	if limit < 0 {
		return 0
	}
	return limit
}

const (
	copiedToClipboardStatus = "copied to clipboard"
	statusMessageDuration   = 3 * time.Second
	detailDebounceDuration  = 80 * time.Millisecond
	detailChunkSize         = 200
	detailChunkLookahead    = 50
)

func (m *Model) statusClearCmd(msg string) tea.Cmd {
	if msg == "" {
		return nil
	}
	m.Status = msg
	m.statusClearGen++
	gen := m.statusClearGen
	return clearStatusAfter(statusMessageDuration, gen)
}

func (m *Model) copyDetailValue() (tea.Model, tea.Cmd) {
	if m.KeyDetail == nil || m.SelectedKey == "" {
		return m, nil
	}
	text, ok := m.copyableValueText()
	if !ok {
		m.ErrMsg = "nothing to copy"
		m.statusClearGen++
		gen := m.statusClearGen
		return m, clearStatusAfter(statusMessageDuration, gen)
	}
	if err := writeSystemClipboard(text); err != nil {
		m.ErrMsg = err.Error()
		m.statusClearGen++
		gen := m.statusClearGen
		return m, clearStatusAfter(statusMessageDuration, gen)
	}
	m.ErrMsg = ""
	return m, m.statusClearCmd(copiedToClipboardStatus)
}

func writeSystemClipboard(text string) error {
	if !clipboard.Unsupported {
		if err := clipboard.WriteAll(text); err == nil {
			return nil
		}
	}
	if err := writeClipboardWlCopy(text); err == nil {
		return nil
	}
	if err := writeClipboardXClip(text); err == nil {
		return nil
	}
	return writeClipboardOSC52(text)
}

func writeClipboardWlCopy(text string) error {
	path, err := exec.LookPath("wl-copy")
	if err != nil {
		return err
	}
	cmd := exec.Command(path)
	cmd.Stdin = strings.NewReader(text)
	return cmd.Run()
}

func writeClipboardXClip(text string) error {
	path, err := exec.LookPath("xclip")
	if err != nil {
		return err
	}
	cmd := exec.Command(path, "-selection", "clipboard")
	cmd.Stdin = strings.NewReader(text)
	return cmd.Run()
}

func writeClipboardOSC52(text string) error {
	seq := osc52.New(text)
	if os.Getenv("TMUX") != "" {
		seq = seq.Tmux()
	}
	_, err := fmt.Fprint(os.Stderr, seq)
	return err
}

func (m *Model) copyableValueText() (string, bool) {
	d := m.KeyDetail
	switch d.Meta.Type {
	case "string":
		return d.String, true
	case "hash":
		if len(d.Hash) == 0 {
			return "", false
		}
		field := hashFields(d.Hash)[m.DetailCursor]
		return d.Hash[field], true
	case "list":
		if len(d.List) == 0 {
			return "", false
		}
		return d.List[m.DetailCursor], true
	case "set":
		if len(d.Set) == 0 {
			return "", false
		}
		return d.Set[m.DetailCursor], true
	case "zset":
		if len(d.ZSet) == 0 {
			return "", false
		}
		member, _ := d.ZSet[m.DetailCursor].Member.(string)
		return member, true
	case "stream":
		if len(d.Stream) == 0 {
			return "", false
		}
		return store.EncodeStreamFields(d.Stream[m.DetailCursor].Fields), true
	default:
		return "", false
	}
}

func (m *Model) elementEditUsesTextarea() bool {
	return m.EditMode == editElement || m.EditMode == editElementAdd
}

func (m *Model) openElementEdit(mode editMode) tea.Cmd {
	m.EditMode = mode
	m.PrevScreen = ScreenBrowser
	m.Screen = ScreenKeyEdit
	m.blurEditInputs()
	m.syncNewKeyLayout()
	return m.NewKeyValue.Focus()
}

func (m *Model) startDetailEdit() (tea.Model, tea.Cmd) {
	if m.KeyDetail == nil || m.Client == nil {
		return m, nil
	}
	d := m.KeyDetail
	m.KeyFormType = d.Meta.Type
	m.NewKeyValue.Reset()
	switch d.Meta.Type {
	case "string":
		m.NewKeyValue.SetValue(d.String)
		m.NewKeyValue.Placeholder = "value"
		return m, m.openElementEdit(editElement)
	case "hash":
		if len(d.Hash) == 0 {
			m.ErrMsg = "no fields to edit"
			m.statusClearGen++
			gen := m.statusClearGen
			return m, clearStatusAfter(statusMessageDuration, gen)
		}
		field := hashFields(d.Hash)[m.DetailCursor]
		m.EditField = field
		m.NewKeyValue.SetValue(d.Hash[field])
		m.NewKeyValue.Placeholder = "value (field: " + field + ")"
		return m, m.openElementEdit(editElement)
	case "list":
		if len(d.List) == 0 {
			m.ErrMsg = "no items to edit"
			m.statusClearGen++
			gen := m.statusClearGen
			return m, clearStatusAfter(statusMessageDuration, gen)
		}
		m.NewKeyValue.SetValue(d.List[m.DetailCursor])
		m.NewKeyValue.Placeholder = "item value"
		return m, m.openElementEdit(editElement)
	case "set":
		if len(d.Set) == 0 {
			m.ErrMsg = "no members to edit"
			m.statusClearGen++
			gen := m.statusClearGen
			return m, clearStatusAfter(statusMessageDuration, gen)
		}
		member := d.Set[m.DetailCursor]
		m.EditField = member
		m.NewKeyValue.SetValue(member)
		m.NewKeyValue.Placeholder = "member"
		return m, m.openElementEdit(editElement)
	case "zset":
		if len(d.ZSet) == 0 {
			m.ErrMsg = "no members to edit"
			m.statusClearGen++
			gen := m.statusClearGen
			return m, clearStatusAfter(statusMessageDuration, gen)
		}
		z := d.ZSet[m.DetailCursor]
		member, _ := z.Member.(string)
		m.EditField = member
		m.NewKeyValue.SetValue(strconv.FormatFloat(z.Score, 'g', -1, 64) + " " + member)
		m.NewKeyValue.Placeholder = "score<TAB>member"
		return m, m.openElementEdit(editElement)
	case "stream":
		if len(d.Stream) == 0 {
			m.ErrMsg = "no entries to edit"
			m.statusClearGen++
			gen := m.statusClearGen
			return m, clearStatusAfter(statusMessageDuration, gen)
		}
		entry := d.Stream[m.DetailCursor]
		m.EditField = entry.ID
		m.NewKeyValue.SetValue(store.EncodeStreamFields(entry.Fields))
		m.NewKeyValue.Placeholder = "field=value (one per line)"
		return m, m.openElementEdit(editElement)
	default:
		m.ErrMsg = fmt.Sprintf("type %s is not editable", d.Meta.Type)
		m.statusClearGen++
		gen := m.statusClearGen
		return m, clearStatusAfter(statusMessageDuration, gen)
	}
}

func (m *Model) startDetailAdd() (tea.Model, tea.Cmd) {
	if m.KeyDetail == nil || m.Client == nil {
		return m, nil
	}
	m.KeyFormType = m.KeyDetail.Meta.Type
	m.EditField = ""
	m.NewKeyValue.Reset()
	switch m.KeyDetail.Meta.Type {
	case "hash":
		m.NewKeyValue.Placeholder = "field=value"
		return m, m.openElementEdit(editElementAdd)
	case "list":
		m.NewKeyValue.Placeholder = "item value"
		return m, m.openElementEdit(editElementAdd)
	case "set":
		m.NewKeyValue.Placeholder = "member"
		return m, m.openElementEdit(editElementAdd)
	case "zset":
		m.NewKeyValue.Placeholder = "score<TAB>member"
		return m, m.openElementEdit(editElementAdd)
	case "stream":
		m.NewKeyValue.Placeholder = "field=value (one per line)"
		return m, m.openElementEdit(editElementAdd)
	default:
		return m, nil
	}
}

func (m *Model) deleteDetailElement() (tea.Model, tea.Cmd) {
	if m.KeyDetail == nil || m.Client == nil {
		return m, nil
	}
	d := m.KeyDetail
	m.Loading = true
	switch d.Meta.Type {
	case "hash":
		if len(d.Hash) == 0 {
			m.Loading = false
			return m, nil
		}
		field := hashFields(d.Hash)[m.DetailCursor]
		return m, removeHashField(m.Client, m.SelectedKey, field)
	case "list":
		if len(d.List) == 0 {
			m.Loading = false
			return m, nil
		}
		return m, removeListItem(m.Client, m.SelectedKey, m.DetailCursor)
	case "set":
		if len(d.Set) == 0 {
			m.Loading = false
			return m, nil
		}
		return m, removeSetMember(m.Client, m.SelectedKey, d.Set[m.DetailCursor])
	case "zset":
		if len(d.ZSet) == 0 {
			m.Loading = false
			return m, nil
		}
		member, _ := d.ZSet[m.DetailCursor].Member.(string)
		return m, removeZSetMember(m.Client, m.SelectedKey, member)
	case "stream":
		if len(d.Stream) == 0 {
			m.Loading = false
			return m, nil
		}
		return m, removeStreamEntry(m.Client, m.SelectedKey, d.Stream[m.DetailCursor].ID)
	default:
		m.Loading = false
		return m, nil
	}
}

func (m *Model) submitElementEdit() (tea.Model, tea.Cmd) {
	if m.Client == nil || m.KeyDetail == nil {
		return m, nil
	}
	value := m.NewKeyValue.Value()
	m.Loading = true
	key := m.SelectedKey
	switch m.KeyFormType {
	case "string":
		if m.EditMode == editElementAdd {
			return m, nil
		}
		return m, patchStringValueFn(m.Client, key, value)
	case "hash":
		if m.EditMode == editElementAdd {
			return m, addHashFieldFn(m.Client, key, value)
		}
		return m, patchHashFieldFn(m.Client, key, m.EditField, value)
	case "list":
		if m.EditMode == editElementAdd {
			return m, appendListItemFn(m.Client, key, value)
		}
		return m, patchListItemFn(m.Client, key, m.DetailCursor, value)
	case "set":
		if m.EditMode == editElementAdd {
			return m, addSetMemberFn(m.Client, key, value)
		}
		return m, replaceSetMemberFn(m.Client, key, m.EditField, value)
	case "zset":
		if m.EditMode == editElementAdd {
			return m, addZSetMemberFn(m.Client, key, value)
		}
		return m, replaceZSetMemberFn(m.Client, key, m.EditField, value)
	case "stream":
		if m.EditMode == editElementAdd {
			return m, addStreamEntryFn(m.Client, key, value)
		}
		return m, replaceStreamEntryFn(m.Client, key, m.EditField, value)
	default:
		m.Loading = false
		return m, nil
	}
}

func (m *Model) updateElementTextareaInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	if m.matchAction(actionSave, key) {
		return m.submitElementEdit()
	}
	var cmd tea.Cmd
	m.NewKeyValue, cmd = m.NewKeyValue.Update(msg)
	return m, cmd
}

func hashFields(h map[string]string) []string {
	out := make([]string, 0, len(h))
	for k := range h {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (m *Model) canAutoRefresh() bool {
	if m.Client == nil || m.Config == nil {
		return false
	}
	if m.Config.GetRefreshIntervalSec() <= 0 {
		return false
	}
	if m.Screen != ScreenBrowser {
		return false
	}
	if m.Loading || m.SearchFocus || m.DetailSearchFocus || m.HelpOpen {
		return false
	}
	return true
}

func (m *Model) scheduleAutoRefreshCmd() tea.Cmd {
	if m.Client == nil {
		return nil
	}
	sec := config.DefaultRefreshIntervalSec
	if m.Config != nil {
		sec = m.Config.GetRefreshIntervalSec()
	}
	if sec <= 0 {
		return nil
	}
	m.refreshGen++
	return scheduleAutoRefresh(time.Duration(sec)*time.Second, m.refreshGen)
}

func (m *Model) scheduleAutoRefreshAt(gen uint64) tea.Cmd {
	if m.Client == nil {
		return nil
	}
	sec := config.DefaultRefreshIntervalSec
	if m.Config != nil {
		sec = m.Config.GetRefreshIntervalSec()
	}
	if sec <= 0 {
		return nil
	}
	return scheduleAutoRefresh(time.Duration(sec)*time.Second, gen)
}

func (m *Model) rescanKeysCmd() tea.Cmd {
	if m.Client == nil {
		return nil
	}
	m.scanGen++
	m.RefreshStartedAt = time.Now()
	return scanKeys(m.Client, 0, m.ScanPattern, false, m.scanGen)
}

func (m *Model) refreshDataCmd() []tea.Cmd {
	m.scanGen++
	gen := m.scanGen
	m.detailGen++
	m.detailRetryCount = 0
	detailGen := m.detailGen
	cmds := []tea.Cmd{
		loadInfo(m.Client),
		scanKeys(m.Client, 0, m.ScanPattern, false, gen),
	}
	if m.SelectedKey != "" {
		cmds = append(cmds, loadKeySummaryFn(m.Client, m.SelectedKey, detailGen))
	}
	return cmds
}

func (m *Model) sortKeys() {
	switch m.SortOrder {
	case sortAZ:
		sort.Slice(m.Keys, func(i, j int) bool {
			return strings.ToLower(m.Keys[i]) < strings.ToLower(m.Keys[j])
		})
	case sortZA:
		sort.Slice(m.Keys, func(i, j int) bool {
			return strings.ToLower(m.Keys[i]) > strings.ToLower(m.Keys[j])
		})
	}
	m.TreeRoot = buildKeyTree(m.Keys, m.SortOrder)
	m.VisibleNodes = flattenTree(m.TreeRoot, m.ExpandedFolders, 0)
}

func (m *Model) rebuildTree() {
	m.TreeRoot = buildKeyTree(m.Keys, m.SortOrder)
	m.VisibleNodes = flattenTree(m.TreeRoot, m.ExpandedFolders, 0)
}

func (m *Model) toggleFolder(index int) {
	if index < 0 || index >= len(m.VisibleNodes) {
		return
	}
	node := m.VisibleNodes[index]
	if !node.isFolder {
		return
	}
	m.ExpandedFolders[node.fullPath] = !m.ExpandedFolders[node.fullPath]
	m.VisibleNodes = flattenTree(m.TreeRoot, m.ExpandedFolders, 0)
}

func (m *Model) sortOrderLabel() string {
	switch m.SortOrder {
	case sortAZ:
		return "sort: A→Z"
	case sortZA:
		return "sort: Z→A"
	default:
		return "sort: original"
	}
}

func (m *Model) sortOrderIndicator() string {
	switch m.SortOrder {
	case sortZA:
		return "Z→A"
	default:
		return ""
	}
}
