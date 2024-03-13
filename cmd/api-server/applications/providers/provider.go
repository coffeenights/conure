package providers

import (
	apiConfig "github.com/coffeenights/conure/cmd/api-server/config"
	"github.com/coffeenights/conure/internal/config"
)

type ProviderType string

const (
	Vela ProviderType = "vela"
)

type ProviderStatus interface {
}

func NewProviderStatus() (ProviderStatus, error) {
	appConfig := config.LoadConfig(apiConfig.Config{})

	providerType := ProviderType(appConfig.ProviderSource)

	switch providerType {
	case Vela:
		return &ProviderStatusVela{}, nil
	}
	return nil, ErrProviderNotSupported
}
