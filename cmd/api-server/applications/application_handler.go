package applications

import (
	"github.com/coffeenights/conure/cmd/api-server/database"
	"github.com/coffeenights/conure/cmd/api-server/models"
)

type Properties interface {
}

type Trait struct {
	Name       string                 `json:"name"`
	Type       string                 `json:"type"`
	Properties map[string]interface{} `json:"properties"`
}

type ApplicationHandler struct {
	ID             string
	OrganizationID string
	Model          *models.Application
	DB             *database.MongoDB
}

func NewApplicationHandler(db *database.MongoDB) (*ApplicationHandler, error) {
	return &ApplicationHandler{
		Model: &models.Application{},
		DB:    db,
	}, nil
}

func ListOrganizationApplications(organizationID string, db *database.MongoDB) ([]*ApplicationHandler, error) {
	apps, err := models.ApplicationList(db, organizationID)
	if err != nil {
		return nil, err
	}
	handlers := make([]*ApplicationHandler, len(apps))
	for i, app := range apps {
		handler, err := NewApplicationHandler(db)
		if err != nil {
			return nil, err
		}
		handler.Model = app
		handlers[i] = handler
	}
	return handlers, nil
}

func (ah *ApplicationHandler) GetApplicationByID(appID string) error {
	err := ah.Model.GetByID(ah.DB, appID)
	if err != nil {
		return err
	}
	return nil
}

func (ah *ApplicationHandler) Status(environment *models.Environment) (ProviderStatus, error) {
	status, err := NewProviderStatus(ah.Model, environment)
	if err != nil {
		return nil, err
	}
	return status, nil
}
