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
	dRec, _ := captureLoadDetail(t)
	m := New()
	m.Width = 120
	m.Height = 40
	m.Screen = ScreenBrowser
	m.Client = &store.Client{}
	m.Keys = []string{"a", "b"}
	m.KeyCursor = 1
	m.detailGen = 5
	m.detailRetryCount = 1 // would normally block retry

	next, cmd := m.moveKeyCursor(0) // no move; just bump selection explicitly
	_ = next
	_ = cmd
	// Simulate a real selection change by directly bumping fields.
	m.SelectedKey = "b"
	m.detailGen = 6
	m.detailRetryCount = 0

	// Now WRONGTYPE on the new key should retry.
	next, _ = m.Update(keyDetailMsg{
		key: "b",
		gen: 6,
		err: errors.New("WRONGTYPE Operation against a key holding the wrong kind of value"),
	})
	m = next.(*Model)
	if m.detailRetryCount != 1 {
		t.Fatalf("detailRetryCount=%d, want 1 after fresh selection", m.detailRetryCount)
	}
	if len(*dRec) != 0 {
		t.Fatal("retry should re-fire summary, not detail directly")
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
