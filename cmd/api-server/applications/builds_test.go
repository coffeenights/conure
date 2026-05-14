package applications

import (
	"testing"

	"github.com/coffeenights/conure/cmd/api-server/models"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestTriggerBuildRequest_Validate(t *testing.T) {
	cases := []struct {
		name string
		req  TriggerBuildRequest
		ok   bool
	}{
		{
			name: "local dockerfile is fine",
			req:  TriggerBuildRequest{BuildTool: "dockerfile", BuildLocation: "local", ImageRef: "x:1"},
			ok:   true,
		},
		{
			name: "local railpack is allowed",
			req:  TriggerBuildRequest{BuildTool: "railpack", BuildLocation: "local", ImageRef: "x:1"},
			ok:   true,
		},
		{
			name: "remote dockerfile needs git fields",
			req:  TriggerBuildRequest{BuildTool: "dockerfile", BuildLocation: "remote", ImageRef: "x:1"},
			ok:   false,
		},
		{
			name: "remote dockerfile with git is fine",
			req:  TriggerBuildRequest{BuildTool: "dockerfile", BuildLocation: "remote", GitRepository: "https://x", GitBranch: "main", ImageRef: "x:1"},
			ok:   true,
		},
		{
			name: "remote railpack is rejected",
			req:  TriggerBuildRequest{BuildTool: "railpack", BuildLocation: "remote", GitRepository: "https://x", GitBranch: "main", ImageRef: "x:1"},
			ok:   false,
		},
		{
			name: "missing image_ref always fails",
			req:  TriggerBuildRequest{BuildTool: "dockerfile", BuildLocation: "local"},
			ok:   false,
		},
		{
			name: "unknown build_tool fails",
			req:  TriggerBuildRequest{BuildTool: "kaniko", BuildLocation: "local", ImageRef: "x:1"},
			ok:   false,
		},
		{
			name: "unknown build_location fails",
			req:  TriggerBuildRequest{BuildTool: "dockerfile", BuildLocation: "elsewhere", ImageRef: "x:1"},
			ok:   false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.req.validate()
			if tc.ok && err != nil {
				t.Errorf("expected ok, got %v", err)
			}
			if !tc.ok && err == nil {
				t.Errorf("expected error, got nil")
			}
		})
	}
}

func TestSplitImageRef(t *testing.T) {
	cases := []struct {
		in       string
		wantRepo string
		wantTag  string
	}{
		{"ghcr.io/org/app:sha-abc", "ghcr.io/org/app", "sha-abc"},
		{"docker.io/library/nginx:1.25", "docker.io/library/nginx", "1.25"},
		{"ghcr.io/org/app", "", ""}, // no tag
		{"ghcr.io:5000/org/app:tag", "ghcr.io:5000/org/app", "tag"},
		{"ghcr.io:5000/org/app", "", ""}, // port colon, no tag
		{"", "", ""},
	}
	for _, tc := range cases {
		repo, tag := splitImageRef(tc.in)
		if repo != tc.wantRepo || tag != tc.wantTag {
			t.Errorf("splitImageRef(%q) = (%q, %q), want (%q, %q)", tc.in, repo, tag, tc.wantRepo, tc.wantTag)
		}
	}
}

func TestMergeImageIntoValues_PreservesOtherKeys(t *testing.T) {
	values := map[string]interface{}{
		"resources": map[string]interface{}{"replicas": 3},
		"source": map[string]interface{}{
			"ociRepository": "old/img",
			"tag":           "v1",
			"command":       []string{"sleep", "1"},
		},
	}
	out := mergeImageIntoValues(values, "new/img", "v2")
	src := out["source"].(map[string]interface{})
	if src["ociRepository"] != "new/img" || src["tag"] != "v2" {
		t.Errorf("unexpected source: %v", src)
	}
	if cmd, ok := src["command"].([]string); !ok || len(cmd) != 2 {
		t.Errorf("command should survive merge: %v", src["command"])
	}
	if _, ok := out["resources"].(map[string]interface{}); !ok {
		t.Errorf("resources should be preserved")
	}
}

func TestMergeImageIntoValues_EmptyMap(t *testing.T) {
	out := mergeImageIntoValues(nil, "repo", "tag")
	src, ok := out["source"].(map[string]interface{})
	if !ok {
		t.Fatalf("source missing: %v", out)
	}
	if src["ociRepository"] != "repo" || src["tag"] != "tag" {
		t.Errorf("got %+v", src)
	}
}

func TestEnvNameByID(t *testing.T) {
	app := &models.Application{
		Environments: []models.Environment{
			{ID: "a", Name: "dev"},
			{ID: "b", Name: "prod"},
		},
	}
	if envNameByID(app, "b") != "prod" {
		t.Errorf("expected prod for b")
	}
	if envNameByID(app, "missing") != "" {
		t.Errorf("expected empty string for missing")
	}
}

func TestRenderBuildJob_LabelsAndStructure(t *testing.T) {
	b := &models.Build{
		ComponentID:   primitive.NewObjectID(),
		ApplicationID: primitive.NewObjectID(),
		BuildTool:     models.BuildToolDockerfile,
		BuildLocation: models.BuildLocationRemote,
		GitRepository: "https://github.com/org/repo",
		GitBranch:     "main",
		ImageRef:      "ghcr.io/org/app:tag",
		Platform:      "linux/amd64",
	}
	b.ID = primitive.NewObjectID()

	job := renderBuildJob(b)
	if job.Namespace != SystemNamespace {
		t.Errorf("namespace: got %s want %s", job.Namespace, SystemNamespace)
	}
	if job.Labels["conure.io/build-id"] != b.ID.Hex() {
		t.Errorf("build-id label missing/wrong")
	}
	if len(job.Spec.Template.Spec.InitContainers) != 1 {
		t.Fatalf("expected one initContainer")
	}
	if len(job.Spec.Template.Spec.Containers) != 1 {
		t.Fatalf("expected one container")
	}
	ic := job.Spec.Template.Spec.InitContainers[0]
	if ic.Image != "alpine/git:latest" {
		t.Errorf("init image: %s", ic.Image)
	}
	bc := job.Spec.Template.Spec.Containers[0]
	if bc.SecurityContext == nil || bc.SecurityContext.Privileged == nil || !*bc.SecurityContext.Privileged {
		t.Errorf("build container must be privileged")
	}
	// Script should reference image_ref and platform.
	if len(bc.Command) < 3 {
		t.Fatalf("unexpected command shape: %v", bc.Command)
	}
	script := bc.Command[2]
	for _, want := range []string{b.ImageRef, "buildkitd", "buildctl", "linux/amd64"} {
		if !contains(script, want) {
			t.Errorf("script missing %q: %s", want, script)
		}
	}
}

func TestIsValidPlatform(t *testing.T) {
	valid := []string{"", "linux/amd64", "linux/arm64", "linux/arm64/v8", "linux/386"}
	for _, p := range valid {
		if !isValidPlatform(p) {
			t.Errorf("isValidPlatform(%q) = false, want true", p)
		}
	}
	invalid := []string{
		"linux/amd64; rm -rf /",
		"linux/amd64 --opt evil=1",
		"linux/amd64`id`",
		"linux/amd64$IFS",
		"linux",
		"linux/",
		"/amd64",
		"linux//amd64",
		"linux/amd64/v8/extra",
		"linux/amd 64",
	}
	for _, p := range invalid {
		if isValidPlatform(p) {
			t.Errorf("isValidPlatform(%q) = true, want false", p)
		}
	}
}

func TestShellQuote(t *testing.T) {
	cases := map[string]string{
		"simple":  `'simple'`,
		"":        `''`,
		"a'b":     `'a'\''b'`,
		"a b c":   `'a b c'`,
		`semi;rm`: `'semi;rm'`,
	}
	for in, want := range cases {
		if got := shellQuote(in); got != want {
			t.Errorf("shellQuote(%q) = %q, want %q", in, got, want)
		}
	}
}

// contains is a substring check that avoids pulling in strings just to satisfy
// the import block alongside the rest of the file.
func contains(haystack, needle string) bool {
	if needle == "" {
		return true
	}
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
