package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/bloodynite/lazyredis/internal/store"
)

// captureLoadDetail replaces loadKeyDetailFn / loadKeySummaryFn with
// recorders so tests can inspect what offset/limit the model requested
// without invoking real Redis commands. It restores the originals on
// test cleanup.
func captureLoadDetail(t *testing.T) (detailRec *[]keyDetailMsg, summaryRec *[]keySummaryMsg) {
	t.Helper()
	dRec := &[]keyDetailMsg{}
	sRec := &[]keySummaryMsg{}
	origDetail := loadKeyDetailFn
	origSummary := loadKeySummaryFn
	loadKeyDetailFn = func(client *store.Client, key string, offset, limit int, gen uint64, chunk bool) tea.Cmd {
		*dRec = append(*dRec, keyDetailMsg{key: key, gen: gen, chunk: chunk, appendOff: offset, appendLimit: limit})
		return nil
	}
	loadKeySummaryFn = func(client *store.Client, key string, gen uint64) tea.Cmd {
		*sRec = append(*sRec, keySummaryMsg{key: key, gen: gen})
		return nil
	}
	t.Cleanup(func() {
		loadKeyDetailFn = origDetail
		loadKeySummaryFn = origSummary
	})
	return dRec, sRec
}

// TestKeySummaryMessageRoutesToFullLoad: a summary with Total <= chunk
// must trigger a full GET (offset=-1, limit=0).
func TestKeySummaryMessageRoutesToFullLoad(t *testing.T) {
	dRec, _ := captureLoadDetail(t)
	m := New()
	m.Width = 120
	m.Height = 40
	m.Screen = ScreenBrowser
	m.Client = &store.Client{}
	m.SelectedKey = "small"
	m.detailGen = 1
	m.detailGen++

	next, _ := m.Update(keySummaryMsg{
		summary: &store.KeySummary{Meta: store.KeyMeta{Key: "small", Type: "hash"}, Total: 10},
		key:     "small",
		gen:     m.detailGen,
	})
	m = next.(*Model)
	if m.DetailTotal != 10 {
		t.Fatalf("DetailTotal=%d, want 10", m.DetailTotal)
	}
	if len(*dRec) != 1 {
		t.Fatalf("detail requests=%d, want 1", len(*dRec))
	}
	got := (*dRec)[0]
	if got.appendOff != -1 || got.appendLimit != 0 {
		t.Fatalf("full-load offset/limit = (%d,%d), want (-1,0)", got.appendOff, got.appendLimit)
	}
	if got.chunk {
		t.Fatal("first request must not be marked chunk=true")
	}
}

// TestKeySummaryMessageRoutesToChunk: Total > chunk triggers first
// windowed chunk (offset=0, limit=chunkSize).
func TestKeySummaryMessageRoutesToChunk(t *testing.T) {
	dRec, _ := captureLoadDetail(t)
	m := New()
	m.Width = 120
	m.Height = 40
	m.Screen = ScreenBrowser
	m.Client = &store.Client{}
	m.SelectedKey = "big"
	m.detailGen = 1
	m.detailGen++

	next, _ := m.Update(keySummaryMsg{
		summary: &store.KeySummary{Meta: store.KeyMeta{Key: "big", Type: "hash"}, Total: int64(detailChunkSize + 100)},
		key:     "big",
		gen:     m.detailGen,
	})
	m = next.(*Model)
	if m.DetailTotal != int64(detailChunkSize+100) {
		t.Fatalf("DetailTotal=%d", m.DetailTotal)
	}
	if len(*dRec) != 1 {
		t.Fatalf("detail requests=%d, want 1", len(*dRec))
	}
	got := (*dRec)[0]
	if got.appendOff != 0 || got.appendLimit != detailChunkSize {
		t.Fatalf("chunk = (%d,%d), want (0,%d)", got.appendOff, got.appendLimit, detailChunkSize)
	}
	if got.chunk {
		t.Fatal("first chunk must not be flagged for merge")
	}
}

// TestKeySummaryStaleDropped: stale generation summary must not
// produce a follow-up request.
func TestKeySummaryStaleDropped(t *testing.T) {
	dRec, _ := captureLoadDetail(t)
	m := New()
	m.Screen = ScreenBrowser
	m.Client = &store.Client{}
	m.SelectedKey = "k"
	m.detailGen = 5

	next, _ := m.Update(keySummaryMsg{
		summary: &store.KeySummary{Meta: store.KeyMeta{Key: "old", Type: "string"}, Total: 100},
		key:     "old",
		gen:     1,
	})
	m = next.(*Model)
	if len(*dRec) != 0 {
		t.Fatalf("stale summary must not request detail, got %d", len(*dRec))
	}
	if m.DetailTotal != 0 {
		t.Fatalf("DetailTotal=%d, want 0", m.DetailTotal)
	}
}

// TestDetailChunkAppendsAndUpdatesLoaded: a chunk response is merged
// into the existing detail and DetailLoaded reflects the new total.
func TestDetailChunkAppendsAndUpdatesLoaded(t *testing.T) {
	m := New()
	m.Screen = ScreenBrowser
	m.Client = &store.Client{}
	m.SelectedKey = "k"
	m.detailGen = 1
	m.DetailTotal = int64(detailChunkSize + 50)
	m.KeyDetail = &store.KeyDetail{
		Meta: store.KeyMeta{Key: "k", Type: "list"},
		List: make([]string, detailChunkSize),
	}

	next, _ := m.Update(keyDetailMsg{
		detail: &store.KeyDetail{
			Meta: store.KeyMeta{Key: "k", Type: "list"},
			List: []string{"a", "b", "c"},
		},
		key:         "k",
		gen:         1,
		chunk:       true,
		appendOff:   detailChunkSize,
		appendLimit: detailChunkSize,
	})
	m = next.(*Model)
	want := detailChunkSize + 3
	if len(m.KeyDetail.List) != want {
		t.Fatalf("len(List)=%d, want %d", len(m.KeyDetail.List), want)
	}
	if m.DetailLoaded != want {
		t.Fatalf("DetailLoaded=%d, want %d", m.DetailLoaded, want)
	}
	if m.detailChunkPending {
		t.Fatal("chunk pending must clear after response")
	}
}

// TestMaybeLoadMoreDetailSkipsWhenCursorSafe: no chunk requested when
// the cursor is comfortably inside the loaded window.
func TestMaybeLoadMoreDetailSkipsWhenCursorSafe(t *testing.T) {
	m := New()
	m.Width = 120
	m.Height = 40
	m.Screen = ScreenBrowser
	m.Client = &store.Client{}
	m.SelectedKey = "k"
	m.KeyDetail = &store.KeyDetail{
		Meta: store.KeyMeta{Key: "k", Type: "list"},
		List: make([]string, detailChunkSize),
	}
	m.DetailTotal = int64(detailChunkSize * 3)
	m.DetailLoaded = detailChunkSize
	m.DetailCursor = 10

	if cmd := m.maybeLoadMoreDetail(); cmd != nil {
		t.Fatal("cursor far from end should not trigger chunk load")
	}
}

// TestMaybeLoadMoreDetailTriggersNearEnd: lookahead trigger fires when
// cursor crosses the lookahead boundary and dedupes parallel loads.
func TestMaybeLoadMoreDetailTriggersNearEnd(t *testing.T) {
	dRec, _ := captureLoadDetail(t)
	m := New()
	m.Width = 120
	m.Height = 40
	m.Screen = ScreenBrowser
	m.Client = &store.Client{}
	m.SelectedKey = "k"
	m.KeyDetail = &store.KeyDetail{
		Meta: store.KeyMeta{Key: "k", Type: "list"},
		List: make([]string, detailChunkSize),
	}
	m.DetailTotal = int64(detailChunkSize * 3)
	m.DetailLoaded = detailChunkSize
	m.DetailCursor = detailChunkSize - detailChunkLookahead + 1

	// sanity: maybeLoadMoreDetail relies on DetailLoaded for the boundary
	if m.DetailLoaded != detailChunkSize {
		t.Fatalf("setup: DetailLoaded=%d, want %d", m.DetailLoaded, detailChunkSize)
	}
	if m.DetailCursor <= m.DetailLoaded-detailChunkLookahead {
		t.Fatalf("setup: cursor=%d not past lookahead boundary %d", m.DetailCursor, m.DetailLoaded-detailChunkLookahead)
	}

	cmd := m.maybeLoadMoreDetail()
	if len(*dRec) == 0 {
		t.Fatal("expected chunk load command near end of window")
	}
	_ = cmd
	if !m.detailChunkPending {
		t.Fatal("chunk pending must be set to dedupe parallel loads")
	}
	if len(*dRec) != 1 {
		t.Fatalf("detail requests=%d, want 1", len(*dRec))
	}
	got := (*dRec)[0]
	if !got.chunk {
		t.Fatal("follow-up load must be marked chunk=true")
	}
	if got.appendOff != detailChunkSize {
		t.Fatalf("chunk offset=%d, want %d", got.appendOff, detailChunkSize)
	}
	// Second call must dedupe.
	if cmd := m.maybeLoadMoreDetail(); cmd != nil {
		t.Fatal("second call while pending must dedupe")
	}
}

// TestMaybeLoadMoreDetailStopsAtTotal: must not request more chunks
// when DetailLoaded == DetailTotal.
func TestMaybeLoadMoreDetailStopsAtTotal(t *testing.T) {
	dRec, _ := captureLoadDetail(t)
	m := New()
	m.Width = 120
	m.Height = 40
	m.Screen = ScreenBrowser
	m.Client = &store.Client{}
	m.SelectedKey = "k"
	m.KeyDetail = &store.KeyDetail{
		Meta: store.KeyMeta{Key: "k", Type: "list"},
		List: make([]string, 50),
	}
	m.DetailTotal = 50
	m.DetailLoaded = 50
	m.DetailCursor = 49

	if cmd := m.maybeLoadMoreDetail(); cmd != nil {
		t.Fatal("must not request more chunks when fully loaded")
	}
	if len(*dRec) != 0 {
		t.Fatalf("detail requests=%d, want 0", len(*dRec))
	}
}

// TestStringSummaryGoesFullLoad: strings bypass the chunk flow and
// request a full GET (offset=-1, limit=0) regardless of byte length.
func TestStringSummaryGoesFullLoad(t *testing.T) {
	dRec, _ := captureLoadDetail(t)
	m := New()
	m.Width = 120
	m.Height = 40
	m.Screen = ScreenBrowser
	m.Client = &store.Client{}
	m.SelectedKey = "k"
	m.detailGen = 1
	m.detailGen++

	next, _ := m.Update(keySummaryMsg{
		summary: &store.KeySummary{Meta: store.KeyMeta{Key: "k", Type: "string"}, Total: 1024 * 1024},
		key:     "k",
		gen:     m.detailGen,
	})
	m = next.(*Model)
	if len(*dRec) != 1 {
		t.Fatalf("detail requests=%d, want 1", len(*dRec))
	}
	got := (*dRec)[0]
	if got.appendOff != -1 || got.appendLimit != 0 {
		t.Fatalf("string must request full load, got off=%d lim=%d", got.appendOff, got.appendLimit)
	}
	_ = next
}

// TestHashChunkMerge: unordered hash merge preserves every field
// without losing entries or overrides.
func TestHashChunkMerge(t *testing.T) {
	dst := &store.KeyDetail{
		Meta: store.KeyMeta{Key: "k", Type: "hash"},
		Hash: map[string]string{"a": "1", "b": "2"},
	}
	src := &store.KeyDetail{
		Meta: store.KeyMeta{Key: "k", Type: "hash"},
		Hash: map[string]string{"c": "3", "b": "OVERRIDDEN"},
	}
	mergeChunkIntoDetail(dst, src, 2)
	if dst.Hash["a"] != "1" || dst.Hash["c"] != "3" {
		t.Fatalf("merged hash missing entries: %v", dst.Hash)
	}
	if dst.Hash["b"] != "OVERRIDDEN" {
		t.Fatalf("hash override lost: b=%q", dst.Hash["b"])
	}
}

// TestPaginationConstants: chunk size and lookahead are sensible.
func TestPaginationConstants(t *testing.T) {
	if detailChunkSize < 50 {
		t.Fatalf("detailChunkSize=%d too small to fill viewport", detailChunkSize)
	}
	if detailChunkSize > 1000 {
		t.Fatalf("detailChunkSize=%d too large, memory pressure", detailChunkSize)
	}
	if detailChunkLookahead >= detailChunkSize {
		t.Fatal("lookahead must be smaller than chunk size")
	}
}
