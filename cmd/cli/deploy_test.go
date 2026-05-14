package main

import (
	"runtime"
	"testing"
)

func TestNormalizePlatform(t *testing.T) {
	cases := map[string]string{
		"darwin/arm64": "linux/arm64",
		"linux/amd64":  "linux/amd64",
		"darwin/amd64": "linux/amd64",
		"weirdvalue":   "weirdvalue",
	}
	for in, want := range cases {
		if got := normalizePlatform(in); got != want {
			t.Errorf("normalizePlatform(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestPickBuildLocation(t *testing.T) {
	cases := []struct {
		name            string
		clusterPlatform string
		gitRepo         string
		want            string
	}{
		{"git supplied means remote", "linux/amd64", "https://x", "remote"},
		{"unknown cluster falls back to local", "", "", "local"},
		// We can't reliably exercise the buildx branch in a unit test, so we
		// only check the cases that don't depend on docker being installed.
		{"same arch is local", "linux/" + goarchForTest(), "", "local"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := pickBuildLocation(tc.clusterPlatform, tc.gitRepo)
			// In the "same arch" case, only assert == local; in the other
			// cases the expectation is fixed.
			if got != tc.want {
				t.Errorf("pickBuildLocation(%q,%q) = %q, want %q", tc.clusterPlatform, tc.gitRepo, got, tc.want)
			}
		})
	}
}

// goarchForTest exists so we can build a canonical "same arch" cluster
// platform string for the current host. Avoids importing runtime in the
// table.
func goarchForTest() string {
	return runtime.GOARCH
}
