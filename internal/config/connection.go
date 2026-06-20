package config

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

type TLSConfig struct {
	Enabled            bool   `yaml:"enabled,omitempty"`
	InsecureSkipVerify bool   `yaml:"insecure_skip_verify,omitempty"`
	CAFile             string `yaml:"ca_cert,omitempty"`
	CertFile           string `yaml:"cert,omitempty"`
	KeyFile            string `yaml:"key,omitempty"`
	ServerName         string `yaml:"server_name,omitempty"`
}

type SSHTunnel struct {
	Enabled              bool   `yaml:"enabled,omitempty"`
	Host                 string `yaml:"host,omitempty"`
	User                 string `yaml:"user,omitempty"`
	Password             string `yaml:"password,omitempty"`
	PrivateKey           string `yaml:"private_key,omitempty"`
	PrivateKeyPassphrase string `yaml:"private_key_passphrase,omitempty"`
	KnownHosts           string `yaml:"known_hosts,omitempty"`
	InsecureSkipVerify   bool   `yaml:"insecure_skip_verify,omitempty"`
}

type ProxyConfig struct {
	Type     string `yaml:"type,omitempty"`
	Addr     string `yaml:"addr,omitempty"`
	Username string `yaml:"username,omitempty"`
	Password string `yaml:"password,omitempty"`
}

func ExpandPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	if strings.HasPrefix(path, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(home, strings.TrimPrefix(path, "~"))
	}
	return path
}

func ParseProxySpec(raw string) (*ProxyConfig, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	if !strings.Contains(raw, "://") {
		return nil, fmt.Errorf("proxy must look like socks5://host:port or http://host:port")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid proxy url: %w", err)
	}
	typ := strings.ToLower(u.Scheme)
	if typ != "http" && typ != "socks5" {
		return nil, fmt.Errorf("proxy type must be http or socks5")
	}
	host := u.Host
	if host == "" {
		return nil, fmt.Errorf("proxy host required")
	}
	cfg := &ProxyConfig{
		Type: typ,
		Addr: host,
	}
	if u.User != nil {
		cfg.Username = u.User.Username()
		cfg.Password, _ = u.User.Password()
	}
	return cfg, nil
}

func FormatProxySpec(p *ProxyConfig) string {
	if p == nil || p.Type == "" || p.Addr == "" {
		return ""
	}
	scheme := strings.ToLower(p.Type)
	if p.Username != "" {
		pass := url.QueryEscape(p.Password)
		user := url.QueryEscape(p.Username)
		return fmt.Sprintf("%s://%s:%s@%s", scheme, user, pass, p.Addr)
	}
	return fmt.Sprintf("%s://%s", scheme, p.Addr)
}

func ParseSSHSpec(userHost string) (user, host string, err error) {
	userHost = strings.TrimSpace(userHost)
	if userHost == "" {
		return "", "", nil
	}
	if strings.Contains(userHost, "@") {
		parts := strings.SplitN(userHost, "@", 2)
		user = strings.TrimSpace(parts[0])
		host = strings.TrimSpace(parts[1])
	} else {
		host = userHost
	}
	if host == "" {
		return "", "", fmt.Errorf("ssh host required")
	}
	if !strings.Contains(host, ":") {
		host += ":22"
	}
	if user == "" {
		return "", "", fmt.Errorf("ssh user required (use user@host:port)")
	}
	return user, host, nil
}

func FormatSSHSpec(t *SSHTunnel) string {
	if t == nil || !t.Enabled || t.Host == "" || t.User == "" {
		return ""
	}
	host := t.Host
	if !strings.Contains(host, ":") {
		host += ":22"
	}
	return t.User + "@" + host
}

func ParseTLSSpec(raw string) (*TLSConfig, error) {
	raw = strings.TrimSpace(strings.ToLower(raw))
	if raw == "" || raw == "off" || raw == "false" {
		return nil, nil
	}
	cfg := &TLSConfig{Enabled: true}
	switch raw {
	case "on", "true", "tls":
		return cfg, nil
	case "skip", "insecure":
		cfg.InsecureSkipVerify = true
		return cfg, nil
	default:
		return nil, fmt.Errorf("tls must be off, on, or skip")
	}
}

func FormatTLSSpec(t *TLSConfig) string {
	if t == nil || !t.Enabled {
		return ""
	}
	if t.InsecureSkipVerify {
		return "skip"
	}
	return "on"
}

func ParseAddrs(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func FormatAddrs(addrs []string) string {
	return strings.Join(addrs, ", ")
}

func MergeProfile(existing, updated Profile) Profile {
	out := updated
	if out.TLS != nil && out.TLS.Enabled && existing.TLS != nil {
		if out.TLS.CAFile == "" {
			out.TLS.CAFile = existing.TLS.CAFile
		}
		if out.TLS.CertFile == "" {
			out.TLS.CertFile = existing.TLS.CertFile
		}
		if out.TLS.KeyFile == "" {
			out.TLS.KeyFile = existing.TLS.KeyFile
		}
		if out.TLS.ServerName == "" {
			out.TLS.ServerName = existing.TLS.ServerName
		}
	}
	if out.SSHTunnel != nil && out.SSHTunnel.Enabled && existing.SSHTunnel != nil {
		if out.SSHTunnel.Password == "" {
			out.SSHTunnel.Password = existing.SSHTunnel.Password
		}
		if out.SSHTunnel.PrivateKeyPassphrase == "" {
			out.SSHTunnel.PrivateKeyPassphrase = existing.SSHTunnel.PrivateKeyPassphrase
		}
		if out.SSHTunnel.KnownHosts == "" {
			out.SSHTunnel.KnownHosts = existing.SSHTunnel.KnownHosts
		}
		if !out.SSHTunnel.InsecureSkipVerify && existing.SSHTunnel.InsecureSkipVerify {
			out.SSHTunnel.InsecureSkipVerify = existing.SSHTunnel.InsecureSkipVerify
		}
	}
	if out.Proxy != nil && existing.Proxy != nil {
		if out.Proxy.Username == "" {
			out.Proxy.Username = existing.Proxy.Username
		}
		if out.Proxy.Password == "" {
			out.Proxy.Password = existing.Proxy.Password
		}
	}
	if out.SentinelUsername == "" {
		out.SentinelUsername = existing.SentinelUsername
	}
	return out
}
