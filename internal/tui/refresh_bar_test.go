package tui

import (
	"strings"
	"testing"
	"time"
)

const (
	refreshBarFilled = "■"
	refreshBarEmpty  = "□"
)

func refreshBarExpectedWidth(intervalSec int) int {
	return barCells(intervalSec)
}

func TestBarCells(t *testing.T) {
	cases := []struct {
		intervalSec int
		want        int
	}{
		{0, 10},
		{1, 10},
		{5, 10},
		{6, 10},
		{10, 10},
		{11, 10},
		{30, 10},
		{60, 10},
		{120, 10},
	}
	for _, tc := range cases {
		if got := barCells(tc.intervalSec); got != tc.want {
			t.Errorf("barCells(%d) = %d, want %d", tc.intervalSec, got, tc.want)
		}
	}
}

func TestRefreshBarEmpty(t *testing.T) {
	bar := refreshBar(0, 10)
	width := refreshBarExpectedWidth(10)
	want := strings.Repeat(refreshBarEmpty, width)
	if bar != want {
		t.Fatalf("bar = %q, want %q", bar, want)
	}
}

func TestRefreshBarHalf(t *testing.T) {
	bar := refreshBar(5*time.Second, 10)
	width := refreshBarExpectedWidth(10)
	want := strings.Repeat(refreshBarFilled, width/2) + strings.Repeat(refreshBarEmpty, width-width/2)
	if bar != want {
		t.Fatalf("bar = %q, want %q", bar, want)
	}
}

func TestRefreshBarFull(t *testing.T) {
	got := refreshBar(15*time.Second, 10)
	width := refreshBarExpectedWidth(10)
	want := strings.Repeat(refreshBarFilled, width)
	if got != want {
		t.Fatalf("got = %q, want %q", got, want)
	}
}

func TestRefreshBarNearFull(t *testing.T) {
	got := refreshBar(9*time.Second, 10)
	width := refreshBarExpectedWidth(10)
	want := strings.Repeat(refreshBarFilled, width-1) + strings.Repeat(refreshBarEmpty, 1)
	if got != want {
		t.Fatalf("got = %q, want %q", got, want)
	}
}

func TestRefreshBarJustBelowReset(t *testing.T) {
	got := refreshBar(9999*time.Millisecond, 10)
	width := refreshBarExpectedWidth(10)
	want := strings.Repeat(refreshBarFilled, width)
	if got != want {
		t.Fatalf("got = %q, want %q", got, want)
	}
}

func TestRefreshBarNoSnapJump(t *testing.T) {
	width := refreshBarExpectedWidth(10)
	nearFull := strings.Count(refreshBar(9*time.Second, 10), refreshBarFilled)
	justBefore := strings.Count(refreshBar(8999*time.Millisecond, 10), refreshBarFilled)
	if nearFull-justBefore > 1 {
		t.Fatalf("snap-jump regression: filled(9s)=%d filled(8.999s)=%d (width=%d)",
			nearFull, justBefore, width)
	}
}

func TestRefreshBarExactInterval(t *testing.T) {
	got := refreshBar(10*time.Second, 10)
	width := refreshBarExpectedWidth(10)
	want := strings.Repeat(refreshBarFilled, width)
	if got != want {
		t.Fatalf("got = %q, want %q", got, want)
	}
}

func TestRefreshBarWidthInvariant(t *testing.T) {
	cases := []struct {
		name        string
		elapsed     time.Duration
		intervalSec int
	}{
		{"zero", 0, 10},
		{"quarter", 2500 * time.Millisecond, 10},
		{"half", 5 * time.Second, 10},
		{"near-full", 9 * time.Second, 10},
		{"full", 10 * time.Second, 10},
		{"clamped-overflow", 30 * time.Second, 10},
		{"disabled", 5 * time.Second, 0},
		{"negative", -5 * time.Second, 10},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			bar := refreshBar(tc.elapsed, tc.intervalSec)
			width := refreshBarExpectedWidth(tc.intervalSec)
			filled := strings.Count(bar, refreshBarFilled)
			empty := strings.Count(bar, refreshBarEmpty)
			if filled+empty != width {
				t.Fatalf("bar width = filled(%d)+empty(%d) = %d, want %d (bar=%q)",
					filled, empty, filled+empty, width, bar)
			}
		})
	}
}

func TestRefreshBarIsMonotonic(t *testing.T) {
	samples := []time.Duration{
		0,
		1 * time.Second,
		3 * time.Second,
		5 * time.Second,
		7 * time.Second,
		9 * time.Second,
		10 * time.Second,
		15 * time.Second,
	}
	prev := 0
	for _, d := range samples {
		bar := refreshBar(d, 10)
		filled := strings.Count(bar, refreshBarFilled)
		if filled < prev {
			t.Fatalf("bar went backward at %s: prev=%d filled=%d bar=%q", d, prev, filled, bar)
		}
		prev = filled
	}
}

func TestRefreshBarClockSkew(t *testing.T) {
	bar := refreshBar(-5*time.Second, 10)
	width := refreshBarExpectedWidth(10)
	want := strings.Repeat(refreshBarEmpty, width)
	if bar != want {
		t.Fatalf("negative elapsed should clamp to 0, got %q", bar)
	}
}

func TestRefreshBarDisabled(t *testing.T) {
	bar := refreshBar(5*time.Second, 0)
	width := refreshBarExpectedWidth(0)
	want := strings.Repeat(refreshBarEmpty, width)
	if bar != want {
		t.Fatalf("disabled interval should render all empty, got %q", bar)
	}
}

func TestRefreshBarDynamicWidth(t *testing.T) {
	cases := []struct {
		name         string
		intervalSec  int
		wantWidth    int
		halfElapsed  time.Duration
		halfFilled   int
		fullFilled   int
	}{
		{name: "5s", intervalSec: 5, wantWidth: 10, halfElapsed: 2500 * time.Millisecond, halfFilled: 5, fullFilled: 10},
		{name: "10s", intervalSec: 10, wantWidth: 10, halfElapsed: 5 * time.Second, halfFilled: 5, fullFilled: 10},
		{name: "30s", intervalSec: 30, wantWidth: 10, halfElapsed: 15 * time.Second, halfFilled: 5, fullFilled: 10},
		{name: "60s", intervalSec: 60, wantWidth: 10, halfElapsed: 30 * time.Second, halfFilled: 5, fullFilled: 10},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := barCells(tc.intervalSec); got != tc.wantWidth {
				t.Fatalf("barCells(%d) = %d, want %d", tc.intervalSec, got, tc.wantWidth)
			}
			empty := refreshBar(0, tc.intervalSec)
			if got := strings.Count(empty, refreshBarEmpty); got != tc.wantWidth {
				t.Fatalf("empty bar width = %d, want %d (bar=%q)", got, tc.wantWidth, empty)
			}
			half := refreshBar(tc.halfElapsed, tc.intervalSec)
			if got := strings.Count(half, refreshBarFilled); got != tc.halfFilled {
				t.Fatalf("half-filled count = %d, want %d (bar=%q)", got, tc.halfFilled, half)
			}
			if got := strings.Count(half, refreshBarEmpty); got != tc.wantWidth-tc.halfFilled {
				t.Fatalf("half-filled empty count = %d, want %d (bar=%q)",
					got, tc.wantWidth-tc.halfFilled, half)
			}
			full := refreshBar(time.Duration(tc.intervalSec)*time.Second, tc.intervalSec)
			if got := strings.Count(full, refreshBarFilled); got != tc.fullFilled {
				t.Fatalf("full count = %d, want %d (bar=%q)", got, tc.fullFilled, full)
			}
			if got := strings.Count(full, refreshBarEmpty); got != 0 {
				t.Fatalf("full should have no empty cells, got %d (bar=%q)", got, full)
			}
		})
	}
}
