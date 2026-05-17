package providers

import (
	"context"
	"fmt"
	"strings"

	"go.mongodb.org/mongo-driver/bson/primitive"
	corev1 "k8s.io/api/core/v1"
	apimachineryvalidation "k8s.io/apimachinery/pkg/util/validation"

	"github.com/coffeenights/conure/cmd/api-server/database"
	"github.com/coffeenights/conure/cmd/api-server/models"
	"github.com/coffeenights/conure/cmd/api-server/variables"
	"github.com/coffeenights/conure/internal/credentials"
	k8sUtils "github.com/coffeenights/conure/internal/k8s"
)

// CredentialResolver is the slice of credential machinery the deploy-time
// projection needs: read the org's encrypted credential from Mongo and the
// AES key to decrypt it. Both are already held by the api-server handler
// (a.MongoDB / a.KeyStorage); threading them in keeps the provider free of a
// Mongo/keyStore dependency in its struct.
type CredentialResolver struct {
	DB         *database.MongoDB
	KeyStorage variables.SecretKeyStorage
}

// projectRegistryCredential resolves a logical registry credential name for an
// org, decrypts it, projects it into the system namespace as a
// dockerconfigjson Secret, and returns the CONCRETE Secret name to stamp on
// the materialized ComponentDefinition.
//
// Empty credentialRef → ("", nil): anonymous pull, the public-registry path,
// unchanged. A non-empty ref that the org has no credential for is a hard
// error (no fallback): the deploy fails fast with an actionable message
// instead of a later, silent pull 403 in the controller.
func (cr *CredentialResolver) projectRegistryCredential(ctx context.Context, clientset *k8sUtils.GenericClientset, orgID primitive.ObjectID, credentialRef string) (string, error) {
	if credentialRef == "" {
		return "", nil
	}

	cred := &models.Credential{}
	if err := cred.GetByOrgAndName(ctx, cr.DB, orgID, credentialRef); err != nil {
		return "", fmt.Errorf("component definition references registry credential %q which this organization has not loaded; create it with `conure credential set %s --kind registry ...`: %w", credentialRef, credentialRef, err)
	}
	if cred.Kind != models.CredentialKindRegistry {
		return "", fmt.Errorf("credential %q is kind %q, but a registry credential is required to pull the component's OCI module", credentialRef, cred.Kind)
	}

	password, err := variables.DecryptValue(cr.KeyStorage, cred.Secret)
	if err != nil {
		return "", fmt.Errorf("decrypting registry credential %q: %w", credentialRef, err)
	}

	secret, err := credentials.RegistrySecret(orgID.Hex(), cred.Name, cred.RegistryURL, cred.Username, password)
	if err != nil {
		return "", fmt.Errorf("building projected Secret for credential %q: %w", credentialRef, err)
	}
	if err := k8sUtils.CreateOrUpdateSecret(clientset, credentials.SystemNamespace, secret); err != nil {
		return "", fmt.Errorf("projecting credential %q into %s: %w", credentialRef, credentials.SystemNamespace, err)
	}
	return secret.Name, nil
}

// ProjectBuildCredential resolves a logical credential name (taken from a
// component's values via the image.credentialRef / git.credentialRef field
// roles), decrypts it, projects it into the system namespace as the right
// Secret shape for kind, and returns the CONCRETE Secret name to mount in the
// build Job. Empty name → ("", nil): public, no auth — the build Job mounts
// nothing and clones/pushes anonymously, exactly as before credentials
// existed. A referenced-but-missing or wrong-kind credential is a hard error
// (no fallback): the build fails fast with an actionable message.
//
// Exported because the build path (cmd/api-server/applications) drives it,
// unlike projectRegistryCredential which is internal to the definition
// materialization on the same deploy path.
func (cr *CredentialResolver) ProjectBuildCredential(ctx context.Context, clientset *k8sUtils.GenericClientset, orgID primitive.ObjectID, credentialRef string, want models.CredentialKind) (string, error) {
	if credentialRef == "" {
		return "", nil
	}

	cred := &models.Credential{}
	if err := cred.GetByOrgAndName(ctx, cr.DB, orgID, credentialRef); err != nil {
		return "", fmt.Errorf("component references %s credential %q which this organization has not loaded; create it with `conure credential set %s --kind %s ...`: %w", want, credentialRef, credentialRef, want, err)
	}
	if cred.Kind != want {
		return "", fmt.Errorf("credential %q is kind %q, but a %q credential is required here", credentialRef, cred.Kind, want)
	}

	password, err := variables.DecryptValue(cr.KeyStorage, cred.Secret)
	if err != nil {
		return "", fmt.Errorf("decrypting credential %q: %w", credentialRef, err)
	}

	var secret *corev1.Secret
	switch want {
	case models.CredentialKindRegistry:
		s, berr := credentials.RegistrySecret(orgID.Hex(), cred.Name, cred.RegistryURL, cred.Username, password)
		if berr != nil {
			return "", fmt.Errorf("building projected registry Secret for %q: %w", credentialRef, berr)
		}
		secret = s
	case models.CredentialKindGit:
		secret = credentials.GitSecret(orgID.Hex(), cred.Name, cred.Username, password)
	default:
		return "", fmt.Errorf("unsupported credential kind %q", want)
	}

	if err := k8sUtils.CreateOrUpdateSecret(clientset, credentials.SystemNamespace, secret); err != nil {
		return "", fmt.Errorf("projecting credential %q into %s: %w", credentialRef, credentials.SystemNamespace, err)
	}
	return secret.Name, nil
}

// EnsurePullSecret resolves a logical registry credential (the value of a
// component's image.credentialRef field role) and ensures a dockerconfigjson
// Secret exists in the workload's namespace BEFORE the Component is applied,
// so the kubelet can authenticate the image pull instead of ImagePullBackOff.
//
// Unlike ProjectBuildCredential (build push secret, system namespace, internal
// cred-<org>-<name> name), the Secret is named after the credential itself
// (cred.Name) so a user-authored module template can reference it by the same
// logical name the developer put in image.credentialRef — no extra field-role
// indirection. The credential name is the source of truth (read from Mongo),
// validated as a legal Secret name; an invalid name fails the deploy with an
// actionable message rather than emitting a Secret the apiserver rejects.
//
// Empty credentialRef → nil: public image, no pull secret, unchanged. A
// referenced-but-missing or wrong-kind credential is a hard error (no
// fallback), consistent with the build path: the deploy fails fast with an
// actionable message instead of a later silent pull failure in the cluster.
func (cr *CredentialResolver) EnsurePullSecret(ctx context.Context, clientset *k8sUtils.GenericClientset, orgID primitive.ObjectID, credentialRef, namespace string) error {
	if credentialRef == "" {
		return nil
	}

	cred := &models.Credential{}
	if err := cred.GetByOrgAndName(ctx, cr.DB, orgID, credentialRef); err != nil {
		return fmt.Errorf("component references registry credential %q which this organization has not loaded; create it with `conure credential set %s --kind registry ...`: %w", credentialRef, credentialRef, err)
	}
	if cred.Kind != models.CredentialKindRegistry {
		return fmt.Errorf("credential %q is kind %q, but a registry credential is required to pull the workload image", credentialRef, cred.Kind)
	}

	// The credential name doubles as the Secret name (and thus the workload's
	// imagePullSecrets reference), so it must be a valid RFC1123 object name.
	if errs := apimachineryvalidation.IsDNS1123Subdomain(cred.Name); len(errs) > 0 {
		return fmt.Errorf("registry credential name %q cannot be used as a Kubernetes Secret name (%s); rename the credential to a lowercase RFC1123 name", cred.Name, strings.Join(errs, "; "))
	}

	password, err := variables.DecryptValue(cr.KeyStorage, cred.Secret)
	if err != nil {
		return fmt.Errorf("decrypting registry credential %q: %w", credentialRef, err)
	}

	secret, err := credentials.RegistrySecretNamed(cred.Name, orgID.Hex(), cred.Name, cred.RegistryURL, cred.Username, password)
	if err != nil {
		return fmt.Errorf("building pull Secret for credential %q: %w", credentialRef, err)
	}
	if err := k8sUtils.CreateOrUpdateSecret(clientset, namespace, secret); err != nil {
		return fmt.Errorf("projecting pull credential %q into %s: %w", credentialRef, namespace, err)
	}
	return nil
}
