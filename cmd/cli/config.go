package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Config is the global, machine-local CLI config: auth + active org context.
// Persisted at ~/.conure/config.json.
type Config struct {
	Server    string `json:"server"`
	Token     string `json:"token"`
	ActiveOrg string `json:"active_org,omitempty"` // org ID
}

func configDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".conure"), nil
}

func configPath() (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.json"), nil
}

func loadConfig() (*Config, error) {
	path, err := configPath()
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

func saveConfig(cfg *Config) error {
	dir, err := configDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	path := filepath.Join(dir, "config.json")
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

func requireAuth() (*Config, error) {
	cfg, err := loadConfig()
	if err != nil {
		return nil, fmt.Errorf("not logged in — run 'conure login' first")
	}
	if cfg.Token == "" || cfg.Server == "" {
		return nil, fmt.Errorf("not logged in — run 'conure login' first")
	}
	if serverFlag != "" {
		cfg.Server = serverFlag
	}
	return cfg, nil
}

// requireActiveOrg returns the auth config and asserts an active org is set.
// Used by commands that operate at org scope without a project link.
func requireActiveOrg() (*Config, error) {
	cfg, err := requireAuth()
	if err != nil {
		return nil, err
	}
	if cfg.ActiveOrg == "" {
		return nil, fmt.Errorf("no active organization — run 'conure switch org <name>' first")
	}
	return cfg, nil
}
