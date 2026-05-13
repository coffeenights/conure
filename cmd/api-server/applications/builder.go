package applications

import (
	"encoding/json"
	"fmt"
	"log"

	conurev1alpha1 "github.com/coffeenights/conure/apis/core/v1alpha1"
	"github.com/coffeenights/conure/cmd/api-server/database"
	"github.com/coffeenights/conure/cmd/api-server/models"
	"github.com/coffeenights/conure/cmd/api-server/providers"
	"github.com/coffeenights/conure/cmd/api-server/variables"
	k8sUtils "github.com/coffeenights/conure/internal/k8s"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// BuildApplicationCRD assembles the env-scoped Application CRD object. It
// does not list components — Application CRD creation is handled per-deploy
// by the provider (idempotent), so this just shapes the metadata.
func BuildApplicationCRD(application *models.Application, environment *models.Environment) *conurev1alpha1.Application {
	return &conurev1alpha1.Application{
		TypeMeta: metav1.TypeMeta{
			APIVersion: conurev1alpha1.GroupVersion.String(),
			Kind:       "Application",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      application.Name,
			Namespace: environment.GetNamespace(),
			Labels: map[string]string{
				k8sUtils.ApplicationIDLabel:  application.ID.Hex(),
				k8sUtils.OrganizationIDLabel: application.OrganizationID.Hex(),
				k8sUtils.EnvironmentLabel:    environment.Name,
				k8sUtils.CreatedByLabel:      "conure",
			},
			Annotations: map[string]string{
				"conure.io/description": application.Description,
			},
		},
	}
}

// BuildComponentCRD turns a single (component, revision-values) pair into a
// Component CRD object scoped to the env namespace. This replaces the old
// "walk every component in Mongo" path: each deploy now operates on exactly
// one revision.
func BuildComponentCRD(application *models.Application, environment *models.Environment, component *models.Component, values map[string]interface{}) (*conurev1alpha1.Component, error) {
	valuesJSON, err := json.Marshal(values)
	if err != nil {
		return nil, fmt.Errorf("marshaling component values: %w", err)
	}

	return &conurev1alpha1.Component{
		TypeMeta: metav1.TypeMeta{
			APIVersion: conurev1alpha1.GroupVersion.String(),
			Kind:       conurev1alpha1.ComponentKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      component.Name,
			Namespace: environment.GetNamespace(),
			Labels: map[string]string{
				k8sUtils.ApplicationIDLabel:  application.ID.Hex(),
				k8sUtils.OrganizationIDLabel: application.OrganizationID.Hex(),
				k8sUtils.EnvironmentLabel:    environment.Name,
				k8sUtils.ComponentIDLabel:    component.ID.Hex(),
				k8sUtils.ComponentNameLabel:  component.Name,
				k8sUtils.CreatedByLabel:      "conure",
			},
		},
		Spec: conurev1alpha1.ComponentSpec{
			ComponentType: component.Type,
			Engine:        conurev1alpha1.ComponentEngine(component.Engine),
			Values:        &runtime.RawExtension{Raw: valuesJSON},
		},
	}, nil
}

// gatherVariables collects variables from all scopes (org → environment → component),
// with lower levels taking priority over higher ones on name conflicts.
// Encrypted variables are decrypted and returned in Secrets; plain variables in Variables.
//
// The merge step delegates to variables.MergeAllScopes so this code path and
// the /allscopes HTTP endpoints cannot disagree on precedence.
func gatherVariables(db *database.MongoDB, application *models.Application, environment *models.Environment, component *models.Component, keyStorage variables.SecretKeyStorage) (providers.ComponentVariables, error) {
	v := new(models.Variable)

	orgVars, err := v.ListByOrg(db, application.OrganizationID)
	if err != nil {
		return providers.ComponentVariables{}, fmt.Errorf("listing org variables for component %q: %w", component.Name, err)
	}
	envVars, err := v.ListByEnv(db, application.OrganizationID, application.ID, environment.Name)
	if err != nil {
		return providers.ComponentVariables{}, fmt.Errorf("listing env variables for component %q: %w", component.Name, err)
	}
	compVars, err := v.ListByComp(db, application.OrganizationID, application.ID, environment.Name, component.ID)
	if err != nil {
		return providers.ComponentVariables{}, fmt.Errorf("listing component variables for component %q: %w", component.Name, err)
	}
	log.Printf("gatherVariables: org=%d env=%d component=%d for component %q",
		len(orgVars), len(envVars), len(compVars), component.Name)

	cv := providers.ComponentVariables{
		ComponentName: component.Name,
		Variables:     make(map[string]string),
		Secrets:       make(map[string]string),
	}
	for _, m := range variables.MergeAllScopes(orgVars, envVars, compVars) {
		if m.IsEncrypted {
			decrypted, err := variables.DecryptValue(keyStorage, m.Value)
			if err != nil {
				return providers.ComponentVariables{}, fmt.Errorf("decrypting variable %q for component %q: %w", m.Name, component.Name, err)
			}
			cv.Secrets[m.Name] = decrypted
		} else {
			cv.Variables[m.Name] = m.Value
		}
	}
	return cv, nil
}
