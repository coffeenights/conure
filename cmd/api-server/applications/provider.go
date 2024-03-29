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
		provider, err := providers.NewProviderStatusVela(application.OrganizationID.Hex(), application.ID.Hex(), environment.GetNamespace())
		if err != nil {
			return nil, err
		}
		return provider, nil
	}
	return nil, providers.ErrProviderNotSupported
}

type ProviderDispatcher interface {
	DeployApplication(manifest map[string]interface{}) error
}

func NewProviderDispatcher(application *Application, environment *Environment) (ProviderDispatcher, error) {
	appConfig := config.LoadConfig(apiConfig.Config{})
	providerType := ProviderType(appConfig.ProviderSource)

	switch providerType {
	case Vela:
		return &providers.ProviderDispatcherVela{
			OrganizationID: application.OrganizationID.Hex(),
			ApplicationID:  application.ID.Hex(),
			Namespace:      environment.GetNamespace(),
			Environment:    environment.Name,
		}, nil
	}
	return nil, providers.ErrProviderNotSupported
}
