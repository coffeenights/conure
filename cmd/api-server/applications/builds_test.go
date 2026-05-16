package applications

import (
	"context"
	"testing"

	"github.com/coffeenights/conure/cmd/api-server/models"
	"github.com/coffeenights/conure/internal/fieldroles"
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
		{
			name: "local with image_ref missing tag is rejected up front",
			req:  TriggerBuildRequest{BuildTool: "dockerfile", BuildLocation: "local", ImageRef: "ghcr.io/org/app"},
			ok:   false,
		},
		{
			name: "remote with image_ref missing tag is rejected up front",
			req:  TriggerBuildRequest{BuildTool: "dockerfile", BuildLocation: "remote", GitRepository: "https://x", GitBranch: "main", ImageRef: "ghcr.io/org/app"},
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

// webserviceImageResolver mirrors the in-repo webservice ComponentDefinition:
// buildable, image.* mapped into the `source.*` block.
func webserviceImageResolver() *fieldroles.Resolver {
	return fieldroles.New(true, map[string]string{
		fieldroles.RoleImageRepository: "source.ociRepository",
		fieldroles.RoleImageTag:        "source.tag",
	})
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
	out, err := mergeImageIntoValues(webserviceImageResolver(), values, "new/img", "v2")
	if err != nil {
		t.Fatalf("mergeImageIntoValues: %v", err)
	}
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
	out, err := mergeImageIntoValues(webserviceImageResolver(), nil, "repo", "tag")
	if err != nil {
		t.Fatalf("mergeImageIntoValues: %v", err)
	}
	src, ok := out["source"].(map[string]interface{})
	if !ok {
		t.Fatalf("source missing: %v", out)
	}
	if src["ociRepository"] != "repo" || src["tag"] != "tag" {
		t.Errorf("got %+v", src)
	}
}

func TestMergeImageIntoValues_CustomPath(t *testing.T) {
	// A definition that maps the image somewhere other than source.*.
	r := fieldroles.New(true, map[string]string{
		fieldroles.RoleImageRepository: "image.repository",
		fieldroles.RoleImageTag:        "image.tag",
	})
	out, err := mergeImageIntoValues(r, map[string]interface{}{}, "ghcr.io/x/y", "sha-1")
	if err != nil {
		t.Fatalf("mergeImageIntoValues: %v", err)
	}
	img := out["image"].(map[string]interface{})
	if img["repository"] != "ghcr.io/x/y" || img["tag"] != "sha-1" {
		t.Errorf("custom path not honored: %+v", img)
	}
}

func TestMergeImageIntoValues_UndeclaredRoleErrors(t *testing.T) {
	// Definition omits the image roles entirely — strict, no fallback.
	r := fieldroles.New(true, map[string]string{})
	if _, err := mergeImageIntoValues(r, map[string]interface{}{}, "repo", "tag"); err == nil {
		t.Fatal("expected an error when image roles are undeclared")
	}
}

func TestBaseValuesForBuild_DraftIsReturnedForPromotion(t *testing.T) {
	_, app, env := orgWithApp(t, "TestBaseValuesForBuild_Draft", "staging")

	component := &models.Component{
		Name:          "buildsrc",
		Type:          "service",
		ApplicationID: app.ID,
	}
	if err := component.Create(testConf.app.MongoDB); err != nil {
		t.Fatal(err)
	}
	cleanupComponent(t, component)

	draft := &models.ComponentRevision{
		ComponentID:   component.ID,
		EnvironmentID: env.ID,
		Values:        map[string]interface{}{"replicas": float64(3)},
	}
	if err := draft.CreateDraft(context.Background(), testConf.app.MongoDB); err != nil {
		t.Fatal(err)
	}

	values, returnedDraft, err := baseValuesForBuild(context.Background(), testConf.app, component, env)
	if err != nil {
		t.Fatalf("baseValuesForBuild err: %v", err)
	}
	if returnedDraft == nil || returnedDraft.ID != draft.ID {
		t.Fatalf("expected draft to be returned for caller-side promotion, got %v", returnedDraft)
	}
	if got, ok := values["replicas"].(float64); !ok || got != 3 {
		t.Errorf("draft values not copied: %+v", values)
	}
}

func TestBaseValuesForBuild_DeployedFallback_NoDraftReturned(t *testing.T) {
	_, app, env := orgWithApp(t, "TestBaseValuesForBuild_Deployed", "staging")

	component := &models.Component{
		Name:          "buildsrc-deployed",
		Type:          "service",
		ApplicationID: app.ID,
	}
	if err := component.Create(testConf.app.MongoDB); err != nil {
		t.Fatal(err)
	}
	cleanupComponent(t, component)

	deployed := &models.ComponentRevision{
		ComponentID:   component.ID,
		EnvironmentID: env.ID,
		Values:        map[string]interface{}{"replicas": float64(5)},
	}
	if err := deployed.CreateDeployed(context.Background(), testConf.app.MongoDB); err != nil {
		t.Fatal(err)
	}

	values, returnedDraft, err := baseValuesForBuild(context.Background(), testConf.app, component, env)
	if err != nil {
		t.Fatalf("baseValuesForBuild err: %v", err)
	}
	if returnedDraft != nil {
		t.Errorf("expected nil draft when only a deployed revision exists, got %v", returnedDraft)
	}
	if got, ok := values["replicas"].(float64); !ok || got != 5 {
		t.Errorf("deployed values not copied: %+v", values)
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
	if job.Spec.ActiveDeadlineSeconds == nil || *job.Spec.ActiveDeadlineSeconds != buildJobDeadline {
		t.Errorf("expected ActiveDeadlineSeconds=%d, got %v", buildJobDeadline, job.Spec.ActiveDeadlineSeconds)
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
