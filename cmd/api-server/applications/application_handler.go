package applications

import (
	"github.com/coffeenights/conure/cmd/api-server/database"
	"log"
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

func (ah *ApplicationHandler) Status(environment *Environment) ProviderStatus {
	status, err := NewProviderStatus(ah.Model, environment)
	if err != nil {
		log.Panicf("Error getting provider status: %v\n", err)
	}
	return status
}
