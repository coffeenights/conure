package providers

import (
	"context"
	"fmt"
	"log"

	conurev1alpha1 "github.com/coffeenights/conure/apis/core/v1alpha1"
	"github.com/coffeenights/conure/cmd/api-server/models"
	k8sUtils "github.com/coffeenights/conure/internal/k8s"
	corev1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// componentDefinitionName is the deterministic cluster object name for an
// org's materialized definition: <orgID>-<type>[-<engine>]. The name keeps
// objects from different orgs from colliding in the cluster's flat
// (cluster-scoped) namespace, but it is NOT what isolates tenants — the
// controller resolves by spec.type, so two orgs' "webservice" rows would both
// match a query regardless of object name. The OrganizationIDLabel below is
// the actual isolation key; the controller filters its lookup on it.
func componentDefinitionName(orgID, compType, engine string) string {
	if engine == "" {
		engine = string(conurev1alpha1.EngineTimoni)
	}
	return fmt.Sprintf("%s-%s-%s", orgID, compType, engine)
}

// toCRD projects a Mongo ComponentDefinition row into the cluster-scoped CRD
// object, stamping the org-id label the controller filters on. The Mongo row
// is the source of truth; this is a per-cluster projection re-applied on every
// deploy, which is also what makes multi-cluster work (each target cluster
// converges to whatever Mongo currently says).
func toCRD(orgID string, def *models.ComponentDefinition) *conurev1alpha1.ComponentDefinition {
	var registryRef *corev1.LocalObjectReference
	if def.RegistrySecretName != "" {
		registryRef = &corev1.LocalObjectReference{Name: def.RegistrySecretName}
	}
	var helm *conurev1alpha1.HelmEngineSpec
	if def.Helm != nil {
		helm = &conurev1alpha1.HelmEngineSpec{
			ReleaseName: def.Helm.ReleaseName,
			Namespace:   def.Helm.Namespace,
			KubeVersion: def.Helm.KubeVersion,
			APIVersions: def.Helm.APIVersions,
		}
	}
	return &conurev1alpha1.ComponentDefinition{
		TypeMeta: metav1.TypeMeta{
			APIVersion: conurev1alpha1.GroupVersion.String(),
			Kind:       "ComponentDefinition",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: componentDefinitionName(orgID, def.Type, def.EngineKey()),
			Labels: map[string]string{
				k8sUtils.OrganizationIDLabel: orgID,
				k8sUtils.CreatedByLabel:      "conure",
			},
		},
		Spec: conurev1alpha1.ComponentDefinitionSpec{
			ComponentType:     def.Type,
			Description:       def.Description,
			Engine:            conurev1alpha1.ComponentEngine(def.Engine),
			OCIRepository:     def.OCIRepository,
			OCITag:            def.OCITag,
			OCIDigest:         def.OCIDigest,
			OCIRegistry:       def.OCIRegistry,
			RegistrySecretRef: registryRef,
			Helm:              helm,
			Buildable:         def.Buildable,
			FieldRoles:        def.FieldRoles,
		},
	}
}

// EnsureComponentDefinition materializes the resolved Mongo definition into
// the target cluster as a cluster-scoped ComponentDefinition CRD, create-or-
// update, before the Component that needs it is applied. This is what
// guarantees "the CRD is in place whenever a deploy happens": the controller
// never races a missing or stale definition because the API has already
// reconciled it for this org in this cluster.
//
// p.OrganizationID supplies the org-id stamped on both the object name and
// the OrganizationIDLabel, matching the label the API already puts on the
// Component — so the controller's label-scoped lookup pairs them up without
// ever knowing what an organization is.
func (p *ProviderDispatcherConure) EnsureComponentDefinition(ctx context.Context, def *models.ComponentDefinition) error {
	clientset, err := k8sUtils.GetClientset()
	if err != nil {
		return err
	}
	desired := toCRD(p.OrganizationID, def)
	defs := clientset.Conure.CoreV1alpha1().ComponentDefinitions()

	existing, err := defs.Get(ctx, desired.Name, metav1.GetOptions{})
	if k8sErrors.IsNotFound(err) {
		if _, err = defs.Create(ctx, desired, metav1.CreateOptions{}); err != nil {
			return fmt.Errorf("creating component definition %q: %w", desired.Name, err)
		}
		log.Printf("Created component definition %q for org %s\n", desired.Name, p.OrganizationID)
		return nil
	}
	if err != nil {
		return fmt.Errorf("getting component definition %q: %w", desired.Name, err)
	}
	desired.ResourceVersion = existing.ResourceVersion
	if _, err = defs.Update(ctx, desired, metav1.UpdateOptions{}); err != nil {
		return fmt.Errorf("updating component definition %q: %w", desired.Name, err)
	}
	log.Printf("Updated component definition %q for org %s\n", desired.Name, p.OrganizationID)
	return nil
}
