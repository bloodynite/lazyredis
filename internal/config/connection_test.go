package config

import "testing"

func TestParseProxySpec(t *testing.T) {
	cfg, err := ParseProxySpec("socks5://proxy.corp:1080")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Type != "socks5" || cfg.Addr != "proxy.corp:1080" {
		t.Fatalf("unexpected proxy: %+v", cfg)
	}

	cfg, err = ParseProxySpec("http://user:pass@proxy.corp:8080")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Username != "user" || cfg.Password != "pass" {
		t.Fatalf("unexpected auth: %+v", cfg)
	}
}

func TestParseSSHSpec(t *testing.T) {
	user, host, err := ParseSSHSpec("deploy@jump.example.com:2222")
	if err != nil {
		t.Fatal(err)
	}
	if user != "deploy" || host != "jump.example.com:2222" {
		t.Fatalf("got %q %q", user, host)
	}
}

func TestParseTLSSpec(t *testing.T) {
	cfg, err := ParseTLSSpec("skip")
	if err != nil {
		t.Fatal(err)
	}
	if cfg == nil || !cfg.Enabled || !cfg.InsecureSkipVerify {
		t.Fatalf("unexpected tls: %+v", cfg)
	}
}

func TestAdvancedProfileRoundTrip(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	cfg := &File{
		Profiles: []Profile{
			{
				Name:             "remote",
				Mode:             ModeSentinel,
				Addr:             "10.0.0.1:26379",
				Addrs:            []string{"10.0.0.1:26379", "10.0.0.2:26379"},
				MasterName:       "mymaster",
				Password:         "redis",
				SentinelPassword: "sentinel",
				DB:               1,
				TLS: &TLSConfig{
					Enabled:    true,
					CAFile:     "/etc/ssl/ca.pem",
					ServerName: "redis.internal",
				},
				SSHTunnel: &SSHTunnel{
					Enabled:            true,
					Host:               "jump:22",
					User:               "deploy",
					PrivateKey:         "~/.ssh/id_ed25519",
					InsecureSkipVerify: true,
				},
				Proxy: &ProxyConfig{
					Type: "socks5",
					Addr: "127.0.0.1:1080",
				},
			},
		},
	}
	if err := Save(cfg); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Profiles) != 1 {
		t.Fatalf("profiles = %d", len(loaded.Profiles))
	}
	p := loaded.Profiles[0]
	if p.MasterName != "mymaster" || p.TLS == nil || p.SSHTunnel == nil || p.Proxy == nil {
		t.Fatalf("unexpected profile: %+v", p)
	}
}

func TestMergeProfileKeepsCertPaths(t *testing.T) {
	existing := Profile{
		Name: "x",
		TLS: &TLSConfig{
			Enabled:  true,
			CAFile:   "/ca.pem",
			CertFile: "/client.pem",
			KeyFile:  "/client.key",
		},
	}
	updated := Profile{
		Name: "x",
		TLS:  &TLSConfig{Enabled: true},
	}
	merged := MergeProfile(existing, updated)
	if merged.TLS.CAFile != "/ca.pem" {
		t.Fatalf("ca = %q", merged.TLS.CAFile)
	}
}
