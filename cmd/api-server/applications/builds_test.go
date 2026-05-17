package applications

import (
	"context"
	"errors"
	"testing"

	"github.com/coffeenights/conure/cmd/api-server/models"
	"github.com/coffeenights/conure/internal/fieldroles"
	"go.mongodb.org/mongo-driver/bson/primitive"
	corev1 "k8s.io/api/core/v1"
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

func TestBaseValuesForBuild_PrefersDraftValues(t *testing.T) {
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

	// A deployed baseline plus a newer draft: the build must layer the image
	// onto the draft's values (the user's prepped non-image edits win).
	deployed := &models.ComponentRevision{
		ComponentID:   component.ID,
		EnvironmentID: env.ID,
		Values:        map[string]interface{}{"replicas": float64(1)},
	}
	if err := deployed.CreateDeployed(context.Background(), testConf.app.MongoDB); err != nil {
		t.Fatal(err)
	}
	draft := &models.ComponentRevision{
		ComponentID:   component.ID,
		EnvironmentID: env.ID,
		Values:        map[string]interface{}{"replicas": float64(3)},
	}
	if err := draft.UpsertDraft(context.Background(), testConf.app.MongoDB); err != nil {
		t.Fatal(err)
	}

	values, err := baseValuesForBuild(context.Background(), testConf.app, component, env)
	if err != nil {
		t.Fatalf("baseValuesForBuild err: %v", err)
	}
	if got, ok := values["replicas"].(float64); !ok || got != 3 {
		t.Errorf("expected draft values to be preferred, got %+v", values)
	}
}

func TestBaseValuesForBuild_DeployedFallback(t *testing.T) {
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

	values, err := baseValuesForBuild(context.Background(), testConf.app, component, env)
	if err != nil {
		t.Fatalf("baseValuesForBuild err: %v", err)
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

	// Public path: no projected credentials.
	job := renderBuildJob(b, "", "")
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

	// Public path invariants: no registry-creds volume/mount, and the
	// clone is a plain `git clone` via Args (no token rewrite, no Env).
	if hasVolume(job.Spec.Template.Spec.Volumes, "registry-creds") {
		t.Error("public build must not mount a registry-creds volume (no hardcoded fallback)")
	}
	for _, vm := range bc.VolumeMounts {
		if vm.Name == "registry-creds" {
			t.Error("public build container must not mount registry-creds")
		}
	}
	if len(ic.Args) == 0 || ic.Args[0] != "clone" {
		t.Errorf("public clone should use plain `clone` Args, got Args=%v Command=%v", ic.Args, ic.Command)
	}
	if len(ic.Env) != 0 {
		t.Errorf("public clone must not inject git credential env, got %v", ic.Env)
	}
}

func hasVolume(volumes []corev1.Volume, name string) bool {
	for _, v := range volumes {
		if v.Name == name {
			return true
		}
	}
	return false
}

func TestRenderBuildJob_RegistryCredentialMountedWhenResolved(t *testing.T) {
	b := &models.Build{
		ComponentID: primitive.NewObjectID(), ApplicationID: primitive.NewObjectID(),
		BuildLocation: models.BuildLocationRemote,
		GitRepository: "https://github.com/org/repo", GitBranch: "main",
		ImageRef: "ghcr.io/org/app:tag",
	}
	b.ID = primitive.NewObjectID()

	job := renderBuildJob(b, "cred-org1-ghcr", "")
	if !hasVolume(job.Spec.Template.Spec.Volumes, "registry-creds") {
		t.Fatal("expected registry-creds volume when a registry Secret is resolved")
	}
	// The volume must reference the CONCRETE projected Secret name, not a
	// hardcoded one.
	for _, v := range job.Spec.Template.Spec.Volumes {
		if v.Name == "registry-creds" {
			if v.Secret == nil || v.Secret.SecretName != "cred-org1-ghcr" {
				t.Fatalf("registry-creds must point at the projected Secret, got %+v", v.Secret)
			}
		}
	}
	bc := job.Spec.Template.Spec.Containers[0]
	var mounted, hasDockerConfig bool
	for _, vm := range bc.VolumeMounts {
		if vm.Name == "registry-creds" && vm.MountPath == "/root/.docker" {
			mounted = true
		}
	}
	for _, e := range bc.Env {
		if e.Name == "DOCKER_CONFIG" {
			hasDockerConfig = true
		}
	}
	if !mounted || !hasDockerConfig {
		t.Fatalf("build container must mount the cred at /root/.docker with DOCKER_CONFIG set (mounted=%v env=%v)", mounted, hasDockerConfig)
	}
	// The git side was not requested → still a plain clone.
	if ic := job.Spec.Template.Spec.InitContainers[0]; len(ic.Args) == 0 || ic.Args[0] != "clone" {
		t.Errorf("git unaffected when only registry cred set; got Args=%v", ic.Args)
	}
}

func TestRenderBuildJob_GitCredentialRewritesCloneViaSecretEnv(t *testing.T) {
	b := &models.Build{
		ComponentID: primitive.NewObjectID(), ApplicationID: primitive.NewObjectID(),
		BuildLocation: models.BuildLocationRemote,
		GitRepository: "https://github.com/org/private", GitBranch: "main",
		ImageRef: "ghcr.io/org/app:tag",
	}
	b.ID = primitive.NewObjectID()

	job := renderBuildJob(b, "", "cred-org1-gh")
	ic := job.Spec.Template.Spec.InitContainers[0]

	// Private clone is script-based, not Args-based.
	if len(ic.Command) < 3 {
		t.Fatalf("private clone should be an sh -c script, got Command=%v Args=%v", ic.Command, ic.Args)
	}
	script := ic.Command[2]
	if !contains(script, "git clone") || !contains(script, "${GIT_USERNAME}:${GIT_TOKEN}@") {
		t.Errorf("clone script must rewrite the URL with token auth, got: %s", script)
	}
	// Token/username come from the projected git Secret via secretKeyRef,
	// never from plain values (so they don't leak into the Job spec).
	var sawUser, sawToken bool
	for _, e := range ic.Env {
		if e.Name == "GIT_USERNAME" && e.ValueFrom != nil && e.ValueFrom.SecretKeyRef != nil {
			if e.ValueFrom.SecretKeyRef.Name != "cred-org1-gh" || e.ValueFrom.SecretKeyRef.Key != "username" {
				t.Errorf("GIT_USERNAME secretKeyRef wrong: %+v", e.ValueFrom.SecretKeyRef)
			}
			sawUser = true
		}
		if e.Name == "GIT_TOKEN" && e.ValueFrom != nil && e.ValueFrom.SecretKeyRef != nil {
			if e.ValueFrom.SecretKeyRef.Name != "cred-org1-gh" || e.ValueFrom.SecretKeyRef.Key != "token" {
				t.Errorf("GIT_TOKEN secretKeyRef wrong: %+v", e.ValueFrom.SecretKeyRef)
			}
			sawToken = true
		}
		if e.Name == "GIT_TOKEN" && e.Value != "" {
			t.Error("git token must not be a literal env value")
		}
	}
	if !sawUser || !sawToken {
		t.Fatalf("expected GIT_USERNAME and GIT_TOKEN from secretKeyRef (user=%v token=%v)", sawUser, sawToken)
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

func TestBuildContainerStarted(t *testing.T) {
	mk := func(name string, state corev1.ContainerState) corev1.Pod {
		return corev1.Pod{Status: corev1.PodStatus{
			ContainerStatuses: []corev1.ContainerStatus{{Name: name, State: state}},
		}}
	}
	waiting := corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "PodInitializing"}}
	running := corev1.ContainerState{Running: &corev1.ContainerStateRunning{}}
	terminated := corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{}}

	cases := []struct {
		name string
		pod  corev1.Pod
		want bool
	}{
		{"build waiting", mk("build", waiting), false},
		{"build running", mk("build", running), true},
		{"build terminated", mk("build", terminated), true},
		{"no statuses", corev1.Pod{}, false},
		{"only non-build container", mk("git-clone", running), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pod := tc.pod
			if got := buildContainerStarted(&pod); got != tc.want {
				t.Errorf("buildContainerStarted() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestIsContainerWaitingErr(t *testing.T) {
	waiting := []error{
		errors.New(`container "build" in pod "build-x-y" is waiting to start: PodInitializing`),
		errors.New(`container "build" in pod "build-x-y" is waiting to start: ContainerCreating`),
		errors.New("some wrapper: PodInitializing"),
	}
	for _, e := range waiting {
		if !isContainerWaitingErr(e) {
			t.Errorf("isContainerWaitingErr(%q) = false, want true", e)
		}
	}
	notWaiting := []error{
		nil,
		errors.New("connection refused"),
		errors.New("pods \"build-x\" not found"),
	}
	for _, e := range notWaiting {
		if isContainerWaitingErr(e) {
			t.Errorf("isContainerWaitingErr(%v) = true, want false", e)
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
