package applications

import (
	"errors"
	k8sUtils "github.com/coffeenights/conure/internal/k8s"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/coffeenights/conure/cmd/api-server/conureerrors"
	"github.com/coffeenights/conure/cmd/api-server/models"
)

func (a *ApiHandler) ListApplications(c *gin.Context) {
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
	handlers, err := ListOrganizationApplications(c.Param("organizationID"), a.MongoDB)
	if err != nil {
		log.Printf("Error getting applications list: %v\n", err)
		conureerrors.AbortWithError(c, err)
		return
	}

	response := ApplicationListResponse{}
	response.Organization = OrganizationResponse{
		Organization: &org,
	}
	applicationResponses := make([]ApplicationResponse, len(handlers))
	for i, handler := range handlers {
		totalComponents, err := handler.Model.CountComponents(a.MongoDB)
		if err != nil {
			log.Printf("Error counting components: %v\n", err)
			conureerrors.AbortWithError(c, err)
			return
		}
		r := ApplicationResponse{
			Application:     handler.Model,
			TotalComponents: totalComponents,
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
		conureerrors.AbortWithError(c, err)
		return
	}
	// Escape the applicationID
	if _, err := primitive.ObjectIDFromHex(c.Param("applicationID")); err != nil {
		log.Printf("Error parsing applicationID: %v\n", err)
		conureerrors.AbortWithError(c, err)
		return
	}

	handler, err := NewApplicationHandler(a.MongoDB)
	if err != nil {
		log.Printf("Error creating application handler: %v\n", err)
		conureerrors.AbortWithError(c, err)
		return
	}
	err = handler.GetApplicationByID(c.Param("applicationID"))
	if err != nil {
		log.Printf("Error getting application: %v\n", err)
		conureerrors.AbortWithError(c, err)
		return
	}

	if handler.Model.AccountID != c.MustGet("currentUser").(models.User).ID {
		conureerrors.AbortWithError(c, conureerrors.ErrNotAllowed)
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
	request := CreateApplicationRequest{}
	err = c.BindJSON(&request)
	if err != nil {
		conureerrors.AbortWithError(c, conureerrors.ErrInvalidRequest)
		return
	}
	application := models.NewApplication(c.Param("organizationID"), request.Name, uID.Hex())
	application.Description = request.Description
	_, err = application.Create(a.MongoDB)
	if err != nil {
		conureerrors.AbortWithError(c, err)
		return
	}
	c.JSON(http.StatusCreated, application)
}

func (a *ApiHandler) DeployApplication(c *gin.Context) {
	handler, err := a.getHandlerFromRoute(c)
	if err != nil {
		conureerrors.AbortWithError(c, err)
		return
	}

	env, err := handler.Model.GetEnvironmentByName(a.MongoDB, c.Param("environment"))
	if err != nil {
		log.Printf("Error getting environment: %v\n", err)
		conureerrors.AbortWithError(c, conureerrors.ErrObjectNotFound)
		return
	}
	manifest, err := BuildApplicationManifest(handler.Model, env, a.MongoDB)
	if err != nil {
		log.Printf("Error building application manifest: %v\n", err)
		conureerrors.AbortWithError(c, err)
		return
	}
	provider, err := NewProviderDispatcher(handler.Model, env)
	if err != nil {
		log.Printf("Error creating provider dispatcher: %v\n", err)
		conureerrors.AbortWithError(c, err)
		return
	}
	err = provider.DeployApplication(manifest)
	if errors.Is(err, conureerrors.ErrApplicationExists) {
		conureerrors.AbortWithError(c, err)
		return
	}
	if err != nil {
		log.Printf("Error deploying application: %v\n", err)
		conureerrors.AbortWithError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"message": "Application deployed",
	})
}

func (a *ApiHandler) StatusApplication(c *gin.Context) {
	handler, err := a.getHandlerFromRoute(c)
	if err != nil {
		conureerrors.AbortWithError(c, err)
		return
	}

	env, err := handler.Model.GetEnvironmentByName(a.MongoDB, c.Param("environment"))
	if err != nil {
		log.Printf("Error getting environment: %v\n", err)
		conureerrors.AbortWithError(c, conureerrors.ErrObjectNotFound)
		return
	}
	status, err := handler.Status(env)
	if errors.Is(err, k8sUtils.ErrApplicationNotFound) {
		conureerrors.AbortWithError(c, conureerrors.ErrApplicationNotDeployed)
		return
	} else if err != nil {
		log.Printf("Error getting status: %v\n", err)
		conureerrors.AbortWithError(c, err)
		return
	}
	appStatus, err := status.GetApplicationStatus()
	if err != nil {
		log.Printf("Error getting application status: %v\n", err)
		conureerrors.AbortWithError(c, err)
		return
	}
	var response ApplicationStatusResponse
	response.Status = ApplicationStatus(appStatus)
	c.JSON(http.StatusOK, gin.H{"status": appStatus})

}

func (a *ApiHandler) getHandlerFromRoute(c *gin.Context) (*ApplicationHandler, error) {
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

	handler, err := NewApplicationHandler(a.MongoDB)
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
