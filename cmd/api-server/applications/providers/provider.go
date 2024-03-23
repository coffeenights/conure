package providers

import (
	"github.com/coffeenights/conure/cmd/api-server/applications"
	apiConfig "github.com/coffeenights/conure/cmd/api-server/config"
	"github.com/coffeenights/conure/internal/config"
)

type ProviderType string

const (
	Vela ProviderType = "vela"
)

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

type Provider struct {
	Application *applications.Application
	Environment *applications.Environment
}

type ProviderStatus interface {
	GetApplicationStatus() (string, error)
	GetNetworkProperties() (*NetworkProperties, error)
	GetResourcesProperties() (*ResourcesProperties, error)
	GetStorageProperties() (*StorageProperties, error)
	GetSourceProperties() (*SourceProperties, error)
}

func NewProviderStatus(environment *applications.Environment, application *applications.Application) (ProviderStatus, error) {
	appConfig := config.LoadConfig(apiConfig.Config{})
	provider := Provider{
		Application: application,
		Environment: environment,
	}
	providerType := ProviderType(appConfig.ProviderSource)

	switch providerType {
	case Vela:
		return &ProviderStatusVela{
			Provider: provider,
		}, nil
	}
	return nil, ErrProviderNotSupported
}
