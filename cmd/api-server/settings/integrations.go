package settings

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/coffeenights/conure/cmd/api-server/conureerrors"
	"github.com/coffeenights/conure/cmd/api-server/models"
	"github.com/coffeenights/conure/cmd/api-server/variables"
)

func (a *ApiHandler) CreateIntegration(c *gin.Context) {
	// Escape the organizationID
	if _, err := primitive.ObjectIDFromHex(c.Param("organizationID")); err != nil {
		log.Printf("Error parsing organizationID: %v\n", err)
		conureerrors.AbortWithError(c, err)
		return
	}
	org := models.Organization{}
	_, err := org.GetById(a.MongoDB, c.Param("organizationID"))
	if err != nil {
		conureerrors.AbortWithError(c, conureerrors.ErrObjectNotFound)
		return
	}

	uID := c.MustGet("currentUser").(models.User).ID
	if org.AccountID != uID {
		conureerrors.AbortWithError(c, conureerrors.ErrNotAllowed)
		return
	}
	request := CreateIntegrationRequest{}
	err = c.BindJSON(&request)
	if err != nil {
		conureerrors.AbortWithError(c, conureerrors.ErrInvalidRequest)
		return
	}

	stringValue, err := convertToString(request.IntegrationValue)
	if err != nil {
		conureerrors.AbortWithError(c, err)
		return
	}
	valueEncrypted := variables.EncryptValue(a.keyStorage, stringValue)

	integration := models.Integration{
		Name:             request.Name,
		OrganizationID:   org.ID,
		IntegrationType:  request.IntegrationType,
		IntegrationValue: valueEncrypted,
	}
	err = integration.Create(a.MongoDB)
	if err != nil {
		conureerrors.AbortWithError(c, err)
		return
	}
	c.JSON(http.StatusCreated, integration)
}

func (a *ApiHandler) ListIntegrations(c *gin.Context) {
	// Escape the organizationID
	if _, err := primitive.ObjectIDFromHex(c.Param("organizationID")); err != nil {
		log.Printf("Error parsing organizationID: %v\n", err)
		conureerrors.AbortWithError(c, err)
		return
	}
	org := models.Organization{}
	_, err := org.GetById(a.MongoDB, c.Param("organizationID"))
	if errors.Is(err, conureerrors.ErrObjectNotFound) {
		conureerrors.AbortWithError(c, err)
		return
	} else if err != nil {
		log.Printf("Error getting organization: %v\n", err)
		conureerrors.AbortWithError(c, conureerrors.ErrInternalError)
		return

	}
	if org.AccountID != c.MustGet("currentUser").(models.User).ID {
		conureerrors.AbortWithError(c, conureerrors.ErrNotAllowed)
		return
	}
	integration := models.Integration{
		OrganizationID: org.ID,
	}
	integrations, err := integration.ListIntegrations(a.MongoDB)
	if err != nil {
		log.Printf("Error getting applications list: %v\n", err)
		conureerrors.AbortWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, integrations)
}

func (a *ApiHandler) DeleteIntegration(c *gin.Context) {
	// Escape the organizationID
	if _, err := primitive.ObjectIDFromHex(c.Param("organizationID")); err != nil {
		log.Printf("Error parsing organizationID: %v\n", err)
		conureerrors.AbortWithError(c, err)
		return
	}
	org := models.Organization{}
	_, err := org.GetById(a.MongoDB, c.Param("organizationID"))
	if errors.Is(err, conureerrors.ErrObjectNotFound) {
		conureerrors.AbortWithError(c, err)
		return
	} else if err != nil {
		log.Printf("Error getting organization: %v\n", err)
		conureerrors.AbortWithError(c, conureerrors.ErrInternalError)
		return

	}
	if org.AccountID != c.MustGet("currentUser").(models.User).ID {
		conureerrors.AbortWithError(c, conureerrors.ErrNotAllowed)
		return
	}
	integration := &models.Integration{}
	err = integration.GetByID(a.MongoDB, c.Param("integrationID"))
	if err != nil {
		conureerrors.AbortWithError(c, err)
	}
	err = integration.Delete(a.MongoDB)
	if err != nil {
		log.Printf("Error deleting integration: %v\n", err)
		conureerrors.AbortWithError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

func convertToString(value interface{}) (string, error) {
	bytes, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}
