// Package credentials owns the naming/label scheme and the Kubernetes Secret
// shapes for conure's org-scoped credentials, shared by the deploy-time
// projection (api-server providers) and the build Job wiring so there is one
// implementation of "what does a projected credential Secret look like".
//
// Credentials live in MongoDB as the source of truth (models.Credential,
// AES-encrypted). At deploy time the resolved, decrypted credential is
// projected into a Kubernetes Secret in the controller/system namespace,
// named and labelled by org. Kubernetes has no native org concept, so org
// isolation is enforced by conure: the name embeds the org id and the
// OrganizationIDLabel carries it for selection.
package credentials

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	k8sUtils "github.com/coffeenights/conure/internal/k8s"
)

// Label keys on a projected credential Secret. OrganizationIDLabel /
// CreatedByLabel are reused from the shared k8s vocabulary so projected
// credentials are selectable exactly like the materialized ComponentDefinition
// CRDs. The credential-name / credential-kind labels make a Secret traceable
// back to its logical credential without decoding the name.
const (
	CredentialNameLabel = "conure.io/credential-name"
	CredentialKindLabel = "conure.io/credential-kind"
)

// SystemNamespace is where projected credential Secrets are written. It is
// the controller's namespace — the same place ComponentDefinition pull
// secrets are looked up (RegistrySecretNamespace, set from POD_NAMESPACE)
// and where the build Job runs — so a single literal isn't duplicated across
// the projection, the controller, and the build path.
const SystemNamespace = "conure-system"

// SecretName is the deterministic projected-Secret name for one org's logical
// credential: cred-<orgID>-<credName>. The org id prefix keeps two orgs'
// same-named credentials from colliding in the flat (namespaced) Secret
// space; the OrganizationIDLabel is the actual isolation key conure filters
// on. credName is sanitized to the RFC1123 subset Kubernetes allows in a
// Secret name.
func SecretName(orgID, credName string) string {
	return fmt.Sprintf("cred-%s-%s", sanitize(orgID), sanitize(credName))
}

// sanitize lowercases and replaces any character outside [a-z0-9-] with '-'
// so an arbitrary logical credential name is a legal Secret name segment.
// It does not need to be reversible — the labels carry the real name.
func sanitize(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-':
			b.WriteRune(r)
		default:
			b.WriteRune('-')
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "x"
	}
	return out
}

func labels(orgID, credName, kind string) map[string]string {
	return map[string]string{
		k8sUtils.OrganizationIDLabel: orgID,
		k8sUtils.CreatedByLabel:      "conure",
		CredentialNameLabel:          sanitize(credName),
		CredentialKindLabel:          kind,
	}
}

// dockerConfigJSON / dockerConfigAuth mirror the kubernetes.io/dockerconfigjson
// payload. The controller's pull-credential resolver and BuildKit both read
// this exact shape; the entry is keyed by registry HOST (the controller
// normalizes auth keys to the host before matching).
type dockerConfigJSON struct {
	Auths map[string]dockerConfigAuth `json:"auths"`
}

type dockerConfigAuth struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Auth     string `json:"auth"`
}

// RegistryHost extracts the registry hostname from a registry URL or an OCI
// repository reference ("ghcr.io", "https://ghcr.io", "oci://r.io/path",
// "ghcr.io/org/img"). The dockerconfigjson entry must be keyed by host so the
// controller's host-normalized lookup matches.
func RegistryHost(registry string) string {
	registry = strings.TrimPrefix(registry, "oci://")
	if u, err := url.Parse("https://" + strings.TrimPrefix(strings.TrimPrefix(registry, "https://"), "http://")); err == nil && u.Host != "" {
		return u.Host
	}
	if i := strings.Index(registry, "/"); i > 0 {
		return registry[:i]
	}
	return registry
}

// DockerConfigJSON builds the .dockerconfigjson bytes for a registry
// credential, keyed by host, with both username/password and the base64
// auth field populated (different tools read different fields; the controller
// accepts either).
func DockerConfigJSON(registry, username, password string) ([]byte, error) {
	host := RegistryHost(registry)
	if host == "" {
		return nil, fmt.Errorf("cannot derive registry host from %q", registry)
	}
	auth := base64.StdEncoding.EncodeToString([]byte(username + ":" + password))
	cfg := dockerConfigJSON{Auths: map[string]dockerConfigAuth{
		host: {Username: username, Password: password, Auth: auth},
	}}
	return json.Marshal(cfg)
}

// RegistrySecret builds the projected dockerconfigjson Secret for a registry
// credential. It is consumed both by the controller (private OCI module pull)
// and the build Job (image push) — same shape, different mounts.
func RegistrySecret(orgID, credName, registry, username, password string) (*corev1.Secret, error) {
	dcj, err := DockerConfigJSON(registry, username, password)
	if err != nil {
		return nil, err
	}
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:   SecretName(orgID, credName),
			Labels: labels(orgID, credName, "registry"),
		},
		Type: corev1.SecretTypeDockerConfigJson,
		Data: map[string][]byte{corev1.DockerConfigJsonKey: dcj},
	}, nil
}

// RegistrySecretNamed is RegistrySecret with a caller-chosen Secret name. The
// deploy-time pull-secret projection names the Secret after the credential
// itself (cred.Name) so a workload's imagePullSecrets can reference it by the
// same logical name the developer set in image.credentialRef — unlike the
// SecretName(orgID, credName) scheme used for build/system-namespace secrets,
// which is internal and never referenced by user-authored templates. The
// org/kind labels are still attached so org isolation and traceability are
// unchanged. name must already be a valid RFC1123 object name (the caller
// validates and surfaces an actionable error; this builder does not sanitize,
// because the whole point is a predictable, user-referencable name).
func RegistrySecretNamed(name, orgID, credName, registry, username, password string) (*corev1.Secret, error) {
	dcj, err := DockerConfigJSON(registry, username, password)
	if err != nil {
		return nil, err
	}
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: labels(orgID, credName, "registry"),
		},
		Type: corev1.SecretTypeDockerConfigJson,
		Data: map[string][]byte{corev1.DockerConfigJsonKey: dcj},
	}, nil
}

// GitSecret builds the projected Opaque Secret for a git credential. The
// build Job's git-clone step reads "token" (and "username", defaulting to
// x-access-token) to rewrite the clone URL for a private HTTPS source.
func GitSecret(orgID, credName, username, token string) *corev1.Secret {
	if username == "" {
		username = "x-access-token"
	}
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:   SecretName(orgID, credName),
			Labels: labels(orgID, credName, "git"),
		},
		Type: corev1.SecretTypeOpaque,
		StringData: map[string]string{
			"username": username,
			"token":    token,
		},
	}
}
