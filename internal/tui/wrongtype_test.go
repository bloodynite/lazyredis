package tui

import (
	"errors"
	"testing"

	"github.com/bloodynite/lazyredis/internal/store"
)

// TestWRONGTYPETriggersSummaryRetry: a WRONGTYPE error on the first
// detail load must trigger exactly one summary retry with a fresh gen.
func TestWRONGTYPETriggersSummaryRetry(t *testing.T) {
	dRec, sRec := captureLoadDetail(t)
	m := New()
	m.Width = 120
	m.Height = 40
	m.Screen = ScreenBrowser
	m.Client = &store.Client{}
	m.SelectedKey = "k"
	m.detailGen = 1
	m.detailRetryCount = 0

	next, _ := m.Update(keyDetailMsg{
		key: "k",
		gen: 1,
		err: errors.New("WRONGTYPE Operation against a key holding the wrong kind of value"),
	})
	m = next.(*Model)
	if m.detailRetryCount != 1 {
		t.Fatalf("detailRetryCount=%d, want 1", m.detailRetryCount)
	}
	if m.detailGen != 2 {
		t.Fatalf("detailGen=%d, want 2 (must bump to invalidate prior in-flight)", m.detailGen)
	}
	if len(*sRec) != 1 {
		t.Fatalf("summary retries=%d, want 1", len(*sRec))
	}
	if (*sRec)[0].gen != 2 {
		t.Fatalf("summary retry gen=%d, want 2", (*sRec)[0].gen)
	}
	if (*sRec)[0].key != "k" {
		t.Fatalf("summary retry key=%q, want k", (*sRec)[0].key)
	}
	if len(*dRec) != 0 {
		t.Fatalf("detail should not be re-fired yet, got %d", len(*dRec))
	}
	if m.ErrMsg == "" {
		t.Fatal("user should see retrying message")
	}
}

// TestSecondWRONGTYPESurfacesError: a second WRONGTYPE on the same
// selection surfaces the error instead of looping forever.
func TestSecondWRONGTYPESurfacesError(t *testing.T) {
	_, sRec := captureLoadDetail(t)
	m := New()
	m.Width = 120
	m.Height = 40
	m.Screen = ScreenBrowser
	m.Client = &store.Client{}
	m.SelectedKey = "k"
	m.detailGen = 2
	m.detailRetryCount = 1 // already retried once

	next, _ := m.Update(keyDetailMsg{
		key: "k",
		gen: 2,
		err: errors.New("WRONGTYPE Operation against a key holding the wrong kind of value"),
	})
	m = next.(*Model)
	if len(*sRec) != 0 {
		t.Fatal("second WRONGTYPE must not retry")
	}
	if !contains(m.ErrMsg, "WRONGTYPE") {
		t.Fatalf("ErrMsg=%q, want it to contain WRONGTYPE", m.ErrMsg)
	}
	if m.detailRetryCount != 0 {
		t.Fatalf("retry counter must reset after surface, got %d", m.detailRetryCount)
	}
}

// TestNonRetriableErrorSurfacesImmediately: a non-WRONGTYPE error
// surfaces immediately without retrying.
func TestNonRetriableErrorSurfacesImmediately(t *testing.T) {
	dRec, sRec := captureLoadDetail(t)
	m := New()
	m.Width = 120
	m.Height = 40
	m.Screen = ScreenBrowser
	m.Client = &store.Client{}
	m.SelectedKey = "k"
	m.detailGen = 1

	next, _ := m.Update(keyDetailMsg{
		key: "k",
		gen: 1,
		err: errors.New("connection refused"),
	})
	m = next.(*Model)
	if len(*dRec) != 0 || len(*sRec) != 0 {
		t.Fatal("no further fetches expected")
	}
	if m.ErrMsg != "connection refused" {
		t.Fatalf("ErrMsg=%q", m.ErrMsg)
	}
}

// TestSelectionResetClearsRetryCount: changing selection resets the
// retry counter so the new key gets a fresh budget.
func TestSelectionResetClearsRetryCount(t *testing.T) {
	_, _ = captureLoadDetail(t)
	m := New()
	m.Width = 120
	m.Height = 40
	m.Screen = ScreenBrowser
	m.Client = &store.Client{}
	m.Keys = []string{"a", "b"}
	m.KeyCursor = 1
	m.detailGen = 5
	m.detailRetryCount = 1 // would normally block retry

	next, _ := m.moveKeyCursor(-1)
	m = next.(*Model)
	if m.KeyCursor != 0 {
		t.Fatalf("KeyCursor=%d, want 0", m.KeyCursor)
	}
	if m.SelectedKey != "a" {
		t.Fatalf("SelectedKey=%q, want a", m.SelectedKey)
	}
	if m.detailRetryCount != 0 {
		t.Fatalf("detailRetryCount=%d, want 0 after selection change", m.detailRetryCount)
	}
	if m.detailGen != 6 {
		t.Fatalf("detailGen=%d, want 6 (must bump on selection change)", m.detailGen)
	}

	// Now WRONGTYPE on the new key should retry because the budget was reset.
	next, _ = m.Update(keyDetailMsg{
		key: "a",
		gen: 6,
		err: errors.New("WRONGTYPE Operation against a key holding the wrong kind of value"),
	})
	m = next.(*Model)
	if m.detailRetryCount != 1 {
		t.Fatalf("detailRetryCount=%d, want 1 after fresh selection", m.detailRetryCount)
	}
}

// TestHandleDetailErrorArmsStatusClear: a non-retriable detail error
// must arm its own statusClearMsg timer so the error fades without
// being wiped by an unrelated path.
func TestHandleDetailErrorArmsStatusClear(t *testing.T) {
	_, _ = captureLoadDetail(t)
	m := New()
	m.Width = 120
	m.Height = 40
	m.Screen = ScreenBrowser
	m.Client = &store.Client{}
	m.SelectedKey = "k"
	m.detailGen = 1

	beforeGen := m.statusClearGen
	next, cmd := m.Update(keyDetailMsg{
		key: "k",
		gen: 1,
		err: errors.New("connection refused"),
	})
	m = next.(*Model)
	if cmd == nil {
		t.Fatal("expected non-nil clear cmd from non-retriable error")
	}
	if m.ErrMsg == "" {
		t.Fatal("ErrMsg should be set after error")
	}
	if m.statusClearGen <= beforeGen {
		t.Fatalf("statusClearGen must be bumped on ErrMsg, got %d (was %d)", m.statusClearGen, beforeGen)
	}

	// Firing the clear with the captured gen wipes ErrMsg.
	next, _ = m.Update(statusClearMsg{gen: m.statusClearGen})
	m = next.(*Model)
	if m.ErrMsg != "" {
		t.Fatalf("ErrMsg=%q, want empty after statusClearMsg", m.ErrMsg)
	}
}

// TestStatusClearMsgDirectlyClearsErrMsg: the F2 contract — any caller
// can hand a statusClearMsg and have ErrMsg cleared when the gen matches.
func TestStatusClearMsgDirectlyClearsErrMsg(t *testing.T) {
	m := New()
	m.ErrMsg = "boom"
	m.statusClearGen++
	gen := m.statusClearGen

	next, _ := m.Update(statusClearMsg{gen: gen})
	m = next.(*Model)
	if m.ErrMsg != "" {
		t.Fatalf("ErrMsg=%q, want empty after statusClearMsg", m.ErrMsg)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
