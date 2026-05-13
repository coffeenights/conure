// Package link owns the per-repo .conure/link.json file. It maps each
// CLI profile to the org/app/component/env that this directory is pinned
// to under that profile, so one checkout can deploy to multiple servers
// at once. The file is machine-written by `conure init` and machine-read
// by every other linked-component command.
package link

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Link is one profile's pin: which org/app/component/env in this dir
// belongs to that profile. The owning profile is the map key in the
// on-disk file, not a field here.
type Link struct {
	OrgID         string `json:"org_id"`
	AppID         string `json:"app_id"`
	ComponentID   string `json:"component_id"`
	ComponentName string `json:"component_name"`
	Environment   string `json:"environment"`
}

// File is the on-disk shape: profile name → link entry. The map is keyed
// by profile name so duplicates within a directory are impossible by
// construction.
type File map[string]*Link

const relPath = ".conure/link.json"

// Path walks up from cwd looking for an existing .conure/link.json so a
// command run from a subdirectory still resolves to the repo's link. When
// nothing is found, returns cwd/.conure/link.json so callers writing a new
// link have a sensible default. The bool reports whether a file was found.
func Path() (string, bool, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", false, err
	}
	dir := cwd
	for {
		candidate := filepath.Join(dir, relPath)
		if _, err := os.Stat(candidate); err == nil {
			return candidate, true, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return filepath.Join(cwd, relPath), false, nil
}

// Load reads the link file as a profile-keyed map. Returns an actionable
// error when no file is reachable from cwd.
func Load() (File, error) {
	path, found, err := Path()
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, errors.New("no .conure/link.json found — run 'conure init' first")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var f File
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	if f == nil {
		f = File{}
	}
	return f, nil
}

// LoadOrEmpty returns Load()'s result, or an empty File when the file is
// missing. `conure init` uses this so the first run can write the file
// from scratch.
func LoadOrEmpty() (File, error) {
	if !FileExists() {
		return File{}, nil
	}
	return Load()
}

// Save writes the map to whichever path Path() resolves to. Existing
// directories are reused; missing ones are created with 0755.
func Save(f File) error {
	path, _, err := Path()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	if f == nil {
		f = File{}
	}
	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// Get returns the entry for profile or an actionable error explaining
// how to add one.
func Get(profile string) (*Link, error) {
	f, err := Load()
	if err != nil {
		return nil, err
	}
	l, ok := f[profile]
	if !ok {
		return nil, fmt.Errorf(
			"no link for profile %q in .conure/link.json — run 'conure init' (with the %q profile active) to add one",
			profile, profile,
		)
	}
	return l, nil
}

// Exists reports whether the link file has an entry for the given
// profile.
func Exists(profile string) bool {
	f, err := Load()
	if err != nil {
		return false
	}
	_, ok := f[profile]
	return ok
}

// FileExists reports whether the link file is present at all, regardless
// of which profiles it pins. Useful for "is this a linked dir?" UX
// defaults where the caller doesn't yet have a profile name in hand.
func FileExists() bool {
	_, found, err := Path()
	return err == nil && found
}
