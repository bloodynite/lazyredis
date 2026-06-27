package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/lipgloss"
	"github.com/redis/go-redis/v9"

	"github.com/bloodynite/lazyredis/internal/store"
)

// startDetailEditForType is a small helper that wires a Model up for a
// given detail type so each test can stay focused on assertions.
func startDetailEditForType(t *testing.T, d *store.KeyDetail, cursor int) *Model {
	t.Helper()
	m := New()
	m.Width = 120
	m.Height = 24
	m.Client = &store.Client{}
	m.SelectedKey = d.Meta.Key
	m.KeyDetail = d
	m.DetailCursor = cursor
	next, _ := m.startDetailEdit()
	return next.(*Model)
}

func TestElementEditUsesTextarea(t *testing.T) {
	m := New()
	for _, mode := range []editMode{editElement, editElementAdd} {
		m.EditMode = mode
		if !m.elementEditUsesTextarea() {
			t.Fatalf("expected textarea for mode %v", mode)
		}
	}
	m.EditMode = editRefreshInterval
	if m.elementEditUsesTextarea() {
		t.Fatal("refresh interval should not use element textarea")
	}
	m.EditMode = editNewKey
	if m.elementEditUsesTextarea() {
		t.Fatal("new key form should not use element textarea")
	}
}

func TestStartDetailEditStringLoadsIntoTextarea(t *testing.T) {
	m := startDetailEditForType(t, &store.KeyDetail{
		Meta:   store.KeyMeta{Key: "demo:key", Type: "string"},
		String: "hello world",
	}, 0)

	if m.EditMode != editElement {
		t.Fatalf("edit mode = %v, want editElement", m.EditMode)
	}
	if m.Screen != ScreenKeyEdit {
		t.Fatalf("screen = %v, want ScreenKeyEdit", m.Screen)
	}
	if m.KeyFormType != "string" {
		t.Fatalf("KeyFormType = %q, want string", m.KeyFormType)
	}
	if got := m.NewKeyValue.Value(); got != "hello world" {
		t.Fatalf("NewKeyValue = %q, want hello world", got)
	}
	if got := m.EditInput.Value(); got != "" {
		t.Fatalf("EditInput should stay empty, got %q", got)
	}
	if !m.elementEditUsesTextarea() {
		t.Fatal("string detail edit should render via textarea")
	}
}

func TestStartDetailEditHashLoadsValueIntoTextarea(t *testing.T) {
	d := &store.KeyDetail{
		Meta: store.KeyMeta{Key: "demo:hash", Type: "hash"},
		Hash: map[string]string{"name": "redis", "lang": "go"},
	}
	fields := hashFields(d.Hash)
	idx := indexOf(fields, "lang")
	if idx < 0 {
		t.Fatal("lang field not found in hash fields")
	}
	m := startDetailEditForType(t, d, idx)

	if got := m.NewKeyValue.Value(); got != "go" {
		t.Fatalf("NewKeyValue = %q, want go", got)
	}
	if m.EditField != "lang" {
		t.Fatalf("EditField = %q, want lang", m.EditField)
	}
	if got := m.EditInput.Value(); got != "" {
		t.Fatalf("EditInput should stay empty, got %q", got)
	}
	if !m.elementEditUsesTextarea() {
		t.Fatal("hash detail edit should render via textarea")
	}
}

func TestStartDetailEditListLoadsIntoTextarea(t *testing.T) {
	d := &store.KeyDetail{
		Meta: store.KeyMeta{Key: "demo:list", Type: "list"},
		List: []string{"alpha", "beta"},
	}
	m := startDetailEditForType(t, d, 1)

	if got := m.NewKeyValue.Value(); got != "beta" {
		t.Fatalf("NewKeyValue = %q, want beta", got)
	}
	if got := m.EditInput.Value(); got != "" {
		t.Fatalf("EditInput should stay empty, got %q", got)
	}
}

func TestStartDetailEditSetLoadsIntoTextarea(t *testing.T) {
	d := &store.KeyDetail{
		Meta: store.KeyMeta{Key: "demo:set", Type: "set"},
		Set:  []string{"alpha", "beta"},
	}
	m := startDetailEditForType(t, d, 0)

	if got := m.NewKeyValue.Value(); got != "alpha" {
		t.Fatalf("NewKeyValue = %q, want alpha", got)
	}
	if m.EditField != "alpha" {
		t.Fatalf("EditField = %q, want alpha (old member)", m.EditField)
	}
}

func TestStartDetailEditZSetLoadsScoreAndMemberIntoTextarea(t *testing.T) {
	d := &store.KeyDetail{
		Meta: store.KeyMeta{Key: "demo:zset", Type: "zset"},
		ZSet: []redis.Z{{Score: 12.5, Member: "k1"}, {Score: 7, Member: "k2"}},
	}
	m := startDetailEditForType(t, d, 1)

	// The textarea replaces tabs with 4 spaces, so we check the round-trip
	// through ParseZSetLine instead of the raw stored string.
	score, member, err := store.ParseZSetLine(m.NewKeyValue.Value())
	if err != nil {
		t.Fatalf("ParseZSetLine(%q): %v", m.NewKeyValue.Value(), err)
	}
	if score != 7 || member != "k2" {
		t.Fatalf("parsed = (%v, %q), want (7, k2)", score, member)
	}
	if m.EditField != "k2" {
		t.Fatalf("EditField = %q, want k2", m.EditField)
	}
}

func TestStartDetailEditStreamLoadsMultiLineIntoTextarea(t *testing.T) {
	d := &store.KeyDetail{
		Meta: store.KeyMeta{Key: "demo:stream", Type: "stream"},
		Stream: []store.StreamEntry{{
			ID:     "1-0",
			Fields: map[string]string{"a": "1", "b": "two"},
		}},
	}
	m := startDetailEditForType(t, d, 0)

	got := m.NewKeyValue.Value()
	if !strings.Contains(got, "a=1") || !strings.Contains(got, "b=two") {
		t.Fatalf("NewKeyValue = %q, want both field=value lines", got)
	}
	if m.EditField != "1-0" {
		t.Fatalf("EditField = %q, want 1-0", m.EditField)
	}
}

func TestStartDetailAddResetsTextareaWithPlaceholder(t *testing.T) {
	d := &store.KeyDetail{
		Meta: store.KeyMeta{Key: "demo:hash", Type: "hash"},
		Hash: map[string]string{"k": "v"},
	}
	m := New()
	m.Width = 120
	m.Height = 24
	m.Client = &store.Client{}
	m.SelectedKey = d.Meta.Key
	m.KeyDetail = d
	m.NewKeyValue.SetValue("leftover from previous edit")
	next, _ := m.startDetailAdd()
	m = next.(*Model)

	if m.EditMode != editElementAdd {
		t.Fatalf("edit mode = %v, want editElementAdd", m.EditMode)
	}
	if got := m.NewKeyValue.Value(); got != "" {
		t.Fatalf("NewKeyValue should be reset, got %q", got)
	}
	if !strings.Contains(m.NewKeyValue.Placeholder, "field=value") {
		t.Fatalf("placeholder = %q, want field=value hint", m.NewKeyValue.Placeholder)
	}
}

func TestViewDetailEditStringRendersTextarea(t *testing.T) {
	m := New()
	m.Width = 120
	m.Height = 24
	m.Client = &store.Client{}
	m.SelectedKey = "demo:key"
	m.KeyDetail = &store.KeyDetail{
		Meta:   store.KeyMeta{Key: "demo:key", Type: "string"},
		String: "hello world",
	}
	m.DetailCursor = 0
	next, _ := m.startDetailEdit()
	m = next.(*Model)

	view := m.View()
	if !strings.Contains(view, "hello world") {
		t.Fatalf("detail edit view should show value, got:\n%s", view)
	}
	// EditInput is not part of the element edit layout; the textarea view
	// replaces it. Make sure the EditInput value (even if rendered through
	// the textarea control chars) does not appear in the body when it is
	// empty: the rendered view should only echo the NewKeyValue content.
	if strings.Contains(view, m.EditInput.View()) {
		t.Fatalf("detail edit view should not render EditInput")
	}
}

func TestStartDetailEditStringCarriesMultilinePayload(t *testing.T) {
	// Simulate user pasting a multi-line value into the textarea: the
	// detail edit must keep newlines intact through the textarea so that
	// saving the value preserves them.
	m := startDetailEditForType(t, &store.KeyDetail{
		Meta:   store.KeyMeta{Key: "demo:multi", Type: "string"},
		String: "line1\nline2\nline3",
	}, 0)
	if got := m.NewKeyValue.Value(); got != "line1\nline2\nline3" {
		t.Fatalf("NewKeyValue = %q, want multi-line payload preserved", got)
	}
}

// TestSubmitElementEditHashSendsRawValue confirms that submitting a hash
// element edit does NOT parse the NewKeyValue as "field=value"; the field
// is preserved on EditField and the raw value is forwarded as-is to the
// patch command.
func TestSubmitElementEditHashSendsRawValue(t *testing.T) {
	m := New()
	m.Width = 120
	m.Height = 24
	m.Client = &store.Client{}
	m.SelectedKey = "demo:hash"
	m.KeyDetail = &store.KeyDetail{
		Meta: store.KeyMeta{Key: "demo:hash", Type: "hash"},
		Hash: map[string]string{"name": "redis"},
	}
	m.DetailCursor = 0
	m.EditField = "name"

	// Load the edit modal, then mutate the textarea as a user would.
	next, _ := m.startDetailEdit()
	m = next.(*Model)
	if m.NewKeyValue.Value() != "redis" {
		t.Fatalf("initial NewKeyValue = %q, want redis", m.NewKeyValue.Value())
	}
	m.NewKeyValue.SetValue("new=value with = signs")

	// Submit must not error out on "=" parsing — it should send the raw
	// value straight through and produce a non-nil command.
	next, cmd := m.submitElementEdit()
	m = next.(*Model)
	if m.ErrMsg != "" {
		t.Fatalf("ErrMsg = %q, want empty (raw value should not be parsed)", m.ErrMsg)
	}
	if cmd == nil {
		t.Fatal("expected a submit command")
	}
	if !m.Loading {
		t.Fatal("submit should set loading")
	}
	if m.EditField != "name" {
		t.Fatalf("EditField changed to %q, want name preserved", m.EditField)
	}
}

func indexOf(items []string, target string) int {
	for i, v := range items {
		if v == target {
			return i
		}
	}
	return -1
}

// loadElementEdit is a tiny shared builder: it wires a Model up with the given
// detail and DetailCursor, opens the element edit modal, and returns it ready
// for the caller to mutate m.NewKeyValue and submit.
func loadElementEdit(t *testing.T, d *store.KeyDetail, cursor int) *Model {
	t.Helper()
	m := New()
	m.Width = 120
	m.Height = 24
	m.Client = &store.Client{}
	m.SelectedKey = d.Meta.Key
	m.KeyDetail = d
	m.DetailCursor = cursor
	next, _ := m.startDetailEdit()
	return next.(*Model)
}

// TestSubmitElementEditStringPreservesExactValue pins the bugfix: submitting a
// string element edit must forward the textarea's exact bytes (including
// leading/trailing whitespace and embedded newlines) to the patch command.
// Trimming here would silently corrupt Redis string payloads that depend on
// exact byte preservation.
func TestSubmitElementEditStringPreservesExactValue(t *testing.T) {
	const payload = "  hello\nworld\n  "
	m := loadElementEdit(t, &store.KeyDetail{
		Meta:   store.KeyMeta{Key: "demo:str", Type: "string"},
		String: "orig",
	}, 0)
	m.NewKeyValue.SetValue(payload)

	var (
		capturedKey   string
		capturedValue string
	)
	orig := patchStringValueFn
	patchStringValueFn = func(_ *store.Client, key, value string) tea.Cmd {
		capturedKey, capturedValue = key, value
		return func() tea.Msg { return actionDoneMsg{status: "ok"} }
	}
	defer func() { patchStringValueFn = orig }()

	next, cmd := m.submitElementEdit()
	m = next.(*Model)

	if m.ErrMsg != "" {
		t.Fatalf("ErrMsg = %q, want empty (exact bytes must not be rejected)", m.ErrMsg)
	}
	if cmd == nil {
		t.Fatal("expected a submit command for string edit")
	}
	if !m.Loading {
		t.Fatal("submit should set loading")
	}
	if got := m.NewKeyValue.Value(); got != payload {
		t.Fatalf("textarea mutated by submit: got %q, want %q", got, payload)
	}
	if capturedKey != "demo:str" {
		t.Fatalf("boundary key = %q, want demo:str", capturedKey)
	}
	if capturedValue != payload {
		t.Fatalf("boundary value trimmed by submit: got %q, want %q", capturedValue, payload)
	}
}

// TestSubmitElementEditHashPreservesExactValue mirrors the string test for
// the hash field edit flow: the textarea holds only the field value (the field
// name lives on EditField), so its exact bytes must reach patchHashField
// without a TrimSpace round-trip.
func TestSubmitElementEditHashPreservesExactValue(t *testing.T) {
	const payload = "  leading\ntrailing  \n\n"
	d := &store.KeyDetail{
		Meta: store.KeyMeta{Key: "demo:hash", Type: "hash"},
		Hash: map[string]string{"name": "redis"},
	}
	m := loadElementEdit(t, d, 0)
	if m.EditField != "name" {
		t.Fatalf("EditField = %q, want name", m.EditField)
	}
	m.NewKeyValue.SetValue(payload)

	var (
		capturedKey   string
		capturedField string
		capturedValue string
	)
	orig := patchHashFieldFn
	patchHashFieldFn = func(_ *store.Client, key, field, value string) tea.Cmd {
		capturedKey, capturedField, capturedValue = key, field, value
		return func() tea.Msg { return actionDoneMsg{status: "ok"} }
	}
	defer func() { patchHashFieldFn = orig }()

	next, cmd := m.submitElementEdit()
	m = next.(*Model)

	if m.ErrMsg != "" {
		t.Fatalf("ErrMsg = %q, want empty (exact bytes must not be rejected)", m.ErrMsg)
	}
	if cmd == nil {
		t.Fatal("expected a submit command for hash field edit")
	}
	if !m.Loading {
		t.Fatal("submit should set loading")
	}
	if got := m.NewKeyValue.Value(); got != payload {
		t.Fatalf("textarea mutated by submit: got %q, want %q", got, payload)
	}
	if m.EditField != "name" {
		t.Fatalf("EditField = %q, want name preserved", m.EditField)
	}
	if capturedKey != "demo:hash" || capturedField != "name" {
		t.Fatalf("boundary args = (%q, %q, %q), want (demo:hash, name, <payload>)",
			capturedKey, capturedField, capturedValue)
	}
	if capturedValue != payload {
		t.Fatalf("boundary value trimmed by submit: got %q, want %q", capturedValue, payload)
	}
}

// TestSubmitElementEditListPreservesExactValue ensures list item edits also
// keep exact bytes. Whitespace-only or whitespace-padded list items are valid
// Redis values; trimming would silently collapse distinct entries.
func TestSubmitElementEditListPreservesExactValue(t *testing.T) {
	// The textarea widget expands tabs to 4 spaces; use only newlines and
	// spaces here so we assert on bytes the widget passes through verbatim.
	const payload = " first\n   spaced\n  with trailing  \n  "
	d := &store.KeyDetail{
		Meta: store.KeyMeta{Key: "demo:list", Type: "list"},
		List: []string{"alpha"},
	}
	m := loadElementEdit(t, d, 0)
	m.NewKeyValue.SetValue(payload)

	var (
		capturedKey   string
		capturedIndex int
		capturedValue string
	)
	orig := patchListItemFn
	patchListItemFn = func(_ *store.Client, key string, index int, value string) tea.Cmd {
		capturedKey, capturedIndex, capturedValue = key, index, value
		return func() tea.Msg { return actionDoneMsg{status: "ok"} }
	}
	defer func() { patchListItemFn = orig }()

	next, cmd := m.submitElementEdit()
	m = next.(*Model)

	if m.ErrMsg != "" {
		t.Fatalf("ErrMsg = %q, want empty", m.ErrMsg)
	}
	if cmd == nil {
		t.Fatal("expected a submit command for list item edit")
	}
	if got := m.NewKeyValue.Value(); got != payload {
		t.Fatalf("textarea mutated by submit: got %q, want %q", got, payload)
	}
	if capturedKey != "demo:list" || capturedIndex != 0 {
		t.Fatalf("boundary args = (%q, %d, %q), want (demo:list, 0, <payload>)",
			capturedKey, capturedIndex, capturedValue)
	}
	if capturedValue != payload {
		t.Fatalf("boundary value trimmed by submit: got %q, want %q", capturedValue, payload)
	}
}

// TestSubmitElementEditSetPreservesExactValue guards set member edits. Set
// members in Redis are exact strings; "  admin  " and "admin" are distinct
// members, so trimming would corrupt membership semantics.
func TestSubmitElementEditSetPreservesExactValue(t *testing.T) {
	const payload = "  spaced member  \n"
	d := &store.KeyDetail{
		Meta: store.KeyMeta{Key: "demo:set", Type: "set"},
		Set:  []string{"alpha"},
	}
	m := loadElementEdit(t, d, 0)
	m.NewKeyValue.SetValue(payload)

	var (
		capturedKey      string
		capturedOld      string
		capturedNew      string
	)
	orig := replaceSetMemberFn
	replaceSetMemberFn = func(_ *store.Client, key, oldMember, newMember string) tea.Cmd {
		capturedKey, capturedOld, capturedNew = key, oldMember, newMember
		return func() tea.Msg { return actionDoneMsg{status: "ok"} }
	}
	defer func() { replaceSetMemberFn = orig }()

	next, cmd := m.submitElementEdit()
	m = next.(*Model)

	if m.ErrMsg != "" {
		t.Fatalf("ErrMsg = %q, want empty", m.ErrMsg)
	}
	if cmd == nil {
		t.Fatal("expected a submit command for set member edit")
	}
	if got := m.NewKeyValue.Value(); got != payload {
		t.Fatalf("textarea mutated by submit: got %q, want %q", got, payload)
	}
	if m.EditField != "alpha" {
		t.Fatalf("EditField = %q, want alpha (old member) preserved", m.EditField)
	}
	if capturedKey != "demo:set" || capturedOld != "alpha" {
		t.Fatalf("boundary args = (%q, %q, %q), want (demo:set, alpha, <payload>)",
			capturedKey, capturedOld, capturedNew)
	}
	if capturedNew != payload {
		t.Fatalf("boundary new member trimmed by submit: got %q, want %q", capturedNew, payload)
	}
}

// TestSubmitElementEditZSetPreservesMemberWhitespace pins zset behavior: the
// parser trims the score but the member goes to Redis verbatim. Leading or
// trailing whitespace in the member is meaningful (distinct member identity),
// so the submit must not pre-trim the textarea.
//
// Note: ParseZSetLine itself does strings.TrimSpace on the whole line before
// splitting, so trailing newlines on the textarea payload are dropped at the
// parser layer. The contract enforced here is narrower: the value forwarded
// FROM submitElementEdit TO replaceZSetMember must be the textarea's exact
// bytes (no extra TrimSpace in submitElementEdit).
func TestSubmitElementEditZSetPreservesMemberWhitespace(t *testing.T) {
	const payload = "3.14   spaced member   "
	d := &store.KeyDetail{
		Meta: store.KeyMeta{Key: "demo:zset", Type: "zset"},
		ZSet: []redis.Z{{Score: 1, Member: "old"}},
	}
	m := loadElementEdit(t, d, 0)
	m.NewKeyValue.SetValue(payload)

	var (
		capturedKey     string
		capturedOld     string
		capturedLine    string
	)
	orig := replaceZSetMemberFn
	replaceZSetMemberFn = func(_ *store.Client, key, oldMember, line string) tea.Cmd {
		capturedKey, capturedOld, capturedLine = key, oldMember, line
		return func() tea.Msg { return actionDoneMsg{status: "ok"} }
	}
	defer func() { replaceZSetMemberFn = orig }()

	next, cmd := m.submitElementEdit()
	m = next.(*Model)

	if m.ErrMsg != "" {
		t.Fatalf("ErrMsg = %q, want empty", m.ErrMsg)
	}
	if cmd == nil {
		t.Fatal("expected a submit command for zset member edit")
	}
	if got := m.NewKeyValue.Value(); got != payload {
		t.Fatalf("textarea mutated by submit: got %q, want %q", got, payload)
	}
	if capturedKey != "demo:zset" || capturedOld != "old" {
		t.Fatalf("boundary args = (%q, %q, %q), want (demo:zset, old, <payload>)",
			capturedKey, capturedOld, capturedLine)
	}
	if capturedLine != payload {
		t.Fatalf("boundary line trimmed by submit: got %q, want %q", capturedLine, payload)
	}
}

// TestSubmitElementEditStreamPreservesFieldValues ensures stream field edits
// keep exact field values. ParseStreamFields splits on \n and skips blank
// lines, but each surviving line's value half goes to Redis verbatim.
//
// We assert on the raw payload forwarded to replaceStreamEntry: submitElementEdit
// itself must not pre-trim it. Per-line value trimming is a parser concern
// covered by store tests.
func TestSubmitElementEditStreamPreservesFieldValues(t *testing.T) {
	const payload = "a=  spaced value  \nb=keep\n"
	d := &store.KeyDetail{
		Meta: store.KeyMeta{Key: "demo:stream", Type: "stream"},
		Stream: []store.StreamEntry{{
			ID:     "1-0",
			Fields: map[string]string{"a": "1", "b": "two"},
		}},
	}
	m := loadElementEdit(t, d, 0)
	m.NewKeyValue.SetValue(payload)

	var (
		capturedKey string
		capturedID  string
		capturedRaw string
	)
	orig := replaceStreamEntryFn
	replaceStreamEntryFn = func(_ *store.Client, key, id, raw string) tea.Cmd {
		capturedKey, capturedID, capturedRaw = key, id, raw
		return func() tea.Msg { return actionDoneMsg{status: "ok"} }
	}
	defer func() { replaceStreamEntryFn = orig }()

	next, cmd := m.submitElementEdit()
	m = next.(*Model)

	if m.ErrMsg != "" {
		t.Fatalf("ErrMsg = %q, want empty", m.ErrMsg)
	}
	if cmd == nil {
		t.Fatal("expected a submit command for stream entry edit")
	}
	if got := m.NewKeyValue.Value(); got != payload {
		t.Fatalf("textarea mutated by submit: got %q, want %q", got, payload)
	}
	if m.EditField != "1-0" {
		t.Fatalf("EditField = %q, want 1-0 (entry id) preserved", m.EditField)
	}
	if capturedKey != "demo:stream" || capturedID != "1-0" {
		t.Fatalf("boundary args = (%q, %q, %q), want (demo:stream, 1-0, <payload>)",
			capturedKey, capturedID, capturedRaw)
	}
	if capturedRaw != payload {
		t.Fatalf("boundary raw trimmed by submit: got %q, want %q", capturedRaw, payload)
	}
}

// TestSubmitElementEditAddHashPreservesValueWhitespace keeps the add/hash
// "field=value" flow honest: the parser trims the field name, but the value
// half must be forwarded with its leading/trailing whitespace intact.
//
// ParseHashFieldLine applies TrimSpace to the field side and returns the
// value side verbatim. The boundary contract enforced here is that the whole
// "field=value" line reaches addHashField exactly as the user typed it.
func TestSubmitElementEditAddHashPreservesValueWhitespace(t *testing.T) {
	const payload = "field=  spaced value\n"
	d := &store.KeyDetail{
		Meta: store.KeyMeta{Key: "demo:hash", Type: "hash"},
		Hash: map[string]string{"k": "v"},
	}
	m := New()
	m.Width = 120
	m.Height = 24
	m.Client = &store.Client{}
	m.SelectedKey = d.Meta.Key
	m.KeyDetail = d
	next, _ := m.startDetailAdd()
	m = next.(*Model)
	m.NewKeyValue.SetValue(payload)

	var (
		capturedKey string
		capturedLine string
	)
	orig := addHashFieldFn
	addHashFieldFn = func(_ *store.Client, key, line string) tea.Cmd {
		capturedKey, capturedLine = key, line
		return func() tea.Msg { return actionDoneMsg{status: "ok"} }
	}
	defer func() { addHashFieldFn = orig }()

	next, cmd := m.submitElementEdit()
	m = next.(*Model)

	if m.ErrMsg != "" {
		t.Fatalf("ErrMsg = %q, want empty (value whitespace should not be rejected)", m.ErrMsg)
	}
	if cmd == nil {
		t.Fatal("expected a submit command for hash field add")
	}
	if got := m.NewKeyValue.Value(); got != payload {
		t.Fatalf("textarea mutated by submit: got %q, want %q", got, payload)
	}
	if capturedKey != "demo:hash" {
		t.Fatalf("boundary key = %q, want demo:hash", capturedKey)
	}
	if capturedLine != payload {
		t.Fatalf("boundary line trimmed by submit: got %q, want %q", capturedLine, payload)
	}
}

func TestSyncNewKeyLayoutBodyOverlayForElementEdit(t *testing.T) {
	cases := []struct {
		name       string
		width      int
		height     int
		wantHeight int
	}{
		{"standard", 120, 24, 17},
		{"wide", 200, 40, 33},
		{"narrow", 40, 12, 5},
		{"tiny clamped to 2", 30, 8, 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, mode := range []editMode{editElement, editElementAdd} {
				m := New()
				m.Width = tc.width
				m.Height = tc.height
				m.EditMode = mode
				m.syncNewKeyLayout()

				if got := textareaRenderedWidth(m.NewKeyValue); got != tc.width {
					t.Errorf("mode=%v rendered width = %d, want %d", mode, got, tc.width)
				}
				if got := m.NewKeyValue.Height(); got != tc.wantHeight {
					t.Errorf("mode=%v textarea height = %d, want %d", mode, got, tc.wantHeight)
				}
				wantTitleLines := 1
				wantHintLines := 1
				totalContent := wantTitleLines + tc.wantHeight + wantHintLines
				if totalContent > m.panelAreaLines() {
					t.Errorf("mode=%v title(%d)+textarea(%d)+hint(%d)=%d overflows body overlay height %d",
						mode, wantTitleLines, tc.wantHeight, wantHintLines, totalContent, m.panelAreaLines())
				}
			}
		})
	}
}

func TestSyncNewKeyLayoutModalKeepsModalDimensions(t *testing.T) {
	for _, mode := range []editMode{editNewKey, editExistingKey, editTTL} {
		t.Run(editModeName(mode), func(t *testing.T) {
			m := New()
			m.Width = 120
			m.Height = 24
			m.EditMode = mode
			m.syncNewKeyLayout()

			wantInputW := min(62, max(36, 120*2/3-8))
			if got := m.NewKeyTTL.Width; got != wantInputW {
				t.Errorf("NewKeyTTL width = %d, want %d", got, wantInputW)
			}
			if got := m.NewKeyName.Width; got != wantInputW {
				t.Errorf("NewKeyName width = %d, want %d", got, wantInputW)
			}
			if got := textareaRenderedWidth(m.NewKeyValue); got != wantInputW {
				t.Errorf("NewKeyValue rendered width = %d, want %d", got, wantInputW)
			}
			wantH := m.newKeyValueHeight()
			if got := m.NewKeyValue.Height(); got != wantH {
				t.Errorf("NewKeyValue height = %d, want %d", got, wantH)
			}
			if m.newKeyValueHeight() > 12 {
				t.Errorf("modal textarea height %d exceeds 12-row cap", m.newKeyValueHeight())
			}
		})
	}
}

func textareaRenderedWidth(t textarea.Model) int {
	view := t.View()
	for _, l := range strings.Split(view, "\n") {
		if w := lipgloss.Width(l); w > 0 {
			return w
		}
	}
	return 0
}

func editModeName(m editMode) string {
	switch m {
	case editElement:
		return "editElement"
	case editElementAdd:
		return "editElementAdd"
	case editNewKey:
		return "editNewKey"
	case editExistingKey:
		return "editExistingKey"
	case editTTL:
		return "editTTL"
	case editRefreshInterval:
		return "editRefreshInterval"
	default:
		return "editString"
	}
}

func TestViewDetailEditTextareaFillsBodyOverlay(t *testing.T) {
	m := New()
	m.Width = 120
	m.Height = 24
	m.Client = &store.Client{}
	m.SelectedKey = "demo:key"
	m.KeyDetail = &store.KeyDetail{
		Meta:   store.KeyMeta{Key: "demo:key", Type: "string"},
		String: "hello world",
	}
	m.DetailCursor = 0
	next, _ := m.startDetailEdit()
	m = next.(*Model)

	if got := textareaRenderedWidth(m.NewKeyValue); got != m.Width {
		t.Fatalf("textarea rendered width = %d, want full width %d", got, m.Width)
	}
	wantH := m.panelAreaLines() - 2
	if got := m.NewKeyValue.Height(); got != wantH {
		t.Fatalf("textarea height = %d, want %d (panelAreaLines=%d - title - hint)",
			got, wantH, m.panelAreaLines())
	}

	out := m.View()
	lines := strings.Split(out, "\n")
	if len(lines) != m.Height {
		t.Fatalf("view lines = %d, want %d", len(lines), m.Height)
	}
	panelStart := gridInfoRows
	panelEnd := panelStart + m.panelAreaLines()
	panelLines := lines[panelStart:panelEnd]

	hasTitle := false
	hasHint := false
	for _, l := range panelLines {
		plain := stripANSI(l)
		if strings.Contains(plain, "Edit item") {
			hasTitle = true
		}
		if strings.Contains(plain, "save") && strings.Contains(plain, "cancel") {
			hasHint = true
		}
	}
	if !hasTitle {
		t.Fatalf("detail edit body overlay missing title line; panel=%q", strings.Join(panelLines, "\n"))
	}
	if !hasHint {
		t.Fatalf("detail edit body overlay missing save/cancel hint line; panel=%q", strings.Join(panelLines, "\n"))
	}

	for i, l := range panelLines {
		if w := lipgloss.Width(l); w != m.Width {
			t.Fatalf("panel line %d width = %d, want %d (line=%q)", i, w, m.Width, l)
		}
	}
}

func TestIsKeyBodyTooLargeForKeyPanel(t *testing.T) {
	cases := []struct {
		name string
		d    *store.KeyDetail
		want bool
	}{
		{
			name: "nil detail",
			d:    nil,
			want: false,
		},
		{
			name: "short string",
			d:    &store.KeyDetail{Meta: store.KeyMeta{Key: "k", Type: "string"}, String: "hello"},
			want: false,
		},
		{
			name: "string too long in one line",
			d:    &store.KeyDetail{Meta: store.KeyMeta{Key: "k", Type: "string"}, String: strings.Repeat("x", 5000)},
			want: true,
		},
		{
			name: "string with many newlines",
			d:    &store.KeyDetail{Meta: store.KeyMeta{Key: "k", Type: "string"}, String: strings.Repeat("a\n", 20)},
			want: true,
		},
		{
			name: "list with many items",
			d:    &store.KeyDetail{Meta: store.KeyMeta{Key: "k", Type: "list"}, List: makeList(20)},
			want: true,
		},
		{
			name: "list with few items",
			d:    &store.KeyDetail{Meta: store.KeyMeta{Key: "k", Type: "list"}, List: []string{"a", "b"}},
			want: false,
		},
		{
			name: "unknown type",
			d:    &store.KeyDetail{Meta: store.KeyMeta{Key: "k", Type: "weird"}},
			want: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := &Model{KeyDetail: tc.d}
			if got := m.isKeyBodyTooLargeForKeyPanel(); got != tc.want {
				t.Fatalf("isKeyBodyTooLargeForKeyPanel = %v, want %v", got, tc.want)
			}
		})
	}
}

func makeList(n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = fmt.Sprintf("item-%d", i)
	}
	return out
}

func TestNewKeyValueCharLimitAcceptsLargeValue(t *testing.T) {
	m := New()
	if m.NewKeyValue.CharLimit != keyPanelCharLimit {
		t.Fatalf("CharLimit = %d, want %d", m.NewKeyValue.CharLimit, keyPanelCharLimit)
	}
	large := strings.Repeat("z", 50000)
	m.NewKeyValue.SetValue(large)
	if got := m.NewKeyValue.Value(); len(got) != len(large) {
		t.Fatalf("textarea truncated large value: got %d chars, want %d", len(got), len(large))
	}
}

func TestKeyPanelEditKeepsModalWhenBodySmall(t *testing.T) {
	m := New()
	m.Width = 120
	m.Height = 24
	m.PanelFocus = panelKeys
	m.SelectedKey = "demo:small"
	m.Client = &store.Client{}
	m.KeyDetail = &store.KeyDetail{
		Meta:   store.KeyMeta{Key: "demo:small", Type: "string"},
		String: "short value",
	}

	next, _ := m.startEdit()
	m = next.(*Model)

	if m.PanelFocus != panelKeys {
		t.Fatalf("PanelFocus = %v, want panelKeys (no layout switch)", m.PanelFocus)
	}
	if m.EditMode != editExistingKey {
		t.Fatalf("EditMode = %v, want editExistingKey", m.EditMode)
	}
	if m.editExistingKeyNeedsFullScreen() {
		t.Fatal("small body should not need full-screen layout")
	}
}

func TestKeyPanelEditUsesFullScreenWhenBodyLarge(t *testing.T) {
	m := New()
	m.Width = 120
	m.Height = 24
	m.PanelFocus = panelKeys
	m.SelectedKey = "demo:big"
	m.Client = &store.Client{}
	m.KeyDetail = &store.KeyDetail{
		Meta:   store.KeyMeta{Key: "demo:big", Type: "string"},
		String: strings.Repeat("x", 5000),
	}

	next, _ := m.startEdit()
	m = next.(*Model)

	if m.PanelFocus != panelKeys {
		t.Fatalf("PanelFocus = %v, want panelKeys (no panel switch)", m.PanelFocus)
	}
	if m.EditMode != editExistingKey {
		t.Fatalf("EditMode = %v, want editExistingKey (no redirect to element edit)", m.EditMode)
	}
	if !m.editExistingKeyNeedsFullScreen() {
		t.Fatal("large body should need full-screen layout")
	}
	if got := m.NewKeyValue.Value(); got != m.KeyDetail.String {
		t.Fatalf("textarea missing chars: got %d, want %d", len(got), len(m.KeyDetail.String))
	}
}

func TestKeyPanelEditUsesFullScreenWhenBodyManyLines(t *testing.T) {
	m := New()
	m.Width = 120
	m.Height = 24
	m.PanelFocus = panelKeys
	m.SelectedKey = "demo:list"
	m.Client = &store.Client{}
	m.KeyDetail = &store.KeyDetail{
		Meta: store.KeyMeta{Key: "demo:list", Type: "list"},
		List: makeList(30),
	}

	next, _ := m.startEdit()
	m = next.(*Model)

	if m.EditMode != editExistingKey {
		t.Fatalf("EditMode = %v, want editExistingKey", m.EditMode)
	}
	if !m.editExistingKeyNeedsFullScreen() {
		t.Fatal("many-line body should need full-screen layout")
	}
}

func TestDetailPanelEditUsesElementEditor(t *testing.T) {
	m := New()
	m.Width = 120
	m.Height = 24
	m.PanelFocus = panelDetail
	m.SelectedKey = "demo:small"
	m.Client = &store.Client{}
	m.KeyDetail = &store.KeyDetail{
		Meta:   store.KeyMeta{Key: "demo:small", Type: "string"},
		String: "short value",
	}

	next, _ := m.startDetailEdit()
	m = next.(*Model)

	if m.EditMode != editElement {
		t.Fatalf("EditMode = %v, want editElement from detail panel", m.EditMode)
	}
}

func TestEditExistingKeyNeedsFullScreen(t *testing.T) {
	cases := []struct {
		name string
		m    *Model
		want bool
	}{
		{
			name: "non-existing-key mode",
			m:    &Model{EditMode: editNewKey, KeyDetail: &store.KeyDetail{}},
			want: false,
		},
		{
			name: "existing-key with small body",
			m: &Model{
				EditMode:  editExistingKey,
				KeyDetail: &store.KeyDetail{Meta: store.KeyMeta{Type: "string"}, String: "tiny"},
			},
			want: false,
		},
		{
			name: "existing-key with large body",
			m: &Model{
				EditMode: editExistingKey,
				KeyDetail: &store.KeyDetail{
					Meta:   store.KeyMeta{Type: "string"},
					String: strings.Repeat("a", 5000),
				},
			},
			want: true,
		},
		{
			name: "existing-key with many lines",
			m: &Model{
				EditMode: editExistingKey,
				KeyDetail: &store.KeyDetail{
					Meta:  store.KeyMeta{Type: "list"},
					List:  makeList(20),
				},
			},
			want: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.m.editExistingKeyNeedsFullScreen(); got != tc.want {
				t.Fatalf("editExistingKeyNeedsFullScreen = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestRenderKeyEditFullScreenHasAllFields(t *testing.T) {
	m := New()
	m.Width = 120
	m.Height = 24
	m.PanelFocus = panelKeys
	m.SelectedKey = "demo:big"
	m.Client = &store.Client{}
	m.KeyDetail = &store.KeyDetail{
		Meta:   store.KeyMeta{Key: "demo:big", Type: "string"},
		String: strings.Repeat("x", 5000),
	}
	if _, cmd := m.startEdit(); cmd != nil {
		_ = cmd
	}
	m.syncNewKeyLayout()

	out := m.renderKeyEditFullScreen()
	for _, marker := range []string{"Edit key", "TTL:", "Type:", "Key:", "Value:"} {
		if !strings.Contains(out, marker) {
			t.Fatalf("full-screen edit missing %q in:\n%s", marker, out)
		}
	}
}

func TestViewKeyEditDispatchesFullScreenForLargeBody(t *testing.T) {
	m := New()
	m.Width = 120
	m.Height = 24
	m.PanelFocus = panelKeys
	m.Screen = ScreenKeyEdit
	m.SelectedKey = "demo:big"
	m.Client = &store.Client{}
	m.KeyDetail = &store.KeyDetail{
		Meta:   store.KeyMeta{Key: "demo:big", Type: "string"},
		String: strings.Repeat("x", 5000),
	}
	if _, cmd := m.startEdit(); cmd != nil {
		_ = cmd
	}

	out := m.View()
	if !strings.Contains(out, "Edit key") {
		t.Fatalf("view should render full-screen editor with 'Edit key' title, got:\n%s", out)
	}
	if !strings.Contains(out, "Type:") {
		t.Fatalf("view should expose Type field for full-screen edit, got:\n%s", out)
	}
}

func TestKeyFormModalPutsTypeBeforeTTL(t *testing.T) {
	m := New()
	m.Width = 120
	m.Height = 24
	m.Client = &store.Client{}
	m.SelectedKey = "demo:key"
	m.KeyDetail = &store.KeyDetail{
		Meta:   store.KeyMeta{Key: "demo:key", Type: "string", TTL: 300 * time.Second},
		String: "hi",
	}
	m.EditMode = editExistingKey
	m.NewKeyFocus = newKeyFieldTTL
	if _, cmd := m.startEdit(); cmd != nil {
		_ = cmd
	}
	m.syncNewKeyLayout()

	out := m.renderKeyFormModal()
	typeIdx := strings.Index(out, "Type:")
	ttlIdx := strings.Index(out, "TTL:")
	if typeIdx < 0 || ttlIdx < 0 {
		t.Fatalf("modal missing Type or TTL marker; out:\n%s", out)
	}
	if typeIdx > ttlIdx {
		t.Fatalf("expected Type before TTL, got Type@%d TTL@%d\n%s", typeIdx, ttlIdx, out)
	}
}

func TestKeyEditFullScreenPutsTypeBeforeTTL(t *testing.T) {
	m := New()
	m.Width = 120
	m.Height = 24
	m.Client = &store.Client{}
	m.SelectedKey = "demo:big"
	m.KeyDetail = &store.KeyDetail{
		Meta:   store.KeyMeta{Key: "demo:big", Type: "string", TTL: 300 * time.Second},
		String: strings.Repeat("x", 5000),
	}
	m.EditMode = editExistingKey
	m.NewKeyFocus = newKeyFieldTTL
	if _, cmd := m.startEdit(); cmd != nil {
		_ = cmd
	}
	m.syncNewKeyLayout()

	out := m.renderKeyEditFullScreen()
	typeIdx := strings.Index(out, "Type:")
	ttlIdx := strings.Index(out, "TTL:")
	if typeIdx < 0 || ttlIdx < 0 {
		t.Fatalf("full-screen missing Type or TTL marker; out:\n%s", out)
	}
	if typeIdx > ttlIdx {
		t.Fatalf("expected Type before TTL, got Type@%d TTL@%d\n%s", typeIdx, ttlIdx, out)
	}
}

func TestFocusNewKeyValueMovesCursorToStartForMultiLine(t *testing.T) {
	m := New()
	m.NewKeyValue.SetValue(strings.Repeat("a\n", 30))
	if line := m.NewKeyValue.Line(); line == 0 {
		t.Fatalf("SetValue should leave cursor on last line, got row %d", line)
	}

	cmd := m.focusNewKeyField(newKeyFieldValue)
	if cmd == nil {
		t.Fatal("expected focus command batch")
	}
	if line := m.NewKeyValue.Line(); line != 0 {
		t.Fatalf("expected cursor on row 0 after focus, got %d", line)
	}
}

func TestFocusNewKeyValueMovesCursorToStartForSingleLineLong(t *testing.T) {
	m := New()
	long := strings.Repeat("x", 5000)
	m.NewKeyValue.SetValue(long)
	if line := m.NewKeyValue.Line(); line != 0 {
		t.Fatalf("single-line value should keep cursor on row 0, got %d", line)
	}
	beforeLen := m.NewKeyValue.Length()
	if beforeLen != 5000 {
		t.Fatalf("textarea length = %d, want 5000", beforeLen)
	}

	cmd := m.focusNewKeyField(newKeyFieldValue)
	if cmd == nil {
		t.Fatal("expected focus command batch")
	}
	if line := m.NewKeyValue.Line(); line != 0 {
		t.Fatalf("expected cursor on row 0 after focus, got %d", line)
	}
	if m.NewKeyValue.Length() != 5000 {
		t.Fatalf("focus must not change value length, got %d", m.NewKeyValue.Length())
	}
}

func TestKeyFormModalFieldOrder(t *testing.T) {
	m := New()
	m.Width = 120
	m.Height = 24
	m.Client = &store.Client{}
	m.SelectedKey = "demo:key"
	m.KeyDetail = &store.KeyDetail{
		Meta:   store.KeyMeta{Key: "demo:key", Type: "string", TTL: 300 * time.Second},
		String: "hi",
	}
	m.EditMode = editExistingKey
	m.NewKeyFocus = newKeyFieldTTL
	if _, cmd := m.startEdit(); cmd != nil {
		_ = cmd
	}
	m.syncNewKeyLayout()

	out := m.renderKeyFormModal()
	typeIdx := strings.Index(out, "Type:")
	keyIdx := strings.Index(out, "Key:")
	ttlIdx := strings.Index(out, "TTL:")
	valueIdx := strings.Index(out, "Value:")
	if typeIdx < 0 || keyIdx < 0 || ttlIdx < 0 || valueIdx < 0 {
		t.Fatalf("modal missing one of Type/Key/TTL/Value markers; out:\n%s", out)
	}
	if !(typeIdx < keyIdx && keyIdx < ttlIdx && ttlIdx < valueIdx) {
		t.Fatalf("expected order Type < Key < TTL < Value, got Type@%d Key@%d TTL@%d Value@%d\n%s",
			typeIdx, keyIdx, ttlIdx, valueIdx, out)
	}
}

func TestKeyEditFullScreenFieldOrder(t *testing.T) {
	m := New()
	m.Width = 120
	m.Height = 24
	m.Client = &store.Client{}
	m.SelectedKey = "demo:big"
	m.KeyDetail = &store.KeyDetail{
		Meta:   store.KeyMeta{Key: "demo:big", Type: "string", TTL: 300 * time.Second},
		String: strings.Repeat("x", 5000),
	}
	m.EditMode = editExistingKey
	m.NewKeyFocus = newKeyFieldTTL
	if _, cmd := m.startEdit(); cmd != nil {
		_ = cmd
	}
	m.syncNewKeyLayout()

	out := m.renderKeyEditFullScreen()
	typeIdx := strings.Index(out, "Type:")
	keyIdx := strings.Index(out, "Key:")
	ttlIdx := strings.Index(out, "TTL:")
	valueIdx := strings.Index(out, "Value:")
	if typeIdx < 0 || keyIdx < 0 || ttlIdx < 0 || valueIdx < 0 {
		t.Fatalf("full-screen missing one of Type/Key/TTL/Value markers; out:\n%s", out)
	}
	if !(typeIdx < keyIdx && keyIdx < ttlIdx && ttlIdx < valueIdx) {
		t.Fatalf("expected order Type < Key < TTL < Value, got Type@%d Key@%d TTL@%d Value@%d\n%s",
			typeIdx, keyIdx, ttlIdx, valueIdx, out)
	}
}

func TestKeyFormFieldOrderPerMode(t *testing.T) {
	cases := []struct {
		mode editMode
		want []int
	}{
		{editNewKey, []int{newKeyFieldType, newKeyFieldKey, newKeyFieldTTL, newKeyFieldValue}},
		{editExistingKey, []int{newKeyFieldKey, newKeyFieldTTL, newKeyFieldValue}},
	}
	for _, tc := range cases {
		m := &Model{EditMode: tc.mode}
		order := m.keyFormFieldOrder()
		if len(order) != len(tc.want) {
			t.Fatalf("mode %v: order length = %d, want %d (order=%v)", tc.mode, len(order), len(tc.want), order)
		}
		for i := range tc.want {
			if order[i] != tc.want[i] {
				t.Fatalf("mode %v: order[%d] = %d, want %d (order=%v)", tc.mode, i, order[i], tc.want[i], order)
			}
		}
	}
}
