//go:build !windows

package tui

import (
	"errors"
	"runtime"
	"sync"
	"testing"

	"github.com/atotto/clipboard"
)

func TestClipboardChainInvokesPbcopyOnDarwin(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skipf("darwin-only (current GOOS=%s)", runtime.GOOS)
	}

	withForcedChain(t)

	rec := newRecorder()
	restore := swapClipboardBackends(t,
		rec.make("pbcopy"),
		rec.make("wl-copy"),
		rec.make("xclip"),
		rec.make("osc52"),
	)
	defer restore()

	if err := writeSystemClipboard("hello"); err != nil {
		t.Fatalf("writeSystemClipboard: %v", err)
	}

	calls := rec.calls()
	if len(calls) == 0 || calls[0] != "pbcopy" {
		t.Fatalf("expected first call to be pbcopy, got %v", calls)
	}
	if !containsCall(calls, "pbcopy") {
		t.Fatalf("expected pbcopy to be invoked on darwin, got %v", calls)
	}
	for _, c := range calls[1:] {
		if c == "wl-copy" || c == "xclip" {
			t.Fatalf("pbcopy succeeded; %q should not be invoked (calls=%v)", c, calls)
		}
	}
}

func TestClipboardChainSkipsPbcopyOnNonDarwin(t *testing.T) {
	if runtime.GOOS == "darwin" {
		t.Skipf("non-darwin only (current GOOS=%s)", runtime.GOOS)
	}

	withForcedChain(t)

	restore := swapClipboardBackends(t,
		func(string) error { return errors.New("pbcopy must not be called on non-darwin") },
		func(string) error { return nil },
		func(string) error { return nil },
		func(string) error { return nil },
	)
	defer restore()

	if err := writeSystemClipboard("hello"); err != nil {
		t.Fatalf("writeSystemClipboard: %v", err)
	}
}

func TestClipboardChainOrderOnNonDarwin(t *testing.T) {
	if runtime.GOOS == "darwin" {
		t.Skipf("non-darwin only (current GOOS=%s)", runtime.GOOS)
	}

	withForcedChain(t)

	rec := newRecorder()
	restore := swapClipboardBackends(t,
		func(string) error { return errors.New("pbcopy must not be called") },
		rec.make("wl-copy"),
		rec.make("xclip"),
		rec.make("osc52"),
	)
	defer restore()

	if err := writeSystemClipboard("hello"); err != nil {
		t.Fatalf("writeSystemClipboard: %v", err)
	}

	calls := rec.calls()
	if len(calls) == 0 || calls[0] != "wl-copy" {
		t.Fatalf("expected first call to be wl-copy on non-darwin, got %v", calls)
	}
	if containsCall(calls, "pbcopy") {
		t.Fatalf("pbcopy must not appear in calls on non-darwin, got %v", calls)
	}
}

func withForcedChain(t *testing.T) {
	t.Helper()
	orig := clipboard.Unsupported
	clipboard.Unsupported = true
	t.Cleanup(func() { clipboard.Unsupported = orig })
}

func swapClipboardBackends(t *testing.T, pbcopy, wl, xc, osc func(string) error) func() {
	t.Helper()
	origPbcopy := writeClipboardPbcopy
	origWl := writeClipboardWlCopy
	origXc := writeClipboardXClip
	origOsc := writeClipboardOSC52
	writeClipboardPbcopy = pbcopy
	writeClipboardWlCopy = wl
	writeClipboardXClip = xc
	writeClipboardOSC52 = osc
	return func() {
		writeClipboardPbcopy = origPbcopy
		writeClipboardWlCopy = origWl
		writeClipboardXClip = origXc
		writeClipboardOSC52 = origOsc
	}
}

type recorder struct {
	mu    sync.Mutex
	order []string
}

func newRecorder() *recorder {
	return &recorder{}
}

func (r *recorder) make(name string) func(string) error {
	return func(string) error {
		r.mu.Lock()
		r.order = append(r.order, name)
		r.mu.Unlock()
		return nil
	}
}

func (r *recorder) calls() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]string, len(r.order))
	copy(out, r.order)
	return out
}

func containsCall(calls []string, want string) bool {
	for _, c := range calls {
		if c == want {
			return true
		}
	}
	return false
}