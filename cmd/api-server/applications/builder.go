package applications

import (
	"github.com/coffeenights/conure/cmd/api-server/database"
	"github.com/coffeenights/conure/cmd/api-server/models"
	"github.com/coffeenights/conure/cmd/api-server/providers"
)

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
	componentsManifest := []map[string]interface{}{}
	components, err := application.ListComponents(db)
	if err != nil {
		return nil, err
	}
	for _, component := range components {
		componentManifest := map[string]interface{}{
			"name": component.Name,
			"type": component.Type,
			"labels": map[string]string{
				providers.ComponentNameLabel: component.Name,
				providers.ComponentIDLabel:   component.ID.Hex(),
			},
		}
		componentsManifest = append(componentsManifest, componentManifest)
	}
	object["spec"].(map[string]interface{})["components"] = componentsManifest
	return object, nil
}
