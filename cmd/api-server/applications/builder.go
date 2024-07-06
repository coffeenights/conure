package applications

import (
	"fmt"
	"github.com/coffeenights/conure/cmd/api-server/database"
	"github.com/coffeenights/conure/cmd/api-server/models"
	"github.com/coffeenights/conure/cmd/api-server/providers"
	"k8s.io/apimachinery/pkg/api/resource"
	"strings"
)

var serviceType = map[string]string{
	"public":  "LoadBalancer",
	"private": "ClusterIP",
}

func buildExposeTrait(component *models.Component) map[string]interface{} {
	if component.Settings.NetworkSettings.Exposed == false {
		return map[string]interface{}{}
	}
	trait := map[string]interface{}{
		"type":       "expose",
		"properties": map[string]interface{}{},
	}
	// Set the service type
	exposeType := string(component.Settings.NetworkSettings.Type)
	trait["properties"].(map[string]interface{})["type"] = serviceType[exposeType]

	type Port map[string]interface{}
	var ports []Port
	// Set the ports
	for _, settingsPort := range component.Settings.NetworkSettings.Ports {
		traitPort := Port{
			"port":     settingsPort.HostPort,
			"protocol": strings.ToUpper(string(settingsPort.Protocol)),
		}
		ports = append(ports, traitPort)
	}
	trait["properties"].(map[string]interface{})["ports"] = ports
	return trait
}

func buildScalerTrait(component *models.Component) map[string]interface{} {
	trait := map[string]interface{}{
		"type":       "scaler",
		"properties": map[string]interface{}{},
	}
	trait["properties"].(map[string]interface{})["replicas"] = component.Settings.ResourcesSettings.Replicas
	return trait
}

func buildComponentProperties(component *models.Component) map[string]interface{} {
	properties := map[string]interface{}{
		"image":           component.Settings.SourceSettings.Repository,
		"workdir":         "/app",
		"imagePullPolicy": "Always",
		"cpu":             fmt.Sprint(component.Settings.ResourcesSettings.CPU),
		"memory":          fmt.Sprintf("%dMi", component.Settings.ResourcesSettings.Memory),
		"cmd":             strings.Fields(component.Settings.SourceSettings.Command),
	}
	return properties
}

func buildStorageTrait(component *models.Component) map[string]interface{} {
	trait := map[string]interface{}{
		"type": "storage",
		"properties": map[string]interface{}{
			"pvc": []map[string]interface{}{},
		},
	}
	type MountPath map[string]interface{}
	var paths []MountPath

	for _, storage := range component.Settings.StorageSettings {
		diskSize := resource.NewQuantity(int64(storage.Size*1000*1000*1000), resource.DecimalSI)
		path := MountPath{
			"mountPath": storage.MountPath,
			"name":      storage.Name,
			"resources": map[string]interface{}{
				"requests": map[string]interface{}{
					"storage": fmt.Sprintf("%s", diskSize),
				},
			},
		}
		paths = append(paths, path)
	}
	trait["properties"].(map[string]interface{})["pvc"] = paths
	return trait
}

func BuildApplicationManifest(application *models.Application, environment *models.Environment, db *database.MongoDB) (map[string]interface{}, error) {
	object := map[string]interface{}{
		"apiVersion": "core.oam.dev/v1beta1",
		"kind":       "Application",
		"metadata": map[string]interface{}{
			"name": application.Name,
			"labels": map[string]interface{}{
				providers.ApplicationIDLabel:  application.ID.Hex(),
				providers.OrganizationIDLabel: application.OrganizationID.Hex(),
				providers.EnvironmentLabel:    environment.Name,
				providers.CreatedByLabel:      "conure",
				providers.NamespaceLabel:      environment.GetNamespace(),
			},
			"annotations": map[string]interface{}{
				"conure.io/description": application.Description,
			},
			"namespace": environment.GetNamespace(),
		},
		"spec": map[string]interface{}{},
	}
	// Add components
	var componentsManifest []map[string]interface{}
	components, err := application.ListComponents(db)
	if err != nil {
		return nil, err
	}
	for _, component := range components {
		componentManifest := map[string]interface{}{
			"name": component.Name,
			"type": component.Type,
		}
		// Add traits
		var traits []map[string]interface{}
		exposeTrait := buildExposeTrait(&component)
		if len(exposeTrait) > 0 {
			traits = append(traits, exposeTrait)
		}
		scalerTrait := buildScalerTrait(&component)
		traits = append(traits, scalerTrait)

		storageTrait := buildStorageTrait(&component)
		traits = append(traits, storageTrait)

		componentManifest["traits"] = traits

		// Add properties
		componentManifest["properties"] = buildComponentProperties(&component)

		componentsManifest = append(componentsManifest, componentManifest)
	}
	object["spec"].(map[string]interface{})["components"] = componentsManifest
	return object, nil
}
