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

func BuildApplicationManifest(application *models.Application, environment *models.Environment, db *database.MongoDB, keyStorage variables.SecretKeyStorage) (*conurev1alpha1.Application, []conurev1alpha1.Component, []providers.ComponentVariables, error) {
	app := conurev1alpha1.Application{
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

	dbComponents, err := application.ListComponents(db)
	if err != nil {
		return nil, nil, nil, err
	}

	var components []conurev1alpha1.Component
	var compVars []providers.ComponentVariables
	for _, comp := range dbComponents {
		component, cv, err := buildComponentManifest(application, environment, &comp, db, keyStorage)
		if err != nil {
			return nil, nil, nil, err
		}
		components = append(components, *component)
		compVars = append(compVars, cv)
	}

	return &app, components, compVars, nil
}

func buildComponentManifest(application *models.Application, environment *models.Environment, component *models.Component, db *database.MongoDB, keyStorage variables.SecretKeyStorage) (*conurev1alpha1.Component, providers.ComponentVariables, error) {
	valuesJSON, err := json.Marshal(component.Values)
	if err != nil {
		return nil, providers.ComponentVariables{}, fmt.Errorf("marshaling component values: %w", err)
	}

	cv, err := gatherVariables(db, application, environment, component, keyStorage)
	if err != nil {
		return nil, providers.ComponentVariables{}, err
	}
	cv.ComponentName = component.Name

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
			Values:        &runtime.RawExtension{Raw: valuesJSON},
		},
	}, cv, nil
}

// gatherVariables collects variables from all scopes (org → environment → component),
// with lower levels taking priority over higher ones on name conflicts.
// Encrypted variables are decrypted and returned in Secrets; plain variables in Variables.
func gatherVariables(db *database.MongoDB, application *models.Application, environment *models.Environment, component *models.Component, keyStorage variables.SecretKeyStorage) (providers.ComponentVariables, error) {
	type entry struct {
		value       string
		isEncrypted bool
	}
	merged := map[string]entry{}

	v := new(models.Variable)
	type scopedFetcher struct {
		scope string
		fetch func() ([]models.Variable, error)
	}
	fetchers := []scopedFetcher{
		{
			scope: "org",
			fetch: func() ([]models.Variable, error) { return v.ListByOrg(db, application.OrganizationID) },
		},
		{
			scope: "env",
			fetch: func() ([]models.Variable, error) {
				return v.ListByEnv(db, application.OrganizationID, application.ID, environment.Name)
			},
		},
		{
			scope: "component",
			fetch: func() ([]models.Variable, error) {
				return v.ListByComp(db, application.OrganizationID, application.ID, environment.Name, component.ID)
			},
		},
	}

	for _, sf := range fetchers {
		vars, err := sf.fetch()
		if err != nil {
			return providers.ComponentVariables{}, fmt.Errorf("listing %s variables for component %q: %w", sf.scope, component.Name, err)
		}
		log.Printf("gatherVariables: %s scope returned %d variable(s) for component %q", sf.scope, len(vars), component.Name)
		for _, v := range vars {
			merged[v.Name] = entry{value: v.Value, isEncrypted: v.IsEncrypted}
		}
	}

	if len(merged) == 0 {
		log.Printf("gatherVariables: no variables found for component %q (org=%s app=%s env=%s component=%s)",
			component.Name, application.OrganizationID.Hex(), application.ID.Hex(), environment.ID, component.ID.Hex())
	}

	cv := providers.ComponentVariables{
		Variables: make(map[string]string),
		Secrets:   make(map[string]string),
	}
	for name, e := range merged {
		if e.isEncrypted {
			decrypted, err := variables.DecryptValue(keyStorage, e.value)
			if err != nil {
				return providers.ComponentVariables{}, fmt.Errorf("decrypting variable %q for component %q: %w", name, component.Name, err)
			}
			cv.Secrets[name] = decrypted
		} else {
			cv.Variables[name] = e.value
		}
	}
	return cv, nil
}
