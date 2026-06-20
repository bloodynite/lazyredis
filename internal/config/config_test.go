package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	oldHome := os.Getenv("HOME")
	t.Setenv("HOME", dir)
	defer t.Setenv("HOME", oldHome)

	cfg := &File{
		Profiles: []Profile{
			{Name: "test", Mode: ModeStandalone, Addr: "127.0.0.1:6379", DB: 1},
		},
	}
	if err := Save(cfg); err != nil {
		t.Fatal(err)
	}

	path, err := Path()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected config file at %s: %v", path, err)
	}

	loaded, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Profiles) != 1 {
		t.Fatalf("expected 1 profile, got %d", len(loaded.Profiles))
	}
	if loaded.Profiles[0].Name != "test" {
		t.Fatalf("unexpected profile name %q", loaded.Profiles[0].Name)
	}
}

func TestUpsertAndDelete(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	cfg := &File{}
	if err := cfg.Upsert(Profile{Name: "a", Addr: "127.0.0.1:6379"}); err != nil {
		t.Fatal(err)
	}
	if err := cfg.Upsert(Profile{Name: "a", Addr: "127.0.0.1:6380", DB: 2}); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Profiles) != 1 || loaded.Profiles[0].DB != 2 {
		t.Fatalf("upsert failed: %+v", loaded.Profiles)
	}
	if err := loaded.Delete("a"); err != nil {
		t.Fatal(err)
	}
	again, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(again.Profiles) != 0 {
		t.Fatalf("expected empty profiles after delete")
	}
}

func TestDirCreatesUnderHome(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	got, err := Dir()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(dir, ".config", "lazyredis")
	if got != want {
		t.Fatalf("dir = %q, want %q", got, want)
	}
}

func TestMigrateFromLegacyConfig(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	legacyDir := filepath.Join(dir, ".config", "redis-tui")
	if err := os.MkdirAll(legacyDir, 0o755); err != nil {
		t.Fatal(err)
	}
	legacyPath := filepath.Join(legacyDir, "profiles.yaml")
	if err := os.WriteFile(legacyPath, []byte("profiles:\n  - name: legacy\n    addr: 127.0.0.1:6379\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	loaded, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Profiles) != 1 || loaded.Profiles[0].Name != "legacy" {
		t.Fatalf("unexpected profiles after migrate: %+v", loaded.Profiles)
	}

	newPath, err := Path()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(newPath); err != nil {
		t.Fatalf("expected migrated config at %s: %v", newPath, err)
	}

	if _, err := os.Stat(legacyPath); !os.IsNotExist(err) {
		t.Fatalf("expected legacy dir renamed away, legacy path still exists")
	}
}

func TestDefaultProfilesIncludesSecureRemote(t *testing.T) {
	cfg := DefaultProfiles()
	var found bool
	for _, p := range cfg.Profiles {
		if p.Name == "secure-remote" {
			found = true
			if p.TLS == nil || !p.TLS.Enabled || p.SSHTunnel == nil || !p.SSHTunnel.Enabled || p.Proxy == nil {
				t.Fatalf("secure-remote missing advanced settings: %+v", p)
			}
			break
		}
	}
	if !found {
		t.Fatal("expected secure-remote profile in defaults")
	}
}

func TestDefaultRefreshInterval(t *testing.T) {
	cfg := DefaultProfiles()
	if cfg.GetRefreshIntervalSec() != DefaultRefreshIntervalSec {
		t.Fatalf("default interval = %d, want %d", cfg.GetRefreshIntervalSec(), DefaultRefreshIntervalSec)
	}

	dir := t.TempDir()
	t.Setenv("HOME", dir)
	if err := Save(cfg); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.GetRefreshIntervalSec() != 5 {
		t.Fatalf("loaded interval = %d, want 5", loaded.GetRefreshIntervalSec())
	}
}

func TestSetRefreshInterval(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	cfg := &File{}
	if err := cfg.SetRefreshIntervalSec(10); err != nil {
		t.Fatal(err)
	}
	if cfg.GetRefreshIntervalSec() != 10 {
		t.Fatalf("interval = %d, want 10", cfg.GetRefreshIntervalSec())
	}
	if err := cfg.SetRefreshIntervalSec(0); err != nil {
		t.Fatal(err)
	}
	if cfg.GetRefreshIntervalSec() != 0 {
		t.Fatalf("interval = %d, want 0", cfg.GetRefreshIntervalSec())
	}
}
