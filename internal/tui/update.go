package tui

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	"github.com/aymanbagabas/go-osc52/v2"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/bloodynite/lazyredis/internal/config"
	"github.com/bloodynite/lazyredis/internal/store"
)

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height
		leftW, _ := m.browserPanelWidths()
		m.SearchInput.Width = max(4, leftW-6)
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
		return m.handleKeyPress(key)

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.Spinner, cmd = m.Spinner.Update(msg)
		return m, cmd

	case statusClearMsg:
		if msg.gen == m.statusClearGen && m.Status == copiedToClipboardStatus {
			m.Status = ""
		}
		return m, nil

	case profilesLoadedMsg:
		m.Loading = false
		if msg.err != nil {
			m.ErrMsg = msg.err.Error()
			return m, nil
		}
		if msg.cfg == nil {
			m.ErrMsg = "config not loaded"
			return m, nil
		}
		m.Config = msg.cfg
		m.Profiles = msg.cfg.Profiles
		if m.ProfileCursor >= len(m.Profiles) {
			m.ProfileCursor = max(0, len(m.Profiles)-1)
		}
		if m.Screen == ScreenProfileForm {
			m.Screen = ScreenProfiles
			m.FormEditing = false
			m.Status = "profile saved"
		}
		if m.Screen == ScreenConfirm && m.ConfirmAction == confirmDeleteProfile {
			m.Screen = ScreenProfiles
			m.ConfirmAction = confirmNone
			m.Status = "profile deleted"
		}
		return m, nil

	case connectedMsg:
		m.Loading = false
		if msg.err != nil {
			m.ErrMsg = msg.err.Error()
			return m, nil
		}
		m.Client = msg.client
		m.ErrMsg = ""
		m.Status = fmt.Sprintf("connected to %s", msg.client.Profile().Name)
		m.Screen = ScreenBrowser
		m.PanelFocus = panelKeys
		m.PrevScreen = ScreenProfiles
		m.scanGen = 1
		return m, tea.Batch(loadInfo(m.Client), scanKeys(m.Client, 0, m.ScanPattern, false, m.scanGen), m.Spinner.Tick, m.scheduleAutoRefreshCmd())

	case autoRefreshMsg:
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
			return m, nil
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
			return m, nil
		}
		m.ScanCursor = msg.cursor
		if msg.append {
			m.Keys = append(m.Keys, msg.keys...)
			return m, nil
		}
		m.Keys = msg.keys
		cursor := 0
		if m.SelectedKey != "" {
			for i, k := range m.Keys {
				if k == m.SelectedKey {
					cursor = i
					break
				}
			}
		}
		m.KeyCursor = cursor
		m.KeyScroll = 0
		m.adjustKeyScroll()
		if len(m.Keys) > 0 {
			m.SelectedKey = m.Keys[m.KeyCursor]
			m.Loading = true
			return m, loadKeyDetail(m.Client, m.SelectedKey)
		}
		m.SelectedKey = ""
		m.KeyDetail = nil
		return m, nil

	case keyDetailMsg:
		m.Loading = false
		if msg.err != nil {
			m.ErrMsg = msg.err.Error()
			return m, nil
		}
		m.KeyDetail = msg.detail
		m.DetailCursor = 0
		m.DetailScroll = 0
		return m, nil

	case actionDoneMsg:
		m.Loading = false
		if msg.err != nil {
			m.ErrMsg = msg.err.Error()
			return m, nil
		}
		m.ErrMsg = ""
		m.Status = msg.status
		if m.Screen == ScreenConfirm {
			switch m.ConfirmAction {
			case confirmDeleteKey:
				m.Screen = ScreenBrowser
				m.SelectedKey = ""
				m.KeyDetail = nil
				m.ConfirmAction = confirmNone
				return m, tea.Batch(loadInfo(m.Client), m.rescanKeysCmd())
			case confirmFlushDB:
				m.Screen = ScreenBrowser
				m.ConfirmAction = confirmNone
				m.KeyDetail = nil
				m.SelectedKey = ""
				return m, tea.Batch(loadInfo(m.Client), m.rescanKeysCmd())
			case confirmDeleteProfile:
				m.ConfirmAction = confirmNone
				return m, nil
			}
		}
		if m.Screen == ScreenKeyEdit {
			m.Screen = ScreenBrowser
			m.blurEditInputs()
			if m.EditMode == editRefreshInterval {
				return m, m.scheduleAutoRefreshCmd()
			}
			if (m.EditMode == editNewKey || m.EditMode == editExistingKey) && msg.reload && m.Client != nil {
				return m, tea.Batch(loadInfo(m.Client), m.rescanKeysCmd())
			}
			if msg.reload && m.Client != nil && m.SelectedKey != "" {
				return m, loadKeyDetail(m.Client, m.SelectedKey)
			}
		}
		if m.Screen == ScreenProfileForm {
			m.Screen = ScreenProfiles
			m.FormEditing = false
		}
		if msg.reload && m.Screen == ScreenBrowser && m.Client != nil && m.SelectedKey != "" {
			m.Loading = true
			return m, loadKeyDetail(m.Client, m.SelectedKey)
		}
		return m, nil
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
	case ScreenBrowser:
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
		if (m.EditMode == editElement || m.EditMode == editElementAdd) && m.elementEditUsesTextarea() {
			return m.updateElementTextareaInput(msg)
		}
		return m.updateEditInput(msg)
	case ScreenBrowser:
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

func (m *Model) handleKeyPress(key string) (tea.Model, tea.Cmd) {
	switch m.Screen {
	case ScreenProfiles:
		return m.handleProfilesKeys(key)
	case ScreenBrowser:
		return m.handleBrowserKeys(key)
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

func (m *Model) handleBrowserKeys(key string) (tea.Model, tea.Cmd) {
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
		m.detailMove(1)
	case m.matchAction(actionBrowserUp, key):
		if m.PanelFocus == panelKeys {
			return m.moveKeyCursor(-1)
		}
		m.detailMove(-1)
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
		return m, tea.Batch(append(m.refreshDataCmd(), m.scheduleAutoRefreshCmd())...)
	case m.matchAction(actionBrowserAutoRefresh, key):
		sec := config.DefaultRefreshIntervalSec
		if m.Config != nil {
			sec = m.Config.GetRefreshIntervalSec()
		}
		m.EditMode = editRefreshInterval
		m.EditInput.SetValue(strconv.Itoa(sec))
		m.EditInput.Placeholder = "seconds (0=off)"
		m.EditInput.Focus()
		m.PrevScreen = ScreenBrowser
		m.Screen = ScreenKeyEdit
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
	}
	return m, nil
}

func (m *Model) moveKeyCursor(delta int) (tea.Model, tea.Cmd) {
	if len(m.Keys) == 0 {
		return m, nil
	}
	next := clamp(m.KeyCursor+delta, 0, len(m.Keys)-1)
	if next == m.KeyCursor {
		return m, nil
	}
	m.KeyCursor = next
	m.adjustKeyScroll()
	m.PanelFocus = panelKeys
	m.SelectedKey = m.Keys[m.KeyCursor]
	m.Loading = true
	return m, loadKeyDetail(m.Client, m.SelectedKey)
}

func (m *Model) handleConfirmKeys(key string) (tea.Model, tea.Cmd) {
	switch {
	case m.matchAction(actionConfirmYes, key):
		m.Loading = true
		switch m.ConfirmAction {
		case confirmDeleteKey:
			return m, deleteKey(m.Client, m.ConfirmTarget)
		case confirmFlushDB:
			return m, flushDB(m.Client)
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
		return m, nil
	}
}

func (m *Model) updateEditInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	var cmd tea.Cmd
	m.EditInput, cmd = m.EditInput.Update(msg)
	if !m.matchAction(actionEditEnter, key) {
		return m, cmd
	}

	value := strings.TrimSpace(m.EditInput.Value())
	switch m.EditMode {
	case editRefreshInterval:
		sec, err := strconv.Atoi(value)
		if err != nil || sec < 0 {
			m.ErrMsg = "seconds must be >= 0"
			return m, cmd
		}
		m.Loading = true
		return m, saveRefreshInterval(m.Config, sec)
	case editElement, editElementAdd:
		return m.submitElementEdit()
	}
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
	if m.matchAction(actionEditEnter, key) {
		return m.submitTTLModal()
	}
	var cmd tea.Cmd
	m.NewKeyTTL, cmd = m.NewKeyTTL.Update(msg)
	return m, cmd
}

func (m *Model) submitTTLModal() (tea.Model, tea.Cmd) {
	ttl, err := store.ParseTTLInput(m.NewKeyTTL.Value())
	if err != nil {
		m.ErrMsg = err.Error()
		return m, nil
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
		return m, nil
	}
	ttl, err := store.ParseTTLInput(m.NewKeyTTL.Value())
	if err != nil {
		m.ErrMsg = err.Error()
		return m, nil
	}
	renameFrom := ""
	if m.EditMode == editExistingKey {
		renameFrom = m.SelectedKey
		body, err := store.ParseKeyBody(m.KeyFormType, m.NewKeyValue.Value())
		if err != nil {
			m.ErrMsg = err.Error()
			return m, nil
		}
		m.SelectedKey = key
		m.Loading = true
		return m, saveKeyBody(m.Client, key, m.KeyFormType, body, ttl, renameFrom)
	}
	keyType := m.KeyFormType
	body, err := store.ParseKeyBody(keyType, m.NewKeyValue.Value())
	if err != nil {
		m.ErrMsg = err.Error()
		return m, nil
	}
	m.KeyFormType = keyType
	m.SelectedKey = key
	m.Loading = true
	return m, saveKeyBody(m.Client, key, keyType, body, ttl, "")
}

func (m *Model) updateKeyFormInputs(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	if m.matchAction(actionEditCtrlEnter, key) {
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
	if m.matchAction(actionEditEnter, key) && m.NewKeyFocus == newKeyFieldTTL {
		next := m.nextKeyFormField(newKeyFieldTTL, 1)
		return m, m.focusNewKeyField(next)
	}
	if m.matchAction(actionEditEnter, key) && m.NewKeyFocus == newKeyFieldType {
		next := m.nextKeyFormField(newKeyFieldType, 1)
		return m, m.focusNewKeyField(next)
	}
	if m.matchAction(actionEditEnter, key) && m.NewKeyFocus == newKeyFieldKey {
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
	m.FormInputs[m.FormFocus], cmd = m.FormInputs[m.FormFocus].Update(msg)
	if !m.matchAction(actionFormEnter, key) {
		return m, cmd
	}

	p, err := profileFromForm(m.FormInputs)
	if err != nil {
		m.ErrMsg = err.Error()
		return m, cmd
	}
	if m.FormEditing && m.Config != nil {
		if existing, _ := m.Config.Find(m.FormOriginal); existing != nil {
			p = config.MergeProfile(*existing, p)
		}
	}
	m.Loading = true
	return m, saveProfile(m.Config, p)
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

func (m *Model) detailMove(delta int) {
	if m.KeyDetail == nil {
		return
	}
	max := detailItemCount(m.KeyDetail) - 1
	if max < 0 {
		return
	}
	m.DetailCursor = clamp(m.DetailCursor+delta, 0, max)
	m.adjustDetailScroll()
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

const (
	copiedToClipboardStatus = "copied to clipboard"
	copiedStatusDuration    = 3 * time.Second
)

func (m *Model) copyDetailValue() (tea.Model, tea.Cmd) {
	if m.KeyDetail == nil || m.SelectedKey == "" {
		return m, nil
	}
	text, ok := m.copyableValueText()
	if !ok {
		m.ErrMsg = "nothing to copy"
		return m, nil
	}
	if err := writeSystemClipboard(text); err != nil {
		m.ErrMsg = err.Error()
		return m, nil
	}
	m.ErrMsg = ""
	m.Status = copiedToClipboardStatus
	m.statusClearGen++
	gen := m.statusClearGen
	return m, clearStatusAfter(copiedStatusDuration, gen)
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
	return m.KeyFormType == "stream"
}

func (m *Model) openElementEdit(mode editMode) tea.Cmd {
	m.EditMode = mode
	m.PrevScreen = ScreenBrowser
	m.Screen = ScreenKeyEdit
	m.blurEditInputs()
	if m.elementEditUsesTextarea() {
		m.syncNewKeyLayout()
		return m.NewKeyValue.Focus()
	}
	return m.EditInput.Focus()
}

func (m *Model) startDetailEdit() (tea.Model, tea.Cmd) {
	if m.KeyDetail == nil || m.Client == nil {
		return m, nil
	}
	d := m.KeyDetail
	m.KeyFormType = d.Meta.Type
	switch d.Meta.Type {
	case "string":
		m.EditInput.SetValue(d.String)
		m.EditInput.Placeholder = "value"
		m.EditMode = editElement
		return m, m.openElementEdit(editElement)
	case "hash":
		if len(d.Hash) == 0 {
			m.ErrMsg = "no fields to edit"
			return m, nil
		}
		field := hashFields(d.Hash)[m.DetailCursor]
		m.EditField = field
		m.EditInput.SetValue(field + "=" + d.Hash[field])
		m.EditInput.Placeholder = "field=value"
		return m, m.openElementEdit(editElement)
	case "list":
		if len(d.List) == 0 {
			m.ErrMsg = "no items to edit"
			return m, nil
		}
		m.EditInput.SetValue(d.List[m.DetailCursor])
		m.EditInput.Placeholder = "item value"
		return m, m.openElementEdit(editElement)
	case "set":
		if len(d.Set) == 0 {
			m.ErrMsg = "no members to edit"
			return m, nil
		}
		m.EditField = d.Set[m.DetailCursor]
		m.EditInput.SetValue(m.EditField)
		m.EditInput.Placeholder = "member"
		return m, m.openElementEdit(editElement)
	case "zset":
		if len(d.ZSet) == 0 {
			m.ErrMsg = "no members to edit"
			return m, nil
		}
		z := d.ZSet[m.DetailCursor]
		member, _ := z.Member.(string)
		m.EditField = member
		m.EditInput.SetValue(store.FormatZSetLine(z.Score, member))
		m.EditInput.Placeholder = "score<TAB>member"
		return m, m.openElementEdit(editElement)
	case "stream":
		if len(d.Stream) == 0 {
			m.ErrMsg = "no entries to edit"
			return m, nil
		}
		entry := d.Stream[m.DetailCursor]
		m.EditField = entry.ID
		m.NewKeyValue.SetValue(store.EncodeStreamFields(entry.Fields))
		m.NewKeyValue.Placeholder = "field=value (one per line)"
		return m, m.openElementEdit(editElement)
	default:
		m.ErrMsg = fmt.Sprintf("type %s is not editable", d.Meta.Type)
		return m, nil
	}
}

func (m *Model) startDetailAdd() (tea.Model, tea.Cmd) {
	if m.KeyDetail == nil || m.Client == nil {
		return m, nil
	}
	m.KeyFormType = m.KeyDetail.Meta.Type
	m.EditField = ""
	m.EditInput.SetValue("")
	switch m.KeyDetail.Meta.Type {
	case "hash":
		m.EditInput.Placeholder = "field=value"
	case "list":
		m.EditInput.Placeholder = "item value"
	case "set":
		m.EditInput.Placeholder = "member"
	case "zset":
		m.EditInput.Placeholder = "score<TAB>member"
	case "stream":
		m.NewKeyValue.Reset()
		m.NewKeyValue.Placeholder = "field=value (one per line)"
		return m, m.openElementEdit(editElementAdd)
	default:
		return m, nil
	}
	return m, m.openElementEdit(editElementAdd)
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
	value := strings.TrimSpace(m.EditInput.Value())
	if m.elementEditUsesTextarea() {
		value = m.NewKeyValue.Value()
	}
	m.Loading = true
	key := m.SelectedKey
	switch m.KeyFormType {
	case "string":
		if m.EditMode == editElementAdd {
			return m, nil
		}
		return m, patchStringValue(m.Client, key, value)
	case "hash":
		if m.EditMode == editElementAdd {
			return m, addHashField(m.Client, key, value)
		}
		_, fieldValue, err := store.ParseHashFieldLine(value)
		if err != nil {
			m.ErrMsg = err.Error()
			m.Loading = false
			return m, nil
		}
		return m, patchHashField(m.Client, key, m.EditField, fieldValue)
	case "list":
		if m.EditMode == editElementAdd {
			return m, appendListItem(m.Client, key, value)
		}
		return m, patchListItem(m.Client, key, m.DetailCursor, value)
	case "set":
		if m.EditMode == editElementAdd {
			return m, addSetMember(m.Client, key, value)
		}
		return m, replaceSetMember(m.Client, key, m.EditField, value)
	case "zset":
		if m.EditMode == editElementAdd {
			return m, addZSetMember(m.Client, key, value)
		}
		return m, replaceZSetMember(m.Client, key, m.EditField, value)
	case "stream":
		if m.EditMode == editElementAdd {
			return m, addStreamEntry(m.Client, key, value)
		}
		return m, replaceStreamEntry(m.Client, key, m.EditField, value)
	default:
		m.Loading = false
		return m, nil
	}
}

func (m *Model) updateElementTextareaInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	if m.matchAction(actionEditCtrlEnter, key) {
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
	for i := 0; i < len(out); i++ {
		for j := i + 1; j < len(out); j++ {
			if out[j] < out[i] {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
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
	if m.Loading || m.SearchFocus || m.HelpOpen {
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
	return scheduleAutoRefresh(time.Duration(sec) * time.Second)
}

func (m *Model) rescanKeysCmd() tea.Cmd {
	if m.Client == nil {
		return nil
	}
	m.scanGen++
	return scanKeys(m.Client, 0, m.ScanPattern, false, m.scanGen)
}

func (m *Model) refreshDataCmd() []tea.Cmd {
	m.scanGen++
	gen := m.scanGen
	cmds := []tea.Cmd{
		loadInfo(m.Client),
		scanKeys(m.Client, 0, m.ScanPattern, false, gen),
	}
	if m.SelectedKey != "" {
		cmds = append(cmds, loadKeyDetail(m.Client, m.SelectedKey))
	}
	return cmds
}
