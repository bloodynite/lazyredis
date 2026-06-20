//go:build integration

package store

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/frankz/lazyredis/internal/config"
	"gopkg.in/yaml.v3"
)

func testRoot(t *testing.T) string {
	t.Helper()
	if root := os.Getenv("LAZYREDIS_TEST_ROOT"); root != "" {
		return root
	}
	if root := os.Getenv("REDIS_TUI_TEST_ROOT"); root != "" {
		return root
	}
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for dir := wd; dir != "/" && dir != "."; dir = filepath.Dir(dir) {
		if _, err := os.Stat(filepath.Join(dir, "test", "docker-compose.yml")); err == nil {
			return dir
		}
	}
	t.Skip("set LAZYREDIS_TEST_ROOT or run from repo with test/docker-compose.yml")
	return ""
}

func loadTestProfiles(t *testing.T, root string) []config.Profile {
	t.Helper()
	path := filepath.Join(root, "test", "profiles.generated.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("run test/up.sh first: %v", err)
	}
	var file struct {
		Profiles []config.Profile `yaml:"profiles"`
	}
	if err := yaml.Unmarshal(data, &file); err != nil {
		t.Fatal(err)
	}
	return file.Profiles
}

func TestIntegrationConnections(t *testing.T) {
	root := testRoot(t)
	profiles := loadTestProfiles(t, root)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	names := []string{
		"test-standalone",
		"test-tls",
		"test-ssh",
		"test-ssh-tls",
		"test-socks5",
		"test-sentinel",
	}

	for _, want := range names {
		t.Run(want, func(t *testing.T) {
			var p *config.Profile
			for i := range profiles {
				if profiles[i].Name == want {
					p = &profiles[i]
					break
				}
			}
			if p == nil {
				t.Fatalf("profile %q not found", want)
			}
			client, err := Connect(ctx, *p)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = client.Close() })
			if err := client.Ping(ctx); err != nil {
				t.Fatal(err)
			}
		})
	}

	t.Run("test-cluster", func(t *testing.T) {
		var p *config.Profile
		for i := range profiles {
			if profiles[i].Name == "test-cluster" {
				p = &profiles[i]
				break
			}
		}
		if p == nil {
			t.Fatal("profile test-cluster not found")
		}
		client, err := Connect(ctx, *p)
		if err != nil {
			t.Skipf("cluster often needs extra docker network tuning: %v", err)
		}
		t.Cleanup(func() { _ = client.Close() })
		if err := client.Ping(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("test-secure-remote", func(t *testing.T) {
		var p *config.Profile
		for i := range profiles {
			if profiles[i].Name == "test-secure-remote" {
				p = &profiles[i]
				break
			}
		}
		if p == nil {
			t.Fatal("profile test-secure-remote not found")
		}
		client, err := Connect(ctx, *p)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = client.Close() })
		if err := client.Ping(ctx); err != nil {
			t.Fatal(err)
		}
	})
}
