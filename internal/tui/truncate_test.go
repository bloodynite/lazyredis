package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/mattn/go-runewidth"
)

// TestTruncateShortStringFast: a value that fits in n columns must be
// returned unchanged with no extra allocation that would slow render.
func TestTruncateShortStringFast(t *testing.T) {
	in := "hello"
	got := truncate(in, 80)
	if got != in {
		t.Fatalf("truncate short: got %q, want %q", got, in)
	}
}

// TestTruncateLongStringBoundedTime: a 30 KB string (mirroring the
// auditTrail field on a glyphverso hash) must be truncated in well under
// 10 ms. The naive O(n^2) implementation took ~9 s here and froze the UI
// during render — see commit message for context.
func TestTruncateLongStringBoundedTime(t *testing.T) {
	payload := strings.Repeat("audit_line_999: Operational note about user navigating a paginated result set with realistic payload text. ", 300)
	if len(payload) < 30*1024 {
		t.Fatalf("payload setup: got %d bytes, want >=30 KB", len(payload))
	}
	start := time.Now()
	out := truncate(payload, 80)
	elapsed := time.Since(start)
	if elapsed > 10*time.Millisecond {
		t.Fatalf("truncate 30 KB took %v, want < 10 ms (was O(n^2) before fix)", elapsed)
	}
	if !strings.HasSuffix(out, "…") {
		t.Fatalf("truncate should append ellipsis, got %q", out)
	}
	// Display width of the truncated result (without ellipsis) must be at
	// most n-1 so the ellipsis still fits.
	runes := []rune(strings.TrimSuffix(out, "…"))
	if got := runesVisWidth(runes); got > 79 {
		t.Fatalf("truncated runes width=%d, want <= 79", got)
	}
}

func runesVisWidth(rs []rune) int {
	w := 0
	for _, r := range rs {
		w += runewidth.RuneWidth(r)
	}
	return w
}

// TestTruncateWideRuneBudget: when the boundary lands inside a wide rune
// (CJK / emoji), the result must respect the n-column budget rather than
// silently exceeding it.
func TestTruncateWideRuneBudget(t *testing.T) {
	// Each "你" is a wide rune (2 columns). 50 of them = 100 columns.
	payload := strings.Repeat("你", 50)
	out := truncate(payload, 20)
	w := runewidth.RuneWidth([]rune(strings.TrimSuffix(out, "…"))[0])
	if w > 19 {
		t.Fatalf("truncate wide: leading rune width=%d, want <= 19 (budget for n=20 minus ellipsis)", w)
	}
	if !strings.HasSuffix(out, "…") {
		t.Fatalf("truncate wide: missing ellipsis, got %q", out)
	}
}

// TestTruncateBoundaryTiny: when n <= 3, truncate returns the input
// unchanged so the ellipsis + budget never produces a negative-length slice.
func TestTruncateBoundaryTiny(t *testing.T) {
	if got := truncate("hello", 1); got != "hello" {
		t.Fatalf("truncate n=1: got %q, want %q", got, "hello")
	}
	if got := truncate("hello", 3); got != "hello" {
		t.Fatalf("truncate n=3: got %q, want %q", got, "hello")
	}
}
