package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"gopkg.in/yaml.v3"
)

type Mode string

const (
	ModeStandalone Mode = "standalone"
	ModeCluster    Mode = "cluster"
	ModeSentinel   Mode = "sentinel"
)

const (
	DefaultRefreshIntervalSec = 5
	configDirName             = "lazyredis"
	legacyConfigDirName       = "redis-tui"
)

type Profile struct {
	Name             string       `yaml:"name"`
	Mode             Mode         `yaml:"mode"`
	Addr             string       `yaml:"addr"`
	Addrs            []string     `yaml:"addrs,omitempty"`
	MasterName       string       `yaml:"master_name,omitempty"`
	Password         string       `yaml:"password,omitempty"`
	SentinelUsername string       `yaml:"sentinel_username,omitempty"`
	SentinelPassword string       `yaml:"sentinel_password,omitempty"`
	DB               int          `yaml:"db"`
	TLS              *TLSConfig   `yaml:"tls,omitempty"`
	SSHTunnel        *SSHTunnel   `yaml:"ssh_tunnel,omitempty"`
	Proxy            *ProxyConfig `yaml:"proxy,omitempty"`
}

type Settings struct {
	RefreshIntervalSec *int   `yaml:"refresh_interval_seconds,omitempty"`
	ShortcutModifier   string `yaml:"shortcut_modifier,omitempty"`
}

type File struct {
	Settings Settings  `yaml:"settings,omitempty"`
	Profiles []Profile `yaml:"profiles"`
}

func (f *File) GetRefreshIntervalSec() int {
	if f != nil && f.Settings.RefreshIntervalSec != nil {
		return *f.Settings.RefreshIntervalSec
	}
	return DefaultRefreshIntervalSec
}

func (f *File) GetShortcutModifier() string {
	if f == nil {
		return "ctrl"
	}
	switch strings.ToLower(strings.TrimSpace(f.Settings.ShortcutModifier)) {
	case "alt":
		return "alt"
	default:
		return "ctrl"
	}
}

func (f *File) SetRefreshIntervalSec(sec int) error {
	if sec < 0 {
		return fmt.Errorf("refresh interval must be >= 0")
	}
	f.Settings.RefreshIntervalSec = &sec
	return Save(f)
}

func legacyDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", legacyConfigDirName), nil
}

func Dir() (string, error) {
	base, err := configBase()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, configDirName), nil
}

func configBase() (string, error) {
	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		return os.UserConfigDir()
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config"), nil
}

func migrateConfigIfNeeded() error {
	newDir, err := Dir()
	if err != nil {
		return err
	}
	newPath, err := Path()
	if err != nil {
		return err
	}
	if _, statErr := os.Stat(newPath); statErr == nil {
		return nil
	} else if !os.IsNotExist(statErr) {
		return statErr
	}
	legacyDirPath, err := legacyDir()
	if err != nil {
		return err
	}
	if _, legacyDirStatErr := os.Stat(newDir); os.IsNotExist(legacyDirStatErr) {
		if _, legacyStatErr := os.Stat(legacyDirPath); legacyStatErr == nil {
			return os.Rename(legacyDirPath, newDir)
		}
	}
	legacyPath := filepath.Join(legacyDirPath, "profiles.yaml")
	data, err := os.ReadFile(legacyPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if err := os.MkdirAll(newDir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(newPath, data, 0o600)
}

func Path() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "profiles.yaml"), nil
}

func Load() (*File, error) {
	if err := migrateConfigIfNeeded(); err != nil {
		return nil, err
	}
	path, err := Path()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &File{}, nil
		}
		return nil, err
	}
	var f File
	if err := yaml.Unmarshal(data, &f); err != nil {
		return nil, err
	}
	return &f, nil
}

func Save(f *File) error {
	dir, err := Dir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	path, err := Path()
	if err != nil {
		return err
	}
	data, err := yaml.Marshal(f)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func (f *File) Find(name string) (*Profile, int) {
	for i, p := range f.Profiles {
		if p.Name == name {
			return &f.Profiles[i], i
		}
	}
	return nil, -1
}

func (f *File) Upsert(p Profile) error {
	if p.Name == "" {
		return fmt.Errorf("profile name required")
	}
	if p.Mode == "" {
		p.Mode = ModeStandalone
	}
	if p.Addr == "" && len(p.Addrs) == 0 {
		return fmt.Errorf("addr or addrs required")
	}
	if idx := indexByName(f.Profiles, p.Name); idx >= 0 {
		f.Profiles[idx] = p
	} else {
		f.Profiles = append(f.Profiles, p)
	}
	return Save(f)
}

func (f *File) Delete(name string) error {
	idx := indexByName(f.Profiles, name)
	if idx < 0 {
		return fmt.Errorf("profile not found")
	}
	f.Profiles = append(f.Profiles[:idx], f.Profiles[idx+1:]...)
	return Save(f)
}

func indexByName(profiles []Profile, name string) int {
	for i, p := range profiles {
		if p.Name == name {
			return i
		}
	}
	return -1
}

func DefaultProfiles() *File {
	sec := DefaultRefreshIntervalSec
	return &File{
		Settings: Settings{RefreshIntervalSec: &sec},
		Profiles: []Profile{
			{
				Name:     "local",
				Mode:     ModeStandalone,
				Addr:     "127.0.0.1:6379",
				Password: "",
				DB:       0,
			},
			{
				Name:     "local-auth",
				Mode:     ModeStandalone,
				Addr:     "127.0.0.1:6379",
				Password: "change-me",
				DB:       0,
			},
			{
				Name:     "secure-remote",
				Mode:     ModeStandalone,
				Addr:     "10.0.0.5:6379",
				Password: "change-me",
				DB:       0,
				TLS: &TLSConfig{
					Enabled:    true,
					CAFile:     "/etc/ssl/redis-ca.pem",
					CertFile:   "/etc/ssl/redis-client.pem",
					KeyFile:    "/etc/ssl/redis-client.key",
					ServerName: "redis.internal",
				},
				SSHTunnel: &SSHTunnel{
					Enabled:            true,
					Host:               "jump.example.com:22",
					User:               "deploy",
					PrivateKey:         "~/.ssh/id_ed25519",
					KnownHosts:         "~/.ssh/known_hosts",
					InsecureSkipVerify: false,
				},
				Proxy: &ProxyConfig{
					Type:     "socks5",
					Addr:     "corp-proxy:1080",
					Username: "proxy-user",
					Password: "proxy-pass",
				},
			},
		},
	}
}

func EnsureDefault() (*File, error) {
	if err := migrateConfigIfNeeded(); err != nil {
		return nil, err
	}
	path, err := Path()
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(path); err == nil {
		cfg, loadErr := Load()
		if loadErr != nil {
			return nil, loadErr
		}
		if cfg == nil {
			cfg = &File{}
		}
		return cfg, nil
	}
	if !os.IsNotExist(err) {
		return nil, err
	}
	dir, err := Dir()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, []byte(defaultConfigTemplate), 0o600); err != nil {
		return nil, err
	}
	cfg, err := Load()
	if err != nil {
		return nil, err
	}
	if cfg == nil {
		cfg = &File{}
	}
	return cfg, nil
}

const defaultConfigTemplate = `# lazyredis settings and connection profiles
# Shortcut modifier: ctrl or alt

settings:
  refresh_interval_seconds: 5
  shortcut_modifier: ctrl

profiles:
  - name: local
    mode: standalone
    addr: 127.0.0.1:6379
    password: ""
    db: 0

  - name: local-auth
    mode: standalone
    addr: 127.0.0.1:6379
    password: change-me
    db: 0

  - name: secure-remote
    mode: standalone
    addr: 10.0.0.5:6379
    password: change-me
    db: 0
    tls:
      enabled: true
      ca_cert: /etc/ssl/redis-ca.pem
      cert: /etc/ssl/redis-client.pem
      key: /etc/ssl/redis-client.key
      server_name: redis.internal
    ssh_tunnel:
      enabled: true
      host: jump.example.com:22
      user: deploy
      private_key: ~/.ssh/id_ed25519
      known_hosts: ~/.ssh/known_hosts
      insecure_skip_verify: false
    proxy:
      type: socks5
      addr: corp-proxy:1080
      username: proxy-user
      password: proxy-pass
`
