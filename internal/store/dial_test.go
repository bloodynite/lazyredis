package store

import (
	"testing"

	"github.com/frankz/lazyredis/internal/config"
)

func TestBuildTLSConfigFromProfile(t *testing.T) {
	p := config.Profile{
		Addr: "redis.example.com:6379",
		TLS: &config.TLSConfig{
			Enabled:            true,
			InsecureSkipVerify: true,
			ServerName:         "redis.example.com",
		},
	}
	cfg, err := buildTLSConfig(p)
	if err != nil {
		t.Fatal(err)
	}
	if cfg == nil || !cfg.InsecureSkipVerify {
		t.Fatalf("unexpected cfg: %+v", cfg)
	}
}

func TestBuildProxyDialerInvalidType(t *testing.T) {
	_, err := buildProxyDialer(&config.ProxyConfig{Type: "ftp", Addr: "x:1"})
	if err == nil {
		t.Fatal("expected error")
	}
}
