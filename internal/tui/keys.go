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

	actionBrowserDisconnect         = "browser.disconnect"
	actionBrowserTab                = "browser.tab"
	actionBrowserUp                 = "browser.up"
	actionBrowserDown               = "browser.down"
	actionBrowserFilter             = "browser.filter"
	actionBrowserNewKey             = "browser.new_key"
	actionBrowserRefresh            = "browser.refresh"
	actionBrowserAutoRefresh        = "browser.auto_refresh"
	actionBrowserFlush              = "browser.flush"
	actionBrowserMoreKeys           = "browser.more_keys"
	actionBrowserTTL                = "browser.ttl"
	actionBrowserDelete             = "browser.delete"
	actionBrowserEdit               = "browser.edit"
	actionBrowserDetailAdd          = "browser.detail_add"
	actionBrowserDetailEdit         = "browser.detail_edit"
	actionBrowserDetailDelete       = "browser.detail_delete"
	actionBrowserCopy               = "browser.copy"
	actionBrowserFilterApply        = "browser.filter_apply"
	actionBrowserFilterCancel       = "browser.filter_cancel"
	actionBrowserDetailSearchNext   = "browser.detail_search_next"
	actionBrowserDetailSearchPrev   = "browser.detail_search_prev"

	actionEditEsc      = "edit.esc"
	actionEditTab      = "edit.tab"
	actionEditShiftTab = "edit.shift_tab"

	actionConfirmYes = "confirm.yes"
	actionConfirmNo  = "confirm.no"
)

// bindScope labels a binding's logical context so the keybar can pick the
// right subset for the current focus and the help modal can group bindings
// under headings.
type bindScope int

const (
	// scopeGlobal applies regardless of screen or focus (help, force quit).
	scopeGlobal bindScope = iota
	// scopeProfiles: profiles list screen.
	scopeProfiles
	// scopeProfileForm: profile form (create / edit profile).
	scopeProfileForm
	// scopeBrowserCommon: bindings that work anywhere in the browser screen
	// regardless of panel focus (disconnect, switch panel, scroll, flush).
	scopeBrowserCommon
	// scopeBrowserKeys: bindings that only apply when the keys panel is
	// focused (filter, new key, refresh, auto refresh, load more, plus the
	// selected-key ops that route through the keys panel).
	scopeBrowserKeys
	// scopeBrowserDetail: bindings that only apply when the detail panel is
	// focused (search value, add/edit/delete items, edit value, copy, ttl).
	scopeBrowserDetail
	// scopeKeyFilter: key filter input is focused.
	scopeKeyFilter
	// scopeDetailSearch: detail search input is focused.
	scopeDetailSearch
	// scopeKeyEdit: key editor screen.
	scopeKeyEdit
	// scopeConfirm: confirm modal.
	scopeConfirm
	// scopeHelp: help modal close key.
	scopeHelp
)

type bindDef struct {
	id    string
	desc  string
	scope bindScope
}

// helpGroup is a heading + the bindings displayed under it in the help modal.
type helpGroup struct {
	Title string
	Defs  []bindDef
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

	actionBrowserDisconnect:       {"q"},
	actionBrowserTab:              {"tab"},
	actionBrowserUp:               {"k", "up"},
	actionBrowserDown:             {"j", "down"},
	actionBrowserFilter:           {"/"},
	actionBrowserNewKey:           {"n"},
	actionBrowserRefresh:          {"r"},
	actionBrowserAutoRefresh:      {"a"},
	actionBrowserFlush:            {"ctrl+f"},
	actionBrowserMoreKeys:         {"g"},
	actionBrowserTTL:              {"t"},
	actionBrowserDelete:           {"d"},
	actionBrowserEdit:             {"e"},
	actionBrowserDetailAdd:        {"i"},
	actionBrowserDetailEdit:       {"e"},
	actionBrowserDetailDelete:     {"d"},
	actionBrowserCopy:             {"c"},
	actionBrowserFilterApply:      {"enter"},
	actionBrowserFilterCancel:     {"esc"},
	actionBrowserDetailSearchNext: {"n"},
	actionBrowserDetailSearchPrev: {"N"},

	actionEditEsc:      {"esc"},
	actionEditTab:      {"tab"},
	actionEditShiftTab: {"shift+tab"},

	actionConfirmYes: {"y"},
	actionConfirmNo:  {"n", "esc"},

	// Save participates in the same ctrl→alt transform as every other
	// ctrl-prefixed binding; settings.shortcut_modifier applies globally.
	actionSave: {"ctrl+s"},
}

func normalizeKey(key string) string {
	key = strings.TrimSpace(strings.ToLower(key))
	key = strings.ReplaceAll(key, " ", "")
	return key
}

// applyShortcutModifier rewrites a ctrl-prefixed binding to use the
// configured shortcut modifier. Anything that does not start with "ctrl+"
// is returned untouched so non-modifier keys (single keys, "tab",
// "shift+tab", "enter", ...) survive the global transform.
func applyShortcutModifier(key, modifier string) string {
	if modifier == "alt" && strings.HasPrefix(key, "ctrl+") {
		return "alt+" + strings.TrimPrefix(key, "ctrl+")
	}
	return key
}

// shortcutModifier resolves the user-configured modifier for derived
// shortcuts. Anything other than "alt" (including an unset config) keeps
// the default ctrl prefix.
func shortcutModifier(cfg *config.File) string {
	if cfg != nil && cfg.GetShortcutModifier() == "alt" {
		return "alt"
	}
	return "ctrl"
}

func keysFor(cfg *config.File, action string) []string {
	defaults, ok := defaultKeyMap[action]
	if !ok {
		return nil
	}
	modifier := shortcutModifier(cfg)
	out := make([]string, len(defaults))
	for i, k := range defaults {
		out[i] = applyShortcutModifier(k, modifier)
	}
	return out
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

// applicableHelpActions returns the flat, deduped list of bindings for the
// current focus. Global and Help-close entries are excluded because the keybar
// appends them itself (last line, always pinned). This is the function the
// keybar consumes and existing tests rely on.
func (m *Model) applicableHelpActions() []bindDef {
	seen := make(map[string]struct{}, 16)
	var out []bindDef
	for _, g := range m.activeHelpGroups() {
		for _, d := range g.Defs {
			// Skip global/help-close scopes — the keybar appends those
			// explicitly so they always end up pinned to line 2.
			if d.scope == scopeGlobal || d.scope == scopeHelp {
				continue
			}
			if _, ok := seen[d.id]; ok {
				continue
			}
			seen[d.id] = struct{}{}
			out = append(out, d)
		}
	}
	return out
}

// activeHelpGroups returns the scopes that should drive the keybar for the
// current focus. It is a subset of helpGroups: it only includes the panel
// scope (keys or detail) the user is currently on, so that the keybar does
// not duplicate bindings shared across panels (notably `/` and `c`).
func (m *Model) activeHelpGroups() []helpGroup {
	groups := []helpGroup{
		// Global binds (help, force quit) live outside the scope groups
		// here; applicableHelpActions filters them out and keyBinds appends
		// them itself.
		{},
	}
	switch m.Screen {
	case ScreenBrowser:
		groups = append(groups, helpGroup{Defs: m.browserCommonDefs()})
		switch {
		case m.DetailSearchFocus:
			groups = append(groups, helpGroup{Defs: m.detailSearchDefs()})
		case m.SearchFocus:
			groups = append(groups, helpGroup{Defs: m.keyFilterDefs()})
		case m.PanelFocus == panelDetail && m.KeyDetail != nil:
			groups = append(groups, helpGroup{Defs: m.browserDetailDefs()})
		default:
			groups = append(groups, helpGroup{Defs: m.browserKeysDefs()})
		}
	case ScreenKeyEdit:
		groups = append(groups, helpGroup{Defs: m.keyEditDefs()})
	case ScreenConfirm:
		if m.PrevScreen == ScreenProfileForm {
			groups = append(groups, helpGroup{Defs: m.profileFormDefs()})
		} else {
			groups = append(groups, helpGroup{Defs: m.confirmDefs()})
		}
	case ScreenProfileForm:
		groups = append(groups, helpGroup{Defs: m.profileFormDefs()})
	default:
		groups = append(groups, helpGroup{Defs: m.profilesDefs()})
	}
	return groups
}

// helpGroups returns the full list of scope groups to display in the help
// modal. Unlike activeHelpGroups it always emits both the Keys-panel and
// Detail-panel groups for the browser screen (plus a Global and a Help-close
// group) so the modal acts as a complete reference of every shortcut the
// user can press in the current screen, regardless of focus.
func (m *Model) helpGroups() []helpGroup {
	groups := []helpGroup{
		{Title: "Global", Defs: []bindDef{
			{actionAppHelp, "help", scopeGlobal},
			{actionAppForceQuit, "force quit", scopeGlobal},
		}},
	}
	switch m.Screen {
	case ScreenBrowser:
		groups = append(groups, helpGroup{
			Title: "Browser · Common",
			Defs:  m.browserCommonDefs(),
		})
		switch {
		case m.DetailSearchFocus:
			groups = append(groups,
				helpGroup{Title: "Detail search", Defs: m.detailSearchDefs()},
				helpGroup{Title: "Browser · Keys panel", Defs: m.browserKeysDefs()},
				helpGroup{Title: "Browser · Detail panel", Defs: m.browserDetailDefs()},
			)
		case m.SearchFocus:
			groups = append(groups,
				helpGroup{Title: "Key filter", Defs: m.keyFilterDefs()},
				helpGroup{Title: "Browser · Keys panel", Defs: m.browserKeysDefs()},
				helpGroup{Title: "Browser · Detail panel", Defs: m.browserDetailDefs()},
			)
		default:
			groups = append(groups,
				helpGroup{Title: "Browser · Keys panel", Defs: m.browserKeysDefs()},
				helpGroup{Title: "Browser · Detail panel", Defs: m.browserDetailDefs()},
			)
		}
	case ScreenKeyEdit:
		groups = append(groups, helpGroup{Title: "Key editor", Defs: m.keyEditDefs()})
	case ScreenConfirm:
		if m.PrevScreen == ScreenProfileForm {
			groups = append(groups, helpGroup{Title: "Profile form", Defs: m.profileFormDefs()})
		} else {
			groups = append(groups, helpGroup{Title: "Confirm", Defs: m.confirmDefs()})
		}
	case ScreenProfileForm:
		groups = append(groups, helpGroup{Title: "Profile form", Defs: m.profileFormDefs()})
	default:
		groups = append(groups, helpGroup{Title: "Profiles", Defs: m.profilesDefs()})
	}
	groups = append(groups, helpGroup{Title: "Help", Defs: []bindDef{
		{actionHelpClose, "close help", scopeHelp},
	}})
	return groups
}

func (m *Model) browserCommonDefs() []bindDef {
	defs := []bindDef{
		{actionBrowserDisconnect, "disconnect", scopeBrowserCommon},
		{actionBrowserTab, "switch panel", scopeBrowserCommon},
		{actionBrowserUp, "up", scopeBrowserCommon},
		{actionBrowserDown, "down", scopeBrowserCommon},
		{actionBrowserFlush, "flush db", scopeBrowserCommon},
	}
	if m.SelectedKey != "" {
		defs = append(defs, bindDef{actionBrowserTTL, "ttl", scopeBrowserCommon})
	}
	return defs
}

func (m *Model) browserKeysDefs() []bindDef {
	defs := []bindDef{
		{actionBrowserFilter, "filter", scopeBrowserKeys},
		{actionBrowserNewKey, "new key", scopeBrowserKeys},
		{actionBrowserRefresh, "refresh", scopeBrowserKeys},
		{actionBrowserAutoRefresh, "auto refresh", scopeBrowserKeys},
	}
	if m.ScanCursor != 0 {
		defs = append([]bindDef{
			{actionBrowserMoreKeys, "load more keys", scopeBrowserKeys},
		}, defs...)
	}
	if m.KeyDetail != nil && m.SelectedKey != "" {
		defs = append(defs, bindDef{actionBrowserCopy, "copy value", scopeBrowserKeys})
	}
	if m.SelectedKey != "" {
		defs = append(defs, bindDef{actionBrowserDelete, "delete key", scopeBrowserKeys})
		if m.KeyDetail != nil && editableKeyType(m.KeyDetail.Meta.Type) {
			defs = append(defs, bindDef{actionBrowserEdit, "edit key", scopeBrowserKeys})
		}
	}
	return defs
}

func (m *Model) browserDetailDefs() []bindDef {
	if m.KeyDetail == nil {
		return nil
	}
	defs := []bindDef{
		{actionBrowserFilter, "search value", scopeBrowserDetail},
	}
	switch {
	case compositeKeyType(m.KeyDetail.Meta.Type):
		defs = append(defs,
			bindDef{actionBrowserDetailAdd, "add item", scopeBrowserDetail},
			bindDef{actionBrowserDetailEdit, "edit item", scopeBrowserDetail},
			bindDef{actionBrowserDetailDelete, "delete item", scopeBrowserDetail},
		)
	case m.KeyDetail.Meta.Type == "string":
		defs = append(defs,
			bindDef{actionBrowserDetailEdit, "edit value", scopeBrowserDetail},
			bindDef{actionBrowserDelete, "delete key", scopeBrowserDetail},
		)
	}
	if m.SelectedKey != "" {
		defs = append(defs, bindDef{actionBrowserCopy, "copy value", scopeBrowserDetail})
	}
	return defs
}

func (m *Model) keyFilterDefs() []bindDef {
	return []bindDef{
		{actionBrowserFilterApply, "apply filter", scopeKeyFilter},
		{actionBrowserFilterCancel, "close filter", scopeKeyFilter},
	}
}

func (m *Model) detailSearchDefs() []bindDef {
	defs := []bindDef{
		{actionBrowserFilterApply, "apply search", scopeDetailSearch},
		{actionBrowserFilterCancel, "close search", scopeDetailSearch},
	}
	// n/N only cycle matches after a successful apply — surface them in
	// the help modal only when the conditions are met so the hint stays
	// honest.
	if m.DetailSearchInput.Value() != "" && len(m.DetailSearchMatches) > 0 {
		defs = append(defs,
			bindDef{actionBrowserDetailSearchNext, "next match", scopeDetailSearch},
			bindDef{actionBrowserDetailSearchPrev, "previous match", scopeDetailSearch},
		)
	}
	return defs
}

func (m *Model) profilesDefs() []bindDef {
	return []bindDef{
		{actionProfilesUp, "up", scopeProfiles},
		{actionProfilesDown, "down", scopeProfiles},
		{actionProfilesConnect, "connect", scopeProfiles},
		{actionProfilesNew, "new", scopeProfiles},
		{actionProfilesEdit, "edit", scopeProfiles},
		{actionProfilesDelete, "delete", scopeProfiles},
		{actionProfilesQuit, "quit", scopeProfiles},
	}
}

func (m *Model) profileFormDefs() []bindDef {
	return []bindDef{
		{actionFormTab, "next field", scopeProfileForm},
		{actionFormShiftTab, "previous field", scopeProfileForm},
		{actionSave, "save", scopeProfileForm},
		{actionFormEsc, "cancel", scopeProfileForm},
	}
}

func (m *Model) keyEditDefs() []bindDef {
	switch m.EditMode {
	case editElement, editElementAdd:
		return []bindDef{
			{actionSave, "save", scopeKeyEdit},
			{actionEditEsc, "cancel", scopeKeyEdit},
		}
	case editNewKey, editExistingKey:
		defs := []bindDef{
			{actionEditTab, "next field", scopeKeyEdit},
			{actionEditShiftTab, "previous field", scopeKeyEdit},
			{actionSave, "save", scopeKeyEdit},
			{actionEditEsc, "cancel", scopeKeyEdit},
		}
		if m.EditMode == editNewKey && m.NewKeyFocus == newKeyFieldType {
			defs = append([]bindDef{
				{actionBrowserUp, "up", scopeKeyEdit},
				{actionBrowserDown, "down", scopeKeyEdit},
			}, defs...)
		}
		return defs
	default:
		return []bindDef{
			{actionSave, "save", scopeKeyEdit},
			{actionEditEsc, "cancel", scopeKeyEdit},
		}
	}
}

func (m *Model) confirmDefs() []bindDef {
	return []bindDef{
		{actionConfirmYes, "confirm", scopeConfirm},
		{actionConfirmNo, "cancel", scopeConfirm},
	}
}
