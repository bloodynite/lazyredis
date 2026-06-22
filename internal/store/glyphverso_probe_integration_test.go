//go:build integration

package store

import (
	"context"
	"sort"
	"testing"
	"time"
)

// TestGlyphversoFirst10KeysProbe reports type, total, and per-call latency for
// the first 10 alphabetically sorted keys in the glyphverso profile. The test
// does not assert hard thresholds — it surfaces real numbers so a developer
// can spot outliers (large strings, large hashes, large streams) before
// chasing perceived UI latency.
//
// Run with:
//   go test -tags=integration -run TestGlyphversoFirst10KeysProbe ./internal/store
func TestGlyphversoFirst10KeysProbe(t *testing.T) {
	p := loadGlyphversoProfile(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	client, err := Connect(ctx, p)
	if err != nil {
		t.Skipf("glyphverso unreachable at %s: %v", p.Addr, err)
	}
	t.Cleanup(func() { _ = client.Close() })

	keys, _, err := client.ScanKeys(ctx, 0, "*", 1000)
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if len(keys) == 0 {
		t.Skip("glyphverso has no keys; seed the profile before running this test")
	}
	sort.Strings(keys)
	if len(keys) > 10 {
		keys = keys[:10]
	}

	type sample struct {
		key       string
		keyType   string
		total     int64
		summaryMs time.Duration
		detailMs  time.Duration
		detailErr error
	}
	samples := make([]sample, 0, len(keys))
	var summaryTotal, detailTotal time.Duration

	t.Logf("probing first %d keys in %s", len(keys), p.Addr)
	for _, k := range keys {
		t0 := time.Now()
		s, err := client.GetKeySummary(ctx, k)
		summaryMs := time.Since(t0)
		if err != nil {
			t.Logf("  %-40s | summary ERR: %v", k, err)
			continue
		}
		t1 := time.Now()
		_, dErr := client.GetKey(ctx, k, -1, 0)
		detailMs := time.Since(t1)
		samples = append(samples, sample{k, s.Meta.Type, s.Total, summaryMs, detailMs, dErr})
		summaryTotal += summaryMs
		detailTotal += detailMs
		t.Logf("  %-40s | %-6s | total=%-6d | summary=%-9s | detail=%-9s",
			k, s.Meta.Type, s.Total, summaryMs.Round(time.Microsecond), detailMs.Round(time.Microsecond))
	}
	if len(samples) == 0 {
		t.Fatal("no keys produced a usable sample")
	}
	t.Logf("avg summary=%s avg detail=%s (across %d keys)",
		(summaryTotal/time.Duration(len(samples))).Round(time.Microsecond),
		(detailTotal/time.Duration(len(samples))).Round(time.Microsecond),
		len(samples))

	// Sanity: every probed detail must succeed at offset=-1 (the path the UI
	// uses on first navigation). A failure here means at least one type the
	// UI claims to support does not round-trip cleanly with a full fetch.
	for _, s := range samples {
		if s.detailErr != nil {
			t.Errorf("GetKey(-1,0) for %s (%s) failed: %v", s.key, s.keyType, s.detailErr)
		}
	}
}

// TestGlyphversoNavigationLatencyPerType isolates per-type summary + detail
// latency for the first key of each type found in glyphverso. This is the
// latency the user pays on every cursor move that lands on a new type. If
// any type costs more than ~50ms, the perceived "lag" is at the network or
// store layer, not the debounce.
//
// Run with:
//   go test -tags=integration -run TestGlyphversoNavigationLatencyPerType ./internal/store
func TestGlyphversoNavigationLatencyPerType(t *testing.T) {
	p := loadGlyphversoProfile(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	client, err := Connect(ctx, p)
	if err != nil {
		t.Skipf("glyphverso unreachable at %s: %v", p.Addr, err)
	}
	t.Cleanup(func() { _ = client.Close() })

	keys, _, err := client.ScanKeys(ctx, 0, "*", 1000)
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	sort.Strings(keys)
	firstOfType := map[string]string{}
	for _, k := range keys {
		s, err := client.GetKeySummary(ctx, k)
		if err != nil {
			continue
		}
		if _, ok := firstOfType[s.Meta.Type]; !ok {
			firstOfType[s.Meta.Type] = k
		}
	}
	if len(firstOfType) == 0 {
		t.Skip("no keys with readable type in glyphverso")
	}

	// Probe 10 navigations per type: simulates the user pressing j nine times
	// past the same key. The first call pays connection-pool warmup so we
	// discard it; the rest reflect steady-state latency.
	const warmup = 1
	const probes = 10
	for keyType, k := range firstOfType {
		var summaryTimes, detailTimes []time.Duration
		for i := 0; i < warmup+probes; i++ {
			t0 := time.Now()
			if _, err := client.GetKeySummary(ctx, k); err != nil {
				t.Fatalf("summary for %s: %v", k, err)
			}
			summaryTimes = append(summaryTimes, time.Since(t0))

			t1 := time.Now()
			if _, err := client.GetKey(ctx, k, -1, 0); err != nil {
				t.Fatalf("detail for %s: %v", k, err)
			}
			detailTimes = append(detailTimes, time.Since(t1))
		}
		summaryTimes = summaryTimes[warmup:]
		detailTimes = detailTimes[warmup:]

		avg := func(ds []time.Duration) time.Duration {
			var total time.Duration
			for _, d := range ds {
				total += d
			}
			return total / time.Duration(len(ds))
		}
		max := func(ds []time.Duration) time.Duration {
			m := time.Duration(0)
			for _, d := range ds {
				if d > m {
					m = d
				}
			}
			return m
		}
		t.Logf("type=%-6s key=%-30s | summary avg=%-9s max=%-9s | detail avg=%-9s max=%-9s",
			keyType, k,
			avg(summaryTimes).Round(time.Microsecond), max(summaryTimes).Round(time.Microsecond),
			avg(detailTimes).Round(time.Microsecond), max(detailTimes).Round(time.Microsecond))
	}
}
