package tui

import (
	"testing"
	"time"
	"unicode/utf8"
)

func TestRefreshBarEmpty(t *testing.T) {
	bar := refreshBar(0, 10, 0)
	want := "▢▢▢▢▢▢▢▢▢▢"
	if bar != want {
		t.Fatalf("bar = %q, want %q", bar, want)
	}
}

func TestRefreshBarHalf(t *testing.T) {
	bar := refreshBar(5*time.Second, 10, 0)
	if bar != "▣▣▣▣▣▣▢▢▢▢" {
		t.Fatalf("bar = %q", bar)
	}
}

func TestRefreshBarFull(t *testing.T) {
	got0 := refreshBar(15*time.Second, 10, 0)
	got1 := refreshBar(15*time.Second, 10, 1)
	if got0 != "▣▣▣▣▣▣▣▣▣▣" {
		t.Fatalf("frame0 = %q", got0)
	}
	if got1 != "▣▣▣▣▣▣▣▣▣▢" {
		t.Fatalf("frame1 = %q", got1)
	}
}

func TestRefreshBarBoundaryAnimates(t *testing.T) {
	got0 := refreshBar(3*time.Second, 10, 0)
	got1 := refreshBar(3*time.Second, 10, 1)
	got2 := refreshBar(3*time.Second, 10, 2)
	rb0, _ := utf8.DecodeRuneInString(got0[3*3:])
	rb1, _ := utf8.DecodeRuneInString(got1[3*3:])
	rb2, _ := utf8.DecodeRuneInString(got2[3*3:])
	if rb0 != rb2 {
		t.Fatalf("frame parity should match: frame0=%c frame2=%c", rb0, rb2)
	}
	if rb0 == rb1 {
		t.Fatalf("boundary must toggle between frames 0 and 1, both = %c", rb0)
	}
}

func TestRefreshBarClockSkew(t *testing.T) {
	bar := refreshBar(-5*time.Second, 10, 0)
	if bar != "▢▢▢▢▢▢▢▢▢▢" {
		t.Fatalf("negative elapsed should clamp to 0, got %q", bar)
	}
}

func TestRefreshBarDisabled(t *testing.T) {
	bar := refreshBar(5*time.Second, 0, 0)
	if bar != "▢▢▢▢▢▢▢▢▢▢" {
		t.Fatalf("disabled interval should render all empty, got %q", bar)
	}
}