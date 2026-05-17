package component

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// RegistrySecretNamespace is the namespace where ComponentDefinition pull
// secrets are looked up. Set by main during controller setup from POD_NAMESPACE
// (downward API) or the --registry-secret-namespace flag.
var RegistrySecretNamespace string

// dockerConfigJSON mirrors the structure of a kubernetes.io/dockerconfigjson
// Secret payload. Only the fields needed for credential extraction are decoded.
type dockerConfigJSON struct {
	Auths map[string]dockerConfigAuth `json:"auths"`
}

type dockerConfigAuth struct {
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
	Auth     string `json:"auth,omitempty"`
}

// resolveRegistryCredentials fetches the named dockerconfigjson Secret from
// the controller's namespace and returns a "username:password" string for the
// registry hosting ociRepository. Returns ("", nil) when secretName is empty
// (public registry path). secretName is the concrete Secret name the API
// server stamped on the ComponentDefinition after resolving the org's logical
// credential; the controller never resolves credentials itself.
func resolveRegistryCredentials(ctx context.Context, c client.Client, secretName, ociRepository string) (string, error) {
	if secretName == "" {
		return "", nil
	}
	if RegistrySecretNamespace == "" {
		return "", fmt.Errorf("registry secret namespace not configured (set POD_NAMESPACE or --registry-secret-namespace)")
	}

	secret := &corev1.Secret{}
	key := types.NamespacedName{Namespace: RegistrySecretNamespace, Name: secretName}
	if err := c.Get(ctx, key, secret); err != nil {
		return "", fmt.Errorf("failed to get registry secret %s/%s: %w", key.Namespace, key.Name, err)
	}
	if secret.Type != corev1.SecretTypeDockerConfigJson {
		return "", fmt.Errorf("registry secret %s/%s must be of type %s, got %s", key.Namespace, key.Name, corev1.SecretTypeDockerConfigJson, secret.Type)
	}

	raw, ok := secret.Data[corev1.DockerConfigJsonKey]
	if !ok || len(raw) == 0 {
		return "", fmt.Errorf("registry secret %s/%s missing %q key", key.Namespace, key.Name, corev1.DockerConfigJsonKey)
	}

	var cfg dockerConfigJSON
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return "", fmt.Errorf("failed to parse %s in secret %s/%s: %w", corev1.DockerConfigJsonKey, key.Namespace, key.Name, err)
	}

	host := registryHost(ociRepository)
	entry, matchedKey := lookupAuth(cfg.Auths, host)
	if matchedKey == "" {
		return "", fmt.Errorf("registry secret %s/%s has no credentials for registry %q", key.Namespace, key.Name, host)
	}

	if entry.Username != "" {
		return entry.Username + ":" + entry.Password, nil
	}
	if entry.Auth != "" {
		decoded, err := base64.StdEncoding.DecodeString(entry.Auth)
		if err != nil {
			return "", fmt.Errorf("failed to decode auth field for %q in secret %s/%s: %w", matchedKey, key.Namespace, key.Name, err)
		}
		creds := string(decoded)
		if !strings.Contains(creds, ":") {
			return "", fmt.Errorf("auth field for %q in secret %s/%s is not in user:password form", matchedKey, key.Namespace, key.Name)
		}
		return creds, nil
	}
	return "", fmt.Errorf("registry secret %s/%s entry for %q has neither username/password nor auth", key.Namespace, key.Name, matchedKey)
}

// registryHost extracts the registry hostname from an OCI repository reference
// like "ghcr.io/coffeenights/foo" or "oci://registry.example.com/path".
func registryHost(repo string) string {
	repo = strings.TrimPrefix(repo, "oci://")
	if u, err := url.Parse("https://" + repo); err == nil && u.Host != "" {
		return u.Host
	}
	if i := strings.Index(repo, "/"); i > 0 {
		return repo[:i]
	}
	return repo
}

// lookupAuth finds the dockerconfigjson .auths entry matching host. Docker CLI
// stores keys as bare hostnames ("ghcr.io"), full URLs
// ("https://index.docker.io/v1/"), or host+path ("ghcr.io/myorg"). Match by
// hostname after normalizing each key.
func lookupAuth(auths map[string]dockerConfigAuth, host string) (dockerConfigAuth, string) {
	for k, v := range auths {
		if normalizeAuthKey(k) == host {
			return v, k
		}
	}
	return dockerConfigAuth{}, ""
}

func normalizeAuthKey(k string) string {
	k = strings.TrimPrefix(k, "https://")
	k = strings.TrimPrefix(k, "http://")
	if i := strings.Index(k, "/"); i > 0 {
		k = k[:i]
	}
	return k
}
