package main

import (
	"runtime"
	"testing"

	"github.com/coffeenights/conure/pkg/api"
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

func TestFirstNonEmpty(t *testing.T) {
	cases := []struct {
		name string
		in   []string
		want string
	}{
		{"first wins", []string{"a", "b", "c"}, "a"},
		{"skips empty", []string{"", "", "c"}, "c"},
		{"all empty", []string{"", "", ""}, ""},
		{"single non-empty", []string{"only"}, "only"},
		{"empty list", nil, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := firstNonEmpty(tc.in...); got != tc.want {
				t.Errorf("firstNonEmpty(%v) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestStringFromValues(t *testing.T) {
	m := map[string]interface{}{
		"a":   "yes",
		"num": 3,
		"nil": nil,
	}
	if stringFromValues(m, "a") != "yes" {
		t.Errorf("expected yes")
	}
	if stringFromValues(m, "num") != "" {
		t.Errorf("non-string should yield empty")
	}
	if stringFromValues(m, "missing") != "" {
		t.Errorf("missing key should yield empty")
	}
	if stringFromValues(m, "nil") != "" {
		t.Errorf("nil value should yield empty")
	}
}

func TestPickValues_PrefersDraftOverDeployed(t *testing.T) {
	draftValues := map[string]interface{}{"flag": "draft"}
	deployedValues := map[string]interface{}{"flag": "deployed"}
	view := &api.ComponentInEnvResponse{
		LatestDraft:      &api.ComponentRevision{Values: draftValues},
		DeployedRevision: &api.ComponentRevision{Values: deployedValues},
	}
	got := pickValues(view)
	if got["flag"] != "draft" {
		t.Errorf("expected draft to win, got %v", got)
	}
}

func TestPickValues_FallsBackToDeployed(t *testing.T) {
	deployedValues := map[string]interface{}{"flag": "deployed"}
	view := &api.ComponentInEnvResponse{
		DeployedRevision: &api.ComponentRevision{Values: deployedValues},
	}
	got := pickValues(view)
	if got["flag"] != "deployed" {
		t.Errorf("expected deployed fallback, got %v", got)
	}
}

func TestPickValues_NoRevisionsReturnsNil(t *testing.T) {
	view := &api.ComponentInEnvResponse{}
	if got := pickValues(view); got != nil {
		t.Errorf("expected nil for empty view, got %v", got)
	}
}
