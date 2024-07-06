package applications

import (
	"github.com/coffeenights/conure/cmd/api-server/conureerrors"
	"github.com/coffeenights/conure/cmd/api-server/database"
	"github.com/coffeenights/conure/cmd/api-server/models"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
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

func getHandlerFromRoute(c *gin.Context, db *database.MongoDB) (*ApplicationHandler, error) {
	// Escape the organizationID
	if _, err := primitive.ObjectIDFromHex(c.Param("organizationID")); err != nil {
		log.Printf("Error parsing organizationID: %v\n", err)
		return nil, err

	}
	// Escape the applicationID
	if _, err := primitive.ObjectIDFromHex(c.Param("applicationID")); err != nil {
		log.Printf("Error parsing applicationID: %v\n", err)
		return nil, err
	}

	handler, err := NewApplicationHandler(db)
	if err != nil {
		log.Printf("Error creating application handler: %v\n", err)
		return nil, err
	}
	err = handler.GetApplicationByID(c.Param("applicationID"))
	if err != nil {
		log.Printf("Error getting application: %v\n", err)
		return nil, conureerrors.ErrObjectNotFound
	}
	if handler.Model.AccountID != c.MustGet("currentUser").(models.User).ID {
		return nil, conureerrors.ErrNotAllowed
	}

	return handler, nil
}
