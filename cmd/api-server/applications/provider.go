package applications

import (
	"context"

	conurev1alpha1 "github.com/coffeenights/conure/apis/core/v1alpha1"
	apiConfig "github.com/coffeenights/conure/cmd/api-server/config"
	"github.com/coffeenights/conure/cmd/api-server/conureerrors"
	"github.com/coffeenights/conure/cmd/api-server/models"
	"github.com/coffeenights/conure/cmd/api-server/providers"
	"github.com/coffeenights/conure/internal/config"
)

type ProviderType string

const (
	Vela   ProviderType = "vela"
	Conure ProviderType = "conure"
)

type ProviderStatus interface {
	GetApplicationStatus() (string, error)
	GetNetworkProperties(componentName string) (*providers.NetworkProperties, error)
	GetResourcesProperties(componentName string) (*providers.ResourcesProperties, error)
	GetStorageProperties(componentName string) (*providers.StorageProperties, error)
	GetSourceProperties(componentName string) (*providers.SourceProperties, error)
	GetComponentStatus(componentName string) (*providers.ComponentStatusHealth, error)
	GetPodList(componentName string) ([]providers.Pod, error)
	StreamLogs(c context.Context, podName string, logStream *providers.LogStream, linesBuffer int)
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
	DeployApplication(app *conurev1alpha1.Application, components []conurev1alpha1.Component) error
	UpdateApplication(app *conurev1alpha1.Application, components []conurev1alpha1.Component) error
}

func NewProviderDispatcher(application *models.Application, environment *models.Environment) (ProviderDispatcher, error) {
	appConfig := config.LoadConfig(apiConfig.Config{})
	providerType := ProviderType(appConfig.ProviderSource)

	switch providerType {
	case Conure:
		return &providers.ProviderDispatcherConure{
			OrganizationID:  application.OrganizationID.Hex(),
			ApplicationID:   application.ID.Hex(),
			ApplicationName: application.Name,
			Namespace:       environment.GetNamespace(),
			Environment:     environment.Name,
		}, nil
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
