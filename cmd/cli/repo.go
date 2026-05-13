package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// detectComponentName picks a friendly default name for a new component:
//
//  1. the repo segment of `git config --get remote.origin.url`, if a git
//     origin is configured;
//  2. otherwise the cwd's basename.
//
// Returns the empty string only if cwd cannot be read.
func detectComponentName() string {
	if name := repoNameFromGitOrigin(); name != "" {
		return name
	}
	return cwdBasename()
}

func repoNameFromGitOrigin() string {
	out, err := exec.Command("git", "config", "--get", "remote.origin.url").Output()
	if err != nil {
		return ""
	}
	url := strings.TrimSpace(string(out))
	if url == "" {
		return ""
	}
	// Strip trailing .git, then take the last path segment. Handles both
	// https://host/owner/repo(.git) and git@host:owner/repo(.git).
	url = strings.TrimSuffix(url, ".git")
	for _, sep := range []string{"/", ":"} {
		if i := strings.LastIndex(url, sep); i >= 0 {
			url = url[i+1:]
		}
	}
	return url
}

func cwdBasename() string {
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}
	return filepath.Base(cwd)
}
