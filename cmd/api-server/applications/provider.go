package applications

import (
	"github.com/coffeenights/conure/cmd/api-server/models"
	"github.com/coffeenights/conure/cmd/api-server/providers"
)

// newConureProvider returns the per-environment provider for an application.
// The dispatcher abstraction was dropped along with Vela — a single direct
// constructor is enough.
func newConureProvider(application *models.Application, environment *models.Environment) *providers.ProviderDispatcherConure {
	return &providers.ProviderDispatcherConure{
		OrganizationID:  application.OrganizationID.Hex(),
		ApplicationID:   application.ID.Hex(),
		ApplicationName: application.Name,
		Namespace:       environment.GetNamespace(),
		Environment:     environment.Name,
	}
}
