package tui

import (
	"strings"

	"github.com/bloodynite/lazyredis/internal/config"
)

const (
	actionAppHelp      = "app.help"
	actionAppForceQuit = "app.force_quit"
	actionHelpClose    = "help.close"
	actionSave         = "save"

	actionProfilesUp      = "profiles.up"
	actionProfilesDown    = "profiles.down"
	actionProfilesConnect = "profiles.connect"
	actionProfilesNew     = "profiles.new"
	actionProfilesEdit    = "profiles.edit"
	actionProfilesDelete  = "profiles.delete"
	actionProfilesQuit    = "profiles.quit"

	actionFormTab      = "form.tab"
	actionFormShiftTab = "form.shift_tab"
	actionFormEsc      = "form.esc"

	actionBrowserDisconnect   = "browser.disconnect"
	actionBrowserTab          = "browser.tab"
	actionBrowserUp           = "browser.up"
	actionBrowserDown         = "browser.down"
	actionBrowserFilter       = "browser.filter"
	actionBrowserNewKey       = "browser.new_key"
	actionBrowserRefresh      = "browser.refresh"
	actionBrowserAutoRefresh  = "browser.auto_refresh"
	actionBrowserFlush        = "browser.flush"
	actionBrowserMoreKeys     = "browser.more_keys"
	actionBrowserTTL          = "browser.ttl"
	actionBrowserDelete       = "browser.delete"
	actionBrowserEdit         = "browser.edit"
	actionBrowserDetailAdd    = "browser.detail_add"
	actionBrowserDetailEdit = "browser.detail_edit"
	actionBrowserDetailDelete = "browser.detail_delete"
	actionBrowserCopy         = "browser.copy"
	actionBrowserFilterApply  = "browser.filter_apply"
	actionBrowserFilterCancel = "browser.filter_cancel"

	actionEditEsc      = "edit.esc"
	actionEditTab      = "edit.tab"
	actionEditShiftTab = "edit.shift_tab"

	actionConfirmYes = "confirm.yes"
	actionConfirmNo  = "confirm.no"
)

type bindDef struct {
	id   string
	desc string
}

var defaultKeyMap = map[string][]string{
	actionAppHelp:      {"?"},
	actionAppForceQuit: {"ctrl+c"},
	actionHelpClose:    {"?", "esc"},

	actionProfilesUp:      {"k", "up"},
	actionProfilesDown:    {"j", "down"},
	actionProfilesConnect: {"enter"},
	actionProfilesNew:     {"n"},
	actionProfilesEdit:    {"e"},
	actionProfilesDelete:  {"d"},
	actionProfilesQuit:    {"q"},

	actionFormTab:      {"tab"},
	actionFormShiftTab: {"shift+tab"},
	actionFormEsc:      {"esc"},

	actionBrowserDisconnect:   {"q"},
	actionBrowserTab:          {"tab"},
	actionBrowserUp:           {"k", "up"},
	actionBrowserDown:         {"j", "down"},
	actionBrowserFilter:       {"/"},
	actionBrowserNewKey:       {"n"},
	actionBrowserRefresh:      {"r"},
	actionBrowserAutoRefresh:  {"a"},
	actionBrowserFlush:        {"ctrl+f"},
	actionBrowserMoreKeys:     {"g"},
	actionBrowserTTL:          {"t"},
	actionBrowserDelete:       {"d"},
	actionBrowserEdit:         {"e"},
	actionBrowserDetailAdd:    {"i"},
	actionBrowserDetailEdit:   {"e"},
	actionBrowserDetailDelete: {"d"},
	actionBrowserCopy:         {"c"},
	actionBrowserFilterApply:  {"enter"},
	actionBrowserFilterCancel: {"esc"},

	actionEditEsc:      {"esc"},
	actionEditTab:      {"tab"},
	actionEditShiftTab: {"shift+tab"},

	actionConfirmYes: {"y"},
	actionConfirmNo:  {"n", "esc"},
}

func normalizeKey(key string) string {
	key = strings.TrimSpace(strings.ToLower(key))
	key = strings.ReplaceAll(key, " ", "")
	return key
}

func keysFor(cfg *config.File, action string) []string {
	if action == actionSave {
		return saveBindKeys(cfg)
	}
	if keys, ok := defaultKeyMap[action]; ok {
		return keys
	}
	return nil
}

func saveBindKeys(cfg *config.File) []string {
	modifier := "ctrl"
	if cfg != nil && cfg.GetShortcutModifier() == "alt" {
		modifier = "alt"
	}
	return []string{modifier + "+s"}
}

func (m *Model) bindKeys(action string) []string {
	return keysFor(m.Config, action)
}

func (m *Model) matchAction(action, key string) bool {
	return matchAny(normalizeKey(key), m.bindKeys(action))
}

func matchAny(key string, keys []string) bool {
	for _, candidate := range keys {
		if key == candidate {
			return true
		}
	}
	return false
}

func formatBindKeys(keys []string) string {
	if len(keys) == 0 {
		return ""
	}
	seen := make(map[string]struct{}, len(keys))
	unique := make([]string, 0, len(keys))
	for _, key := range keys {
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		unique = append(unique, displayKey(key))
	}
	return strings.Join(unique, "/")
}

func displayKey(key string) string {
	switch key {
	case "up":
		return "↑"
	case "down":
		return "↓"
	default:
		return key
	}
}

func editableKeyType(keyType string) bool {
	switch keyType {
	case "string", "hash", "list", "set", "zset", "stream":
		return true
	default:
		return false
	}
}

func compositeKeyType(keyType string) bool {
	switch keyType {
	case "hash", "list", "set", "zset", "stream":
		return true
	default:
		return false
	}
}

func (m *Model) bindEntry(id, desc string) keyBind {
	return keyBind{
		Key:  formatBindKeys(m.bindKeys(id)),
		Desc: desc,
	}
}

func (m *Model) bindHint(action, label string) string {
	keys := formatBindKeys(m.bindKeys(action))
	if keys == "" {
		return label
	}
	return keys + " " + label
}

func (m *Model) saveCancelHint(saveAction string) string {
	return m.bindHint(saveAction, "save") + "   " + m.bindHint(actionEditEsc, "cancel")
}

func (m *Model) editEnterSaveCancelHint() string {
	return m.saveCancelHint(actionSave)
}

func (m *Model) editCtrlEnterSaveCancelHint() string {
	return m.saveCancelHint(actionSave)
}

func (m *Model) keyFormModalHint() string {
	if m.EditMode == editNewKey && m.NewKeyFocus == newKeyFieldType {
		return strings.Join([]string{
			m.bindHint(actionBrowserUp, "up"),
			m.bindHint(actionBrowserDown, "down"),
			m.bindHint(actionEditTab, "next"),
			m.bindHint(actionSave, "save"),
			m.bindHint(actionEditEsc, "cancel"),
		}, "   ")
	}
	return strings.Join([]string{
		m.bindHint(actionEditTab, "next"),
		m.bindHint(actionSave, "save"),
		m.bindHint(actionEditEsc, "cancel"),
	}, "   ")
}

func (m *Model) appendHelpBind(binds []keyBind) []keyBind {
	return append(binds, m.bindEntry(actionAppHelp, "help"))
}

func (m *Model) applicableHelpActions() []bindDef {
	switch {
	case m.Screen == ScreenProfileForm || (m.Screen == ScreenConfirm && m.PrevScreen == ScreenProfileForm):
		return []bindDef{
			{actionFormTab, "next field"},
			{actionFormShiftTab, "previous field"},
			{actionSave, "save"},
			{actionFormEsc, "cancel"},
		}
	case m.Screen == ScreenKeyEdit && (m.EditMode == editNewKey || m.EditMode == editExistingKey):
		defs := []bindDef{
			{actionEditTab, "next field"},
			{actionEditShiftTab, "previous field"},
			{actionSave, "save"},
			{actionEditEsc, "cancel"},
		}
		if m.EditMode == editNewKey && m.NewKeyFocus == newKeyFieldType {
			defs = append([]bindDef{
				{actionBrowserUp, "up"},
				{actionBrowserDown, "down"},
			}, defs...)
		}
		return defs
	case m.Screen == ScreenKeyEdit && (m.EditMode == editElement || m.EditMode == editElementAdd):
		if m.elementEditUsesTextarea() {
			return []bindDef{
				{actionSave, "save"},
				{actionEditEsc, "cancel"},
			}
		}
		return []bindDef{
			{actionSave, "save"},
			{actionEditEsc, "cancel"},
		}
	case m.Screen == ScreenKeyEdit:
		return []bindDef{
			{actionSave, "save"},
			{actionEditEsc, "cancel"},
		}
	case m.Screen == ScreenBrowser && m.SearchFocus:
		return []bindDef{
			{actionBrowserFilterCancel, "close filter"},
		}
	case m.Screen == ScreenBrowser:
		defs := []bindDef{
			{actionBrowserTab, "switch panel"},
			{actionBrowserUp, "up"},
			{actionBrowserDown, "down"},
			{actionBrowserFilter, "filter"},
			{actionBrowserNewKey, "new key"},
			{actionBrowserRefresh, "refresh"},
			{actionBrowserAutoRefresh, "auto refresh"},
			{actionBrowserDisconnect, "disconnect"},
			{actionBrowserFlush, "flush db"},
		}
		if m.ScanCursor != 0 {
			defs = append([]bindDef{{actionBrowserMoreKeys, "load more keys"}}, defs...)
		}
		if m.KeyDetail != nil && m.SelectedKey != "" {
			defs = append(defs, bindDef{actionBrowserCopy, "copy value"})
		}
		if m.PanelFocus == panelDetail && m.KeyDetail != nil {
			if compositeKeyType(m.KeyDetail.Meta.Type) {
				defs = append(defs,
					bindDef{actionBrowserDetailAdd, "add item"},
					bindDef{actionBrowserDetailEdit, "edit item"},
					bindDef{actionBrowserDetailDelete, "delete item"},
				)
			} else if m.KeyDetail.Meta.Type == "string" {
				defs = append(defs,
					bindDef{actionBrowserDetailEdit, "edit value"},
					bindDef{actionBrowserDelete, "delete key"},
				)
			}
			if m.SelectedKey != "" {
				defs = append(defs, bindDef{actionBrowserTTL, "ttl"})
			}
		} else if m.SelectedKey != "" {
			defs = append(defs,
				bindDef{actionBrowserTTL, "ttl"},
				bindDef{actionBrowserDelete, "delete key"},
			)
			if m.KeyDetail != nil && editableKeyType(m.KeyDetail.Meta.Type) {
				defs = append(defs, bindDef{actionBrowserEdit, "edit key"})
			}
		}
		return defs
	case m.Screen == ScreenConfirm:
		return []bindDef{
			{actionConfirmYes, "confirm"},
			{actionConfirmNo, "cancel"},
		}
	default:
		return []bindDef{
			{actionProfilesUp, "up"},
			{actionProfilesDown, "down"},
			{actionProfilesConnect, "connect"},
			{actionProfilesNew, "new"},
			{actionProfilesEdit, "edit"},
			{actionProfilesDelete, "delete"},
			{actionProfilesQuit, "quit"},
		}
	}
}
