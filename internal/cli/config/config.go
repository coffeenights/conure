// Package config owns the machine-local CLI config at ~/.conure/config.json:
// server URL, auth token, and active org. Commands call Load/Save and the
// Require* helpers; they don't reach into the JSON shape directly.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Config is the global per-machine CLI config. Persisted at
// ~/.conure/config.json with 0600.
type Config struct {
	Server    string `json:"server"`
	Token     string `json:"token"`
	ActiveOrg string `json:"active_org,omitempty"` // org ID
}

func dir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".conure"), nil
}

func Path() (string, error) {
	d, err := dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "config.json"), nil
}

func Load() (*Config, error) {
	path, err := Path()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func Save(cfg *Config) error {
	d, err := dir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(d, 0700); err != nil {
		return err
	}
	path := filepath.Join(d, "config.json")
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

// RequireAuth returns the config if the user is logged in, otherwise an
// error pointing them at `conure login`. serverOverride is applied last so
// the --server flag still wins.
func RequireAuth(serverOverride string) (*Config, error) {
	cfg, err := Load()
	if err != nil || cfg.Token == "" || cfg.Server == "" {
		return nil, fmt.Errorf("not logged in — run 'conure login' first")
	}
	if serverOverride != "" {
		cfg.Server = serverOverride
	}
	return cfg, nil
}

// RequireActiveOrg returns the config and asserts an active org is set.
// Used by commands that operate at org scope without a project link.
func RequireActiveOrg(serverOverride string) (*Config, error) {
	cfg, err := RequireAuth(serverOverride)
	if err != nil {
		return nil, err
	}
	if cfg.ActiveOrg == "" {
		return nil, fmt.Errorf("no active organization — run 'conure org use <name>' first")
	}
	return cfg, nil
}
