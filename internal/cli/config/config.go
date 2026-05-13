// Package config owns the machine-local CLI config at ~/.conure/config.json.
// The file holds one or more named profiles (server + token + active org)
// and a pointer to the currently active one. Commands call Load/Save and
// the Require* helpers; they don't reach into the JSON shape directly.
package config

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// Profile is one named login: a server URL, an auth token, and the org the
// user has selected on that server. Orgs are server-scoped, so they live
// inside the profile rather than at the top level.
type Profile struct {
	Server    string `json:"server"`
	Token     string `json:"token"`
	ActiveOrg string `json:"active_org,omitempty"`
}

// Config is the on-disk shape of ~/.conure/config.json. Active names the
// currently selected profile; Profiles is a name → profile map.
type Config struct {
	Active   string              `json:"active"`
	Profiles map[string]*Profile `json:"profiles"`
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

// Load reads the config file, migrating the legacy flat shape
// (top-level server/token/active_org) to the new multi-profile shape on
// the fly. Migration is silently persisted so the on-disk file stops
// re-migrating on every read.
func Load() (*Config, error) {
	path, err := Path()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	// First, try the new shape. If the file already has a Profiles map,
	// trust it.
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err == nil && cfg.Profiles != nil {
		return &cfg, nil
	}

	// Fall back to the legacy flat shape and migrate.
	var legacy struct {
		Server    string `json:"server"`
		Token     string `json:"token"`
		ActiveOrg string `json:"active_org,omitempty"`
	}
	if err := json.Unmarshal(data, &legacy); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}
	if legacy.Server == "" && legacy.Token == "" {
		// File exists but is empty/blank — return an empty config with an
		// initialised map so callers can Upsert without nil checks.
		return &Config{Profiles: map[string]*Profile{}}, nil
	}
	name := DefaultProfileName(legacy.Server)
	migrated := &Config{
		Active: name,
		Profiles: map[string]*Profile{
			name: {
				Server:    legacy.Server,
				Token:     legacy.Token,
				ActiveOrg: legacy.ActiveOrg,
			},
		},
	}
	// Persist so we only migrate once.
	_ = Save(migrated)
	return migrated, nil
}

// Save writes the config atomically-enough (single write, 0600 mode).
func Save(cfg *Config) error {
	d, err := dir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(d, 0700); err != nil {
		return err
	}
	if cfg.Profiles == nil {
		cfg.Profiles = map[string]*Profile{}
	}
	path := filepath.Join(d, "config.json")
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

// DefaultProfileName derives a profile name from a server URL by taking
// the leading hostname label (so `https://api.conure.io` → `api`). When
// the URL is unparseable or has no host, it falls back to "default".
func DefaultProfileName(server string) string {
	if server == "" {
		return "default"
	}
	u, err := url.Parse(server)
	if err != nil {
		return "default"
	}
	host := u.Hostname()
	if host == "" {
		host = server
	}
	if i := strings.Index(host, "."); i > 0 {
		host = host[:i]
	}
	if host == "" {
		return "default"
	}
	return host
}

// ---- profile-level operations -------------------------------------------

// GetActive returns the currently active profile, or nil if Active is
// unset or doesn't match any profile.
func (c *Config) GetActive() *Profile {
	if c.Active == "" || c.Profiles == nil {
		return nil
	}
	return c.Profiles[c.Active]
}

// Get returns the named profile or nil.
func (c *Config) Get(name string) *Profile {
	if c.Profiles == nil {
		return nil
	}
	return c.Profiles[name]
}

// Names returns all profile names. Order is unspecified.
func (c *Config) Names() []string {
	out := make([]string, 0, len(c.Profiles))
	for n := range c.Profiles {
		out = append(out, n)
	}
	return out
}

// Upsert installs or replaces a profile under name. If no profile is
// active yet, this one becomes active.
func (c *Config) Upsert(name string, p *Profile) {
	if c.Profiles == nil {
		c.Profiles = map[string]*Profile{}
	}
	c.Profiles[name] = p
	if c.Active == "" {
		c.Active = name
	}
}

// Use sets the active profile, or errors when name doesn't exist.
func (c *Config) Use(name string) error {
	if _, ok := c.Profiles[name]; !ok {
		return fmt.Errorf("no profile named %q", name)
	}
	c.Active = name
	return nil
}

// Remove drops a profile. If it was active, Active is cleared (the user
// is now logged out of the active session even though other profiles
// remain).
func (c *Config) Remove(name string) error {
	if _, ok := c.Profiles[name]; !ok {
		return fmt.Errorf("no profile named %q", name)
	}
	delete(c.Profiles, name)
	if c.Active == name {
		c.Active = ""
	}
	return nil
}

// FindByServer returns the first profile whose server URL matches (after
// trimming trailing slashes). Used by `login` to decide whether to
// overwrite an existing profile rather than creating a duplicate.
func (c *Config) FindByServer(server string) (string, *Profile) {
	server = strings.TrimRight(server, "/")
	for name, p := range c.Profiles {
		if strings.TrimRight(p.Server, "/") == server {
			return name, p
		}
	}
	return "", nil
}

// ---- Require* helpers ---------------------------------------------------
//
// These return (config, profile) so callers that mutate (login, org use,
// init) can save the parent config after editing the profile. Read-only
// callers ignore the Config return.

// RequireAuth resolves the profile a command should act on: --profile
// wins, else the active profile. serverOverride applies --server in
// memory only (does not persist). Returns an actionable error when not
// logged in.
func RequireAuth(serverOverride, profileOverride string) (*Config, *Profile, error) {
	cfg, err := Load()
	if err != nil {
		return nil, nil, fmt.Errorf("not logged in — run 'conure login' first")
	}
	prof, err := resolveProfile(cfg, profileOverride)
	if err != nil {
		return nil, nil, err
	}
	if prof.Token == "" || prof.Server == "" {
		return nil, nil, fmt.Errorf("not logged in — run 'conure login' first")
	}
	if serverOverride != "" {
		// Copy so we don't accidentally persist the override to disk.
		p := *prof
		p.Server = serverOverride
		prof = &p
	}
	return cfg, prof, nil
}

// RequireActiveOrg adds the "active org must be set" check on top of
// RequireAuth.
func RequireActiveOrg(serverOverride, profileOverride string) (*Config, *Profile, error) {
	cfg, prof, err := RequireAuth(serverOverride, profileOverride)
	if err != nil {
		return nil, nil, err
	}
	if prof.ActiveOrg == "" {
		return nil, nil, fmt.Errorf("no active organization — run 'conure org use <name>' first")
	}
	return cfg, prof, nil
}

func resolveProfile(cfg *Config, profileOverride string) (*Profile, error) {
	if profileOverride != "" {
		p := cfg.Get(profileOverride)
		if p == nil {
			return nil, fmt.Errorf("no profile named %q — run 'conure profile list'", profileOverride)
		}
		return p, nil
	}
	if cfg.Active == "" {
		return nil, fmt.Errorf("not logged in — run 'conure login' first")
	}
	p := cfg.GetActive()
	if p == nil {
		return nil, fmt.Errorf("active profile %q is missing — run 'conure profile use <name>'", cfg.Active)
	}
	return p, nil
}
