package applications

import (
	"github.com/coffeenights/conure/cmd/api-server/applications/providers"
	apiConfig "github.com/coffeenights/conure/cmd/api-server/config"
	"github.com/coffeenights/conure/internal/config"
)

type ProviderType string

const (
	Vela ProviderType = "vela"
)

type ProviderStatus interface {
	GetApplicationStatus() (string, error)
	GetNetworkProperties(componentID string) (*providers.NetworkProperties, error)
	GetResourcesProperties(componentID string) (*providers.ResourcesProperties, error)
	GetStorageProperties(componentID string) (*providers.StorageProperties, error)
	GetSourceProperties(componentID string) (*providers.SourceProperties, error)
}

func NewProviderStatus(application *Application, environment *Environment) (ProviderStatus, error) {
	appConfig := config.LoadConfig(apiConfig.Config{})
	providerType := ProviderType(appConfig.ProviderSource)

	switch providerType {
	case Vela:
		return &providers.ProviderStatusVela{
			OrganizationID: application.OrganizationID.Hex(),
			ApplicationID:  application.ID.Hex(),
			Namespace:      environment.GetNamespace(),
		}, nil
	}
	return nil, providers.ErrProviderNotSupported
}
