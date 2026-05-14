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
	yes := func() bool { return true }
	no := func() bool { return false }
	cases := []struct {
		name            string
		clusterPlatform string
		gitRepo         string
		hasBuildx       func() bool
		want            string
	}{
		{"git supplied means remote", "linux/amd64", "https://x", yes, "remote"},
		{"git supplied still remote even without buildx", "linux/amd64", "https://x", no, "remote"},
		{"no buildx routes to remote", "", "", no, "remote"},
		{"no buildx routes to remote even on same arch", "linux/" + runtime.GOARCH, "", no, "remote"},
		{"unknown cluster falls back to local when buildx present", "", "", yes, "local"},
		{"same arch is local when buildx present", "linux/" + runtime.GOARCH, "", yes, "local"},
		{"cross arch is local when buildx present", "linux/wrongarch", "", yes, "local"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := pickBuildLocationWith(tc.clusterPlatform, tc.gitRepo, tc.hasBuildx)
			if got != tc.want {
				t.Errorf("pickBuildLocationWith(%q,%q,buildx=%v) = %q, want %q", tc.clusterPlatform, tc.gitRepo, tc.hasBuildx(), got, tc.want)
			}
		})
	}
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
