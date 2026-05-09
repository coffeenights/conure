package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Link is the per-repo, machine-written state file at .conure/link.json.
// It points at one app + one component in one default env. Written only
// after the relevant API calls succeed.
type Link struct {
	OrgID         string `json:"org_id"`
	AppID         string `json:"app_id"`
	ComponentID   string `json:"component_id"`
	ComponentName string `json:"component_name"`
	Environment   string `json:"environment"`
}

const linkRelPath = ".conure/link.json"

// linkPath walks up from cwd to find an existing .conure/link.json. Falls back
// to cwd/.conure/link.json if none is found (used for write).
func linkPath() (string, bool, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", false, err
	}
	dir := cwd
	for {
		candidate := filepath.Join(dir, linkRelPath)
		if _, err := os.Stat(candidate); err == nil {
			return candidate, true, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return filepath.Join(cwd, linkRelPath), false, nil
}

func loadLink() (*Link, error) {
	path, found, err := linkPath()
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

func saveLink(l *Link) error {
	path, _, err := linkPath()
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

// linkExists reports whether a link is reachable from cwd, without loading it.
func linkExists() bool {
	_, found, err := linkPath()
	return err == nil && found
}

// requireLink loads the link or returns an actionable error.
func requireLink() (*Link, error) {
	return loadLink()
}
