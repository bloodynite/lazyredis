//go:build integration

package store

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/bloodynite/lazyredis/internal/config"
	"gopkg.in/yaml.v3"
)

func loadSampleProfile(t *testing.T) config.Profile {
	t.Helper()
	path := os.Getenv("LAZYREDIS_SAMPLE_PROFILES")
	if path == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			t.Skipf("no HOME: %v", err)
		}
		path = filepath.Join(home, ".config", "lazyredis", "profiles.yaml")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("sample profile not readable at %s: %v", path, err)
	}
	var file struct {
		Profiles []config.Profile `yaml:"profiles"`
	}
	if err := yaml.Unmarshal(data, &file); err != nil {
		t.Fatal(err)
	}
	for i := range file.Profiles {
		if file.Profiles[i].Name == "sample" {
			return file.Profiles[i]
		}
	}
	t.Skip("sample profile not found in " + path)
	return config.Profile{}
}

func TestGetKeySummaryAllTypes(t *testing.T) {
	p := loadSampleProfile(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := Connect(ctx, p)
	if err != nil {
		t.Skipf("sample unreachable at %s: %v", p.Addr, err)
	}
	t.Cleanup(func() { _ = client.Close() })

	prefix := "lazyredis:summary:" + time.Now().Format("150405.000000")

	cases := []struct {
		name    string
		keyType string
		total   int64
		setup   func(t *testing.T, c *Client, key string)
	}{
		{
			name:    "string",
			keyType: "string",
			total:   11,
			setup: func(t *testing.T, c *Client, key string) {
				t.Helper()
				if err := c.SetString(ctx, key, "hello world", 30*time.Second); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name:    "hash",
			keyType: "hash",
			total:   2,
			setup: func(t *testing.T, c *Client, key string) {
				t.Helper()
				if err := c.SetHashField(ctx, key, "f1", "v1"); err != nil {
					t.Fatal(err)
				}
				if err := c.SetHashField(ctx, key, "f2", "v2"); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name:    "list",
			keyType: "list",
			total:   3,
			setup: func(t *testing.T, c *Client, key string) {
				t.Helper()
				for _, v := range []string{"a", "b", "c"} {
					if err := c.AppendListItem(ctx, key, v); err != nil {
						t.Fatal(err)
					}
				}
			},
		},
		{
			name:    "set",
			keyType: "set",
			total:   3,
			setup: func(t *testing.T, c *Client, key string) {
				t.Helper()
				for _, m := range []string{"a", "b", "c"} {
					if err := c.SetAddMember(ctx, key, m); err != nil {
						t.Fatal(err)
					}
				}
			},
		},
		{
			name:    "zset",
			keyType: "zset",
			total:   3,
			setup: func(t *testing.T, c *Client, key string) {
				t.Helper()
				if err := c.ZSetAddMember(ctx, key, 1, "a"); err != nil {
					t.Fatal(err)
				}
				if err := c.ZSetAddMember(ctx, key, 2, "b"); err != nil {
					t.Fatal(err)
				}
				if err := c.ZSetAddMember(ctx, key, 3, "c"); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name:    "stream",
			keyType: "stream",
			total:   2,
			setup: func(t *testing.T, c *Client, key string) {
				t.Helper()
				if err := c.StreamAddEntry(ctx, key, "*", map[string]string{"f": "1"}); err != nil {
					t.Fatal(err)
				}
				if err := c.StreamAddEntry(ctx, key, "*", map[string]string{"f": "2"}); err != nil {
					t.Fatal(err)
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			key := prefix + ":" + tc.name
			tc.setup(t, client, key)
			t.Cleanup(func() { _ = client.DeleteKey(ctx, key) })

			summary, err := client.GetKeySummary(ctx, key)
			if err != nil {
				t.Fatalf("GetKeySummary: %v", err)
			}
			if summary.Meta.Type != tc.keyType {
				t.Fatalf("Type = %q, want %q", summary.Meta.Type, tc.keyType)
			}
			if summary.Total != tc.total {
				t.Fatalf("Total = %d, want %d", summary.Total, tc.total)
			}
		})
	}
}
