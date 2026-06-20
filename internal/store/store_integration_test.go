package store

import (
	"context"
	"testing"
	"time"

	"github.com/frankz/lazyredis/internal/config"
)

func TestIntegrationRedis(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx := context.Background()
	p := config.Profile{
		Name:     "integration",
		Mode:     config.ModeStandalone,
		Addr:     "127.0.0.1:6379",
		Password: "redis_secret",
		DB:       0,
	}

	client, err := Connect(ctx, p)
	if err != nil {
		t.Skipf("redis not available: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })

	key := "lazyredis:test:" + time.Now().Format("150405")
	if err := client.SetString(ctx, key, "hello", 30*time.Second); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.DeleteKey(ctx, key) })

	detail, err := client.GetKey(ctx, key)
	if err != nil {
		t.Fatal(err)
	}
	if detail.String != "hello" {
		t.Fatalf("value = %q", detail.String)
	}
	if detail.Meta.Type != "string" {
		t.Fatalf("type = %q", detail.Meta.Type)
	}

	if err := client.SetString(ctx, key, "updated", 30*time.Second); err != nil {
		t.Fatal(err)
	}
	detail, err = client.GetKey(ctx, key)
	if err != nil {
		t.Fatal(err)
	}
	if detail.String != "updated" {
		t.Fatalf("updated value = %q", detail.String)
	}

	keys, cursor, err := client.ScanKeys(ctx, 0, key, 10)
	if err != nil {
		t.Fatal(err)
	}
	if cursor != 0 {
		t.Fatalf("unexpected cursor %d", cursor)
	}
	found := false
	for _, k := range keys {
		if k == key {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("scan did not find key %q in %v", key, keys)
	}

	info, err := client.Info(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if info.Version == "" {
		t.Fatal("expected redis version in info")
	}
}
