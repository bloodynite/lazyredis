//go:build darwin

package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDirUsesUserConfigDirOnDarwin(t *testing.T) {
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

func TestDirOnDarwinIsNotHomeDotConfig(t *testing.T) {
	got, err := Dir()
	if err != nil {
		t.Fatal(err)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	posixStyle := filepath.Join(home, ".config", "lazyredis")
	if got == posixStyle {
		t.Fatalf("expected Darwin-native config dir, got POSIX-style %q", got)
	}
}

func TestUserConfigDirOnDarwinIsApplicationSupport(t *testing.T) {
	base, err := os.UserConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(base, "Library/Application Support") {
		t.Fatalf("darwin UserConfigDir = %q, want path containing Library/Application Support", base)
	}
}