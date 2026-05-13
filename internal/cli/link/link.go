// Package link owns the per-repo .conure/link.json file that pins a
// directory to a single org/app/component/env. It's intentionally tiny:
// the file is machine-written by `conure init` and machine-read by every
// other linked-component command.
package link

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Link is the on-disk shape of .conure/link.json. Field tags match what
// `conure init` writes; do not rename without a migration.
type Link struct {
	OrgID         string `json:"org_id"`
	AppID         string `json:"app_id"`
	ComponentID   string `json:"component_id"`
	ComponentName string `json:"component_name"`
	Environment   string `json:"environment"`
}

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

// Load reads the link reachable from cwd, or returns an actionable error
// telling the user to run `conure init` first.
func Load() (*Link, error) {
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
	var l Link
	if err := json.Unmarshal(data, &l); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	return &l, nil
}

// Save writes the link to whichever path Path() resolves to. Existing
// directories are reused; missing ones are created with 0755.
func Save(l *Link) error {
	path, _, err := Path()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(l, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// Exists reports whether a link is reachable from cwd without loading it.
// Useful for the `conure init` guard that refuses to overwrite.
func Exists() bool {
	_, found, err := Path()
	return err == nil && found
}
