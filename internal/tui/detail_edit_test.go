package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
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
