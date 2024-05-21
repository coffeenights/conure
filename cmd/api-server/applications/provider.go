package applications

import (
	apiConfig "github.com/coffeenights/conure/cmd/api-server/config"
	"github.com/coffeenights/conure/cmd/api-server/conureerrors"
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
	GetComponentStatus(componentName string) (*providers.ComponentStatusHealth, error)
	GetPodList(componentName string) ([]string, error)
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
	return nil, conureerrors.ErrProviderNotSupported
}

type ProviderDispatcher interface {
	DeployApplication(manifest map[string]interface{}) error
	UpdateApplication(manifest map[string]interface{}) error
}

func NewProviderDispatcher(application *models.Application, environment *models.Environment) (ProviderDispatcher, error) {
	appConfig := config.LoadConfig(apiConfig.Config{})
	providerType := ProviderType(appConfig.ProviderSource)

	switch providerType {
	case Vela:
		return &providers.ProviderDispatcherVela{
			OrganizationID:  application.OrganizationID.Hex(),
			ApplicationID:   application.ID.Hex(),
			ApplicationName: application.Name,
			Namespace:       environment.GetNamespace(),
			Environment:     environment.Name,
		}, nil
	}
	return nil, conureerrors.ErrProviderNotSupported
}
