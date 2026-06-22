//go:build integration

package store

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// TestStreamFullLoadRegression covers the bug where loadStreamWindow called
// XRangeN with COUNT=0 in full mode. go-redis treats COUNT=0 as invalid and
// returns "redis: nil", so the UI saw an empty stream for any key with
// Total <= detailChunkSize (200). Fix: loadStreamWindow now uses XRange
// (no COUNT) for full mode.
//
// The seed uses a small stream on purpose — the bug only triggers in the
// full-load branch, which is taken when Total <= 200.
//
// Run with:
//   go test -tags=integration -run TestStreamFullLoadRegression ./internal/store
func TestStreamFullLoadRegression(t *testing.T) {
	p := loadGlyphversoProfile(t)
	p.DB = 15
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	c, err := Connect(ctx, p)
	if err != nil {
		t.Skipf("glyphverso unreachable: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })
	if err := c.FlushDB(ctx); err != nil {
		t.Fatal(err)
	}

	key := "lazyredis:stream-full-load:" + time.Now().Format("150405.000000")
	for i := 0; i < 50; i++ {
		if err := c.StreamAddEntry(ctx, key, "*",
			map[string]string{"k": fmt.Sprintf("v%d", i)}); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() { _ = c.DeleteKey(ctx, key) })

	detail, err := c.GetKey(ctx, key, -1, 0)
	if err != nil {
		t.Fatalf("GetKey full: %v", err)
	}
	if len(detail.Stream) != 50 {
		t.Fatalf("stream full-load returned %d entries, want 50 (regression: XRangeN COUNT=0 returns nil)", len(detail.Stream))
	}
	if detail.Stream[0].Fields["k"] != "v0" {
		t.Fatalf("first entry fields=%v, want k=v0", detail.Stream[0].Fields)
	}
}
