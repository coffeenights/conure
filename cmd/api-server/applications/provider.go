package applications

import (
	apiConfig "github.com/coffeenights/conure/cmd/api-server/config"
	"github.com/coffeenights/conure/cmd/api-server/models"
	"github.com/coffeenights/conure/cmd/api-server/providers"
	"github.com/coffeenights/conure/internal/config"
)

type ProviderType string

const (
	Vela ProviderType = "vela"
)

type ProviderStatus interface {
	GetApplicationStatus() (string, error)
	GetNetworkProperties(componentName string) (*providers.NetworkProperties, error)
	GetResourcesProperties(componentName string) (*providers.ResourcesProperties, error)
	GetStorageProperties(componentName string) (*providers.StorageProperties, error)
	GetSourceProperties(componentName string) (*providers.SourceProperties, error)
}

func NewProviderStatus(application *models.Application, environment *models.Environment) (ProviderStatus, error) {
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

func NewProviderDispatcher(application *models.Application, environment *models.Environment) (ProviderDispatcher, error) {
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
