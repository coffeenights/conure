package applications

import (
	"errors"
	"github.com/coffeenights/conure/cmd/api-server/applications/providers"
	"github.com/coffeenights/conure/cmd/api-server/models"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"log"
	"net/http"
)

func (a *ApiHandler) ListApplications(c *gin.Context) {
	// Escape the organizationID
	if _, err := primitive.ObjectIDFromHex(c.Param("organizationID")); err != nil {
		log.Printf("Error parsing organizationID: %v\n", err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	org := models.Organization{}
	_, err := org.GetById(a.MongoDB, c.Param("organizationID"))
	if err != nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	handlers, err := ListOrganizationApplications(c.Param("organizationID"), a.MongoDB)
	if err != nil {
		log.Printf("Error getting applications list: %v\n", err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	response := ApplicationListResponse{}
	response.Organization.ParseModelToResponse(&org)
	applicationResponses := make([]ApplicationResponse, len(handlers))
	for i, handler := range handlers {
		r := ApplicationResponse{
			Application: handler.Model,
		}
		applicationResponses[i] = r
	}
	response.Applications = applicationResponses
	c.JSON(http.StatusOK, response)
}

func (a *ApiHandler) DetailApplication(c *gin.Context) {
	// Escape the organizationID
	if _, err := primitive.ObjectIDFromHex(c.Param("organizationID")); err != nil {
		log.Printf("Error parsing organizationID: %v\n", err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	// Escape the applicationID
	if _, err := primitive.ObjectIDFromHex(c.Param("applicationID")); err != nil {
		log.Printf("Error parsing applicationID: %v\n", err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	handler, err := NewApplicationHandler(a.MongoDB)
	if err != nil {
		log.Printf("Error creating application handler: %v\n", err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	err = handler.GetApplicationByID(c.Param("applicationID"))
	if err != nil {
		log.Printf("Error getting application: %v\n", err)
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	response := ApplicationResponse{
		Application: handler.Model,
	}
	c.JSON(http.StatusOK, response)
}

func (a *ApiHandler) CreateApplication(c *gin.Context) {
	// Escape the organizationID
	if _, err := primitive.ObjectIDFromHex(c.Param("organizationID")); err != nil {
		log.Printf("Error parsing organizationID: %v\n", err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	org := models.Organization{}
	_, err := org.GetById(a.MongoDB, c.Param("organizationID"))
	if err != nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	request := CreateApplicationRequest{}
	err = c.BindJSON(&request)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}
	application := models.NewApplication(c.Param("organizationID"), request.Name, primitive.NewObjectID().Hex())
	application.Description = request.Description
	_, err = application.Create(a.MongoDB)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}
	c.JSON(http.StatusCreated, application)
}

func (a *ApiHandler) DeployApplication(c *gin.Context) {
	// Escape the organizationID
	if _, err := primitive.ObjectIDFromHex(c.Param("organizationID")); err != nil {
		log.Printf("Error parsing organizationID: %v\n", err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	// Escape the applicationID
	if _, err := primitive.ObjectIDFromHex(c.Param("applicationID")); err != nil {
		log.Printf("Error parsing applicationID: %v\n", err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	handler, err := NewApplicationHandler(a.MongoDB)
	if err != nil {
		log.Printf("Error creating application handler: %v\n", err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	err = handler.GetApplicationByID(c.Param("applicationID"))
	if err != nil {
		log.Printf("Error getting application: %v\n", err)
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	env, err := handler.Model.GetEnvironmentByName(a.MongoDB, c.Param("environment"))
	if err != nil {
		log.Printf("Error getting environment: %v\n", err)
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	manifest, err := BuildApplicationManifest(handler.Model, env, a.MongoDB)
	if err != nil {
		log.Printf("Error building application manifest: %v\n", err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	provider, err := NewProviderDispatcher(handler.Model, env)
	if err != nil {
		log.Printf("Error creating provider dispatcher: %v\n", err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	err = provider.DeployApplication(manifest)
	if errors.Is(err, providers.ErrApplicationExists) {
		c.JSON(http.StatusConflict, gin.H{
			"error": err.Error(),
		})
		return
	}
	if err != nil {
		log.Printf("Error deploying application: %v\n", err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"message": "Application deployed",
	})
}
