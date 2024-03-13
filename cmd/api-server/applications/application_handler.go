package applications

import (
	"github.com/coffeenights/conure/cmd/api-server/applications/providers"
	"github.com/coffeenights/conure/cmd/api-server/database"
)

type Properties interface {
}

type Trait struct {
	Name       string                 `json:"name"`
	Type       string                 `json:"type"`
	Properties map[string]interface{} `json:"properties"`
}

type NetworkProperties struct {
	IP         string  `json:"ip"`
	ExternalIP string  `json:"external_ip"`
	Host       string  `json:"host"`
	Ports      []int32 `json:"port"`
}

type ResourcesProperties struct {
	Replicas int32  `json:"replicas"`
	CPU      string `json:"cpu"`
	Memory   string `json:"memory"`
}

type StorageProperties struct {
	Size string `json:"size"`
}

type SourceProperties struct {
	ContainerImage string `json:"container_image"`
}

type ComponentProperties struct {
	Name                string               `json:"name"`
	Type                string               `json:"type"`
	Description         string               `json:"description"`
	NetworkProperties   *NetworkProperties   `json:"network"`
	ResourcesProperties *ResourcesProperties `json:"resources"`
	StorageProperties   *StorageProperties   `json:"storage"`
	SourceProperties    *SourceProperties    `json:"source"`
}

type ComponentHandler struct {
	Name        string              `json:"name"`
	Type        string              `json:"type"`
	Description string              `json:"description"`
	Traits      []Trait             `json:"traits"`
	Properties  ComponentProperties `json:"properties"`
}

type ApplicationHandler struct {
	ID             string
	OrganizationID string
	Model          *Application
	Status         *providers.ProviderStatus
}

func NewApplicationHandler(organizationID string, applicationID string, db *database.MongoDB) (*ApplicationHandler, error) {
	model, err := NewApplication(organizationID, "", "").GetByID(db, applicationID)
	if err != nil {
		return nil, err
	}
	status, err := providers.NewProviderStatus()
	if err != nil {
		return nil, err
	}
	return &ApplicationHandler{
		Model:  model,
		Status: &status,
	}, nil
}

func ListOrganizationApplications(organizationID string, db *database.MongoDB) ([]*ApplicationHandler, error) {
	models, err := ApplicationList(db, organizationID)
	if err != nil {
		return nil, err
	}
	handlers := make([]*ApplicationHandler, len(models))
	for i, model := range models {
		status, err := providers.NewProviderStatus()
		if err != nil {
			return nil, err
		}
		handlers[i] = &ApplicationHandler{
			Model:  model,
			Status: &status,
		}

	}
	return handlers, nil
}
