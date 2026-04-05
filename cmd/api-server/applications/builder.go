package applications

import (
	"encoding/json"
	"fmt"
	"strings"

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
	values := conurev1alpha1.Values{
		Resources: conurev1alpha1.Resources{
			Replicas: component.Settings.ResourcesSettings.Replicas,
			CPU:      component.Settings.ResourcesSettings.CPU,
			Memory:   component.Settings.ResourcesSettings.Memory,
		},
		Network: conurev1alpha1.Network{
			Exposed: component.Settings.NetworkSettings.Exposed,
			Type:    conurev1alpha1.AccessType(component.Settings.NetworkSettings.Type),
			Ports:   buildPorts(component.Settings.NetworkSettings.Ports),
		},
		Source: conurev1alpha1.Source{
			SourceType:    "oci",
			OCIRepository: component.Settings.SourceSettings.Repository,
			Command:       strings.Fields(component.Settings.SourceSettings.Command),
		},
		Storage: buildStorage(component.Settings.StorageSettings),
	}

	valuesJSON, err := json.Marshal(values)
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

	levels := []struct {
		fetch func() ([]models.Variable, error)
	}{
		{func() ([]models.Variable, error) {
			return new(models.Variable).ListByOrg(db, application.OrganizationID)
		}},
		{func() ([]models.Variable, error) {
			return new(models.Variable).ListByEnv(db, application.OrganizationID, application.ID, environment.ID)
		}},
		{func() ([]models.Variable, error) {
			return new(models.Variable).ListByComp(db, application.OrganizationID, application.ID, environment.ID, component.ID)
		}},
	}

	for _, level := range levels {
		vars, err := level.fetch()
		if err != nil {
			return providers.ComponentVariables{}, fmt.Errorf("listing variables: %w", err)
		}
		for _, v := range vars {
			merged[v.Name] = entry{value: v.Value, isEncrypted: v.IsEncrypted}
		}
	}

	cv := providers.ComponentVariables{
		Variables: make(map[string]string),
		Secrets:   make(map[string]string),
	}
	for name, e := range merged {
		if e.isEncrypted {
			decrypted, err := variables.DecryptValue(keyStorage, e.value)
			if err != nil {
				return providers.ComponentVariables{}, fmt.Errorf("decrypting variable %q: %w", name, err)
			}
			cv.Secrets[name] = decrypted
		} else {
			cv.Variables[name] = e.value
		}
	}
	return cv, nil
}

func buildPorts(ports []models.PortSettings) []conurev1alpha1.Port {
	result := make([]conurev1alpha1.Port, len(ports))
	for i, p := range ports {
		result[i] = conurev1alpha1.Port{
			HostPort:   p.HostPort,
			TargetPort: p.TargetPort,
			Protocol:   conurev1alpha1.Protocol(p.Protocol),
		}
	}
	return result
}

func buildStorage(storages []models.StorageSettings) []conurev1alpha1.Storage {
	result := make([]conurev1alpha1.Storage, len(storages))
	for i, s := range storages {
		result[i] = conurev1alpha1.Storage{
			Size:      fmt.Sprintf("%.1fGi", s.Size),
			Name:      s.Name,
			MountPath: s.MountPath,
		}
	}
	return result
}
