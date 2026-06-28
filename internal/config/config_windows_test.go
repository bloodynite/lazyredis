//go:build windows

package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDirUsesUserConfigDirOnWindows(t *testing.T) {
	expectedBase, err := os.UserConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	got, err := Dir()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(expectedBase, "lazyredis")
	if got != want {
		t.Fatalf("dir = %q, want %q", got, want)
	}
}

func TestDirOnWindowsIsNotHomeDotConfig(t *testing.T) {
	got, err := Dir()
	if err != nil {
		t.Fatal(err)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	posixStyle := filepath.Join(home, ".config", "lazyredis")
	if strings.EqualFold(got, posixStyle) {
		t.Fatalf("expected Windows-native config dir, got POSIX-style %q", got)
	}
}
