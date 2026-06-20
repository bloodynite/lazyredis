package config

import (
	"fmt"
	"os"
	"path/filepath"

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
	RefreshIntervalSec *int              `yaml:"refresh_interval_seconds,omitempty"`
	Keybindings        map[string]string `yaml:"keybindings,omitempty"`
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
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", configDirName), nil
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
				Name:     "glyphverso",
				Mode:     ModeStandalone,
				Addr:     "127.0.0.1:6379",
				Password: "redis_secret",
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
	def := DefaultProfiles()
	if err := Save(def); err != nil {
		return nil, err
	}
	if def == nil {
		def = &File{}
	}
	return def, nil
}
