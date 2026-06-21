package tui

import (
	"strings"
	"testing"

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