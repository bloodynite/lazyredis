package tui

import (
	"testing"
	"time"
)

func TestRefreshBarEmpty(t *testing.T) {
	bar := refreshBar(0, 10)
	want := "▢▢▢▢▢▢▢▢▢▢"
	if bar != want {
		t.Fatalf("bar = %q, want %q", bar, want)
	}
}

func TestRefreshBarHalf(t *testing.T) {
	bar := refreshBar(5*time.Second, 10)
	if bar != "▣▣▣▣▣▢▢▢▢▢" {
		t.Fatalf("bar = %q", bar)
	}
}

func TestRefreshBarFull(t *testing.T) {
	got := refreshBar(15*time.Second, 10)
	if got != "▣▣▣▣▣▣▣▣▣▣" {
		t.Fatalf("bar = %q", got)
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
		filled := 0
		for _, r := range bar {
			if r == '▣' {
				filled++
			}
		}
		if filled < prev {
			t.Fatalf("bar went backward at %s: prev=%d filled=%d bar=%q", d, prev, filled, bar)
		}
		prev = filled
	}
}

func TestRefreshBarClockSkew(t *testing.T) {
	bar := refreshBar(-5*time.Second, 10)
	if bar != "▢▢▢▢▢▢▢▢▢▢" {
		t.Fatalf("negative elapsed should clamp to 0, got %q", bar)
	}
}

func TestRefreshBarDisabled(t *testing.T) {
	bar := refreshBar(5*time.Second, 0)
	if bar != "▢▢▢▢▢▢▢▢▢▢" {
		t.Fatalf("disabled interval should render all empty, got %q", bar)
	}
}
