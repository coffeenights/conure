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

func TestDecideDeployAction(t *testing.T) {
	cases := []struct {
		name       string
		spec       componentSourceSpec
		flagImage  string
		wantAction deployAction
		wantImage  string
		wantErr    bool
	}{
		{
			name:       "not buildable, no flag -> promote",
			spec:       componentSourceSpec{Buildable: false},
			wantAction: deployActionPromote,
		},
		{
			name:       "buildable git with spec image -> build from spec",
			spec:       componentSourceSpec{Buildable: true, SourceType: "git", OCIRepository: "ghcr.io/me/app", Tag: "v1"},
			wantAction: deployActionBuild,
			wantImage:  "ghcr.io/me/app:v1",
		},
		{
			name:    "buildable git but spec image incomplete -> error",
			spec:    componentSourceSpec{Buildable: true, SourceType: "git", OCIRepository: "ghcr.io/me/app"},
			wantErr: true,
		},
		{
			name:       "buildable oci, no flag -> promote (deploy prebuilt)",
			spec:       componentSourceSpec{Buildable: true, SourceType: "oci", OCIRepository: "ghcr.io/me/app", Tag: "v1"},
			wantAction: deployActionPromote,
		},
		{
			name:       "flag image overrides oci -> build",
			spec:       componentSourceSpec{Buildable: true, SourceType: "oci"},
			flagImage:  "ghcr.io/me/app:override",
			wantAction: deployActionBuild,
			wantImage:  "ghcr.io/me/app:override",
		},
		{
			name:       "flag image wins over spec image",
			spec:       componentSourceSpec{Buildable: true, SourceType: "git", OCIRepository: "ghcr.io/me/app", Tag: "spec"},
			flagImage:  "ghcr.io/me/app:flag",
			wantAction: deployActionBuild,
			wantImage:  "ghcr.io/me/app:flag",
		},
		{
			name:      "flag image on non-buildable -> error",
			spec:      componentSourceSpec{Buildable: false},
			flagImage: "ghcr.io/me/app:x",
			wantErr:   true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			action, image, err := decideDeployAction(tc.spec, tc.flagImage, "mycomp")
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got action=%v image=%q", action, image)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if action != tc.wantAction {
				t.Errorf("action = %v, want %v", action, tc.wantAction)
			}
			if image != tc.wantImage {
				t.Errorf("image = %q, want %q", image, tc.wantImage)
			}
		})
	}
}

func TestComponentSourceSpec_ImageRef(t *testing.T) {
	if got := (componentSourceSpec{OCIRepository: "r", Tag: "t"}).ImageRef(); got != "r:t" {
		t.Errorf("ImageRef = %q, want r:t", got)
	}
	if got := (componentSourceSpec{OCIRepository: "r"}).ImageRef(); got != "" {
		t.Errorf("ImageRef with no tag = %q, want empty", got)
	}
	if got := (componentSourceSpec{Tag: "t"}).ImageRef(); got != "" {
		t.Errorf("ImageRef with no repo = %q, want empty", got)
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
