package applications

import (
	"encoding/json"
	"fmt"
	"strings"

	conurev1alpha1 "github.com/coffeenights/conure/apis/core/v1alpha1"
	"github.com/coffeenights/conure/cmd/api-server/database"
	"github.com/coffeenights/conure/cmd/api-server/models"
	k8sUtils "github.com/coffeenights/conure/internal/k8s"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func BuildApplicationManifest(application *models.Application, environment *models.Environment, db *database.MongoDB) (*conurev1alpha1.Application, []conurev1alpha1.Component, error) {
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
		return nil, nil, err
	}

	var components []conurev1alpha1.Component
	for _, comp := range dbComponents {
		component, err := buildComponentManifest(application, environment, &comp, db)
		if err != nil {
			return nil, nil, err
		}
		components = append(components, *component)
	}

	return &app, components, nil
}

func buildComponentManifest(application *models.Application, environment *models.Environment, component *models.Component, db *database.MongoDB) (*conurev1alpha1.Component, error) {
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
		return nil, fmt.Errorf("marshaling component values: %w", err)
	}

	dbVars, err := new(models.Variable).ListByComp(db, application.OrganizationID, application.ID, environment.ID, component.ID)
	if err != nil {
		return nil, fmt.Errorf("listing component variables: %w", err)
	}
	variables := make([]conurev1alpha1.Variable, len(dbVars))
	for i, v := range dbVars {
		variables[i] = conurev1alpha1.Variable{
			Name:  v.Name,
			Value: v.Value,
		}
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
			Values:        &runtime.RawExtension{Raw: valuesJSON},
			Variables:     variables,
		},
	}, nil
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
