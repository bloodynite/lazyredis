package store

import (
	"testing"
	"time"
)

func TestNormalizeScanPattern(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{in: "", want: "*"},
		{in: "*", want: "*"},
		{in: "demo", want: "*demo*"},
		{in: "016", want: "*016*"},
		{in: "glyph", want: "*glyph*"},
		{in: "user:", want: "*user:*"},
		{in: "user:*", want: "user:*"},
		{in: "key?", want: "key?"},
	}
	for _, tt := range tests {
		if got := NormalizeScanPattern(tt.in); got != tt.want {
			t.Fatalf("NormalizeScanPattern(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestParseTTLInput(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    time.Duration
		wantErr bool
	}{
		{name: "persist dash", in: "-", want: -1},
		{name: "persist word", in: "persist", want: -1},
		{name: "seconds", in: "300", want: 300 * time.Second},
		{name: "duration", in: "1h", want: time.Hour},
		{name: "invalid", in: "nope", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseTTLInput(tt.in)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Fatalf("got %v want %v", got, tt.want)
			}
		})
	}
}

func TestFormatTTL(t *testing.T) {
	if FormatTTL(-2*time.Second) != "does not exist" {
		t.Fatal("expected does not exist")
	}
	if FormatTTL(-1*time.Second) != "no expiry" {
		t.Fatal("expected no expiry")
	}
}

func TestParseInfoClientsSection(t *testing.T) {
	raw := "# Clients\r\nconnected_clients:2\r\n# Server\r\nredis_version:7.4.9\r\n"
	m := parseInfo(raw)
	if m["connected_clients"] != "2" {
		t.Fatalf("connected_clients = %q", m["connected_clients"])
	}
}

func TestEncodeParseKeyBodyHash(t *testing.T) {
	d := &KeyDetail{
		Meta: KeyMeta{Type: "hash"},
		Hash: map[string]string{"a": "1", "b": "2"},
	}
	raw := EncodeKeyBody(d)
	body, err := ParseKeyBody("hash", raw)
	if err != nil {
		t.Fatal(err)
	}
	if body.Hash["a"] != "1" || body.Hash["b"] != "2" {
		t.Fatalf("unexpected hash: %#v", body.Hash)
	}
}

func TestParseKeyBodyStream(t *testing.T) {
	body, err := ParseKeyBody("stream", "1-0\ta=1\n2-0\tb=2")
	if err != nil {
		t.Fatal(err)
	}
	if len(body.Stream) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(body.Stream))
	}
	if body.Stream[0].Fields["a"] != "1" {
		t.Fatalf("unexpected fields: %#v", body.Stream[0].Fields)
	}
}

func TestParseKeyBodyZSet(t *testing.T) {
	body, err := ParseKeyBody("zset", "1.5\tmember\n2\tother")
	if err != nil {
		t.Fatal(err)
	}
	if len(body.ZSet) != 2 {
		t.Fatalf("expected 2 members, got %d", len(body.ZSet))
	}
	if body.ZSet[0].Score != 1.5 {
		t.Fatalf("score = %v", body.ZSet[0].Score)
	}
}
