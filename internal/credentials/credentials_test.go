package credentials

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"

	k8sUtils "github.com/coffeenights/conure/internal/k8s"
)

func TestSecretName_DeterministicAndScoped(t *testing.T) {
	a := SecretName("org1", "ghcr")
	if a != SecretName("org1", "ghcr") {
		t.Fatal("SecretName must be deterministic")
	}
	if SecretName("org1", "ghcr") == SecretName("org2", "ghcr") {
		t.Fatal("same cred name in different orgs must not collide")
	}
	if !strings.HasPrefix(a, "cred-org1-") {
		t.Fatalf("name should embed org id, got %q", a)
	}
}

func TestSanitize_RFC1123Subset(t *testing.T) {
	cases := map[string]string{
		"My Cred":    "my-cred",
		"GHCR_token": "ghcr-token",
		"a.b/c":      "a-b-c",
		"--edge--":   "edge",
		"":           "x",
		"UPPER":      "upper",
		"ok-1":       "ok-1",
	}
	for in, want := range cases {
		if got := sanitize(in); got != want {
			t.Errorf("sanitize(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestRegistryHost(t *testing.T) {
	cases := map[string]string{
		"ghcr.io":                 "ghcr.io",
		"https://ghcr.io":         "ghcr.io",
		"http://ghcr.io":          "ghcr.io",
		"oci://reg.example.com/x": "reg.example.com",
		"ghcr.io/org/img":         "ghcr.io",
		"reg.example.com:5000/x":  "reg.example.com:5000",
	}
	for in, want := range cases {
		if got := RegistryHost(in); got != want {
			t.Errorf("RegistryHost(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestDockerConfigJSON_HostKeyedWithBothFields(t *testing.T) {
	raw, err := DockerConfigJSON("ghcr.io/myorg", "octocat", "s3cr3t")
	if err != nil {
		t.Fatal(err)
	}
	var cfg struct {
		Auths map[string]struct {
			Username, Password, Auth string
		} `json:"auths"`
	}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatalf("not valid json: %v", err)
	}
	entry, ok := cfg.Auths["ghcr.io"]
	if !ok {
		t.Fatalf("auths must be keyed by host, got keys %v", cfg.Auths)
	}
	if entry.Username != "octocat" || entry.Password != "s3cr3t" {
		t.Fatalf("user/pass not populated: %+v", entry)
	}
	dec, _ := base64.StdEncoding.DecodeString(entry.Auth)
	if string(dec) != "octocat:s3cr3t" {
		t.Fatalf("auth field = %q, want octocat:s3cr3t", dec)
	}
}

func TestRegistrySecret_ShapeAndLabels(t *testing.T) {
	s, err := RegistrySecret("org1", "ghcr", "ghcr.io", "octocat", "tok")
	if err != nil {
		t.Fatal(err)
	}
	if s.Type != corev1.SecretTypeDockerConfigJson {
		t.Fatalf("type = %s, want dockerconfigjson", s.Type)
	}
	if _, ok := s.Data[corev1.DockerConfigJsonKey]; !ok {
		t.Fatalf("missing %s key", corev1.DockerConfigJsonKey)
	}
	if s.Labels[k8sUtils.OrganizationIDLabel] != "org1" {
		t.Errorf("org label = %q, want org1", s.Labels[k8sUtils.OrganizationIDLabel])
	}
	if s.Labels[CredentialKindLabel] != "registry" {
		t.Errorf("kind label = %q, want registry", s.Labels[CredentialKindLabel])
	}
	if s.Labels[k8sUtils.CreatedByLabel] != "conure" {
		t.Errorf("created-by label = %q, want conure", s.Labels[k8sUtils.CreatedByLabel])
	}
}

func TestRegistrySecretNamed_UsesGivenNameKeepsShapeAndLabels(t *testing.T) {
	// The deploy-time pull secret is named after the credential itself so a
	// user template can reference it directly — NOT the internal
	// SecretName(orgID, credName) scheme.
	s, err := RegistrySecretNamed("mredvard-registry", "org1", "mredvard-registry", "ghcr.io", "octocat", "tok")
	if err != nil {
		t.Fatal(err)
	}
	if s.Name != "mredvard-registry" {
		t.Fatalf("Secret name = %q, want the given name %q (not the cred-<org>-<name> scheme)", s.Name, "mredvard-registry")
	}
	if got := SecretName("org1", "mredvard-registry"); s.Name == got {
		t.Fatalf("Secret name must NOT be the internal SecretName scheme %q", got)
	}
	if s.Type != corev1.SecretTypeDockerConfigJson {
		t.Fatalf("type = %s, want dockerconfigjson", s.Type)
	}
	if _, ok := s.Data[corev1.DockerConfigJsonKey]; !ok {
		t.Fatalf("missing %s key", corev1.DockerConfigJsonKey)
	}
	// Org isolation + traceability labels are unchanged from RegistrySecret.
	if s.Labels[k8sUtils.OrganizationIDLabel] != "org1" {
		t.Errorf("org label = %q, want org1", s.Labels[k8sUtils.OrganizationIDLabel])
	}
	if s.Labels[CredentialKindLabel] != "registry" {
		t.Errorf("kind label = %q, want registry", s.Labels[CredentialKindLabel])
	}
	if s.Labels[k8sUtils.CreatedByLabel] != "conure" {
		t.Errorf("created-by label = %q, want conure", s.Labels[k8sUtils.CreatedByLabel])
	}
}

func TestGitSecret_DefaultsUsernameAndCarriesToken(t *testing.T) {
	s := GitSecret("org1", "gh", "", "ghp_xxx")
	if s.Type != corev1.SecretTypeOpaque {
		t.Fatalf("type = %s, want Opaque", s.Type)
	}
	if s.StringData["username"] != "x-access-token" {
		t.Errorf("default username = %q, want x-access-token", s.StringData["username"])
	}
	if s.StringData["token"] != "ghp_xxx" {
		t.Errorf("token = %q, want ghp_xxx", s.StringData["token"])
	}

	s2 := GitSecret("org1", "gh", "deploy-bot", "t")
	if s2.StringData["username"] != "deploy-bot" {
		t.Errorf("explicit username = %q, want deploy-bot", s2.StringData["username"])
	}
	if s2.Labels[CredentialKindLabel] != "git" {
		t.Errorf("kind label = %q, want git", s2.Labels[CredentialKindLabel])
	}
}
