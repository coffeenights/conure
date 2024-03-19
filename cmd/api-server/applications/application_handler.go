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
	NetworkProperties   *NetworkProperties   `json:"network"`
	ResourcesProperties *ResourcesProperties `json:"resources"`
	StorageProperties   *StorageProperties   `json:"storage"`
	SourceProperties    *SourceProperties    `json:"source"`
}

type ComponentHandler struct {
	Properties ComponentProperties
	Status     *providers.ProviderStatus
	Model      *Component
}

func NewComponentHandler(name, componentType, description string, properties ComponentProperties) *ComponentHandler {
	status, _ := providers.NewProviderStatus()
	return &ComponentHandler{
		Properties: properties,
		Status:     &status,
	}
}

type ApplicationHandler struct {
	ID             string
	OrganizationID string
	Model          *Application
	DB             *database.MongoDB
}

func NewApplicationHandler(db *database.MongoDB) (*ApplicationHandler, error) {
	return &ApplicationHandler{
		Model: &Application{},
		DB:    db,
	}, nil
}

func ListOrganizationApplications(organizationID string, db *database.MongoDB) ([]*ApplicationHandler, error) {
	models, err := ApplicationList(db, organizationID)
	if err != nil {
		return nil, err
	}
	handlers := make([]*ApplicationHandler, len(models))
	for i, model := range models {
		handler, err := NewApplicationHandler(db)
		if err != nil {
			return nil, err
		}
		handler.Model = model
		handlers[i] = handler
	}
	return handlers, nil
}

func (ah *ApplicationHandler) GetApplicationByID(appID string) error {
	_, err := ah.Model.GetByID(ah.DB, appID)
	if err != nil {
		return err
	}
	return nil
}

func (ah *ApplicationHandler) ListComponents() ([]*ComponentHandler, error) {
	models, err := ah.Model.ListComponents(ah.DB)
	if err != nil {
		return nil, err
	}
	handlers := make([]*ComponentHandler, len(models))
	for i, model := range models {
		handlers[i] = &ComponentHandler{
			Model: &model,
		}
	}
	return handlers, nil
}
