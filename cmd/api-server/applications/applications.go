package applications

import (
	"errors"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/coffeenights/conure/cmd/api-server/conureerrors"
	"github.com/coffeenights/conure/cmd/api-server/middlewares"
	"github.com/coffeenights/conure/cmd/api-server/models"
)

func (a *ApiHandler) ListApplications(c *gin.Context) {
	if _, err := primitive.ObjectIDFromHex(c.Param("organizationID")); err != nil {
		log.Printf("Error parsing organizationID: %v\n", err)
		conureerrors.AbortWithError(c, conureerrors.ErrInvalidRequest)
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
	user := c.MustGet("currentUser").(models.User)
	if !middlewares.CanReadOrg(user, &org) {
		conureerrors.AbortWithError(c, conureerrors.ErrNotAllowed)
		return
	}
	handlers, err := ListOrganizationApplications(c.Param("organizationID"), a.MongoDB)
	if err != nil {
		log.Printf("Error getting applications list: %v\n", err)
		conureerrors.AbortWithError(c, err)
		return
	}

	response := ApplicationListResponse{
		Organization: OrganizationResponse{Organization: &org},
	}
	applicationResponses := make([]ApplicationResponse, len(handlers))
	for i, handler := range handlers {
		totalComponents, err := handler.Model.CountComponents(a.MongoDB)
		if err != nil {
			log.Printf("Error counting components: %v\n", err)
			conureerrors.AbortWithError(c, err)
			return
		}
		applicationResponses[i] = ApplicationResponse{
			Application:     handler.Model,
			TotalComponents: totalComponents,
		}
	}
	response.Applications = applicationResponses
	c.JSON(http.StatusOK, response)
}

func (a *ApiHandler) DetailApplication(c *gin.Context) {
	if _, err := primitive.ObjectIDFromHex(c.Param("organizationID")); err != nil {
		log.Printf("Error parsing organizationID: %v\n", err)
		conureerrors.AbortWithError(c, conureerrors.ErrInvalidRequest)
		return
	}
	if _, err := primitive.ObjectIDFromHex(c.Param("applicationID")); err != nil {
		log.Printf("Error parsing applicationID: %v\n", err)
		conureerrors.AbortWithError(c, conureerrors.ErrInvalidRequest)
		return
	}

	handler, err := NewApplicationHandler(a.MongoDB)
	if err != nil {
		log.Printf("Error creating application handler: %v\n", err)
		conureerrors.AbortWithError(c, err)
		return
	}
	if err := handler.GetApplicationByID(c.Param("applicationID")); err != nil {
		log.Printf("Error getting application: %v\n", err)
		conureerrors.AbortWithError(c, err)
		return
	}
	// Read access is gated on org membership, not app ownership — any
	// developer in the org can read apps created by their peers.
	orgForApp := models.Organization{}
	if _, err := orgForApp.GetById(a.MongoDB, handler.Model.OrganizationID.Hex()); err != nil {
		conureerrors.AbortWithError(c, conureerrors.ErrObjectNotFound)
		return
	}
	if !middlewares.CanReadOrg(c.MustGet("currentUser").(models.User), &orgForApp) {
		conureerrors.AbortWithError(c, conureerrors.ErrNotAllowed)
		return
	}
	c.JSON(http.StatusOK, ApplicationResponse{Application: handler.Model})
}

func (a *ApiHandler) CreateApplication(c *gin.Context) {
	if _, err := primitive.ObjectIDFromHex(c.Param("organizationID")); err != nil {
		log.Printf("Error parsing organizationID: %v\n", err)
		conureerrors.AbortWithError(c, conureerrors.ErrInvalidRequest)
		return
	}
	org := models.Organization{}
	_, err := org.GetById(a.MongoDB, c.Param("organizationID"))
	if err != nil {
		conureerrors.AbortWithError(c, conureerrors.ErrObjectNotFound)
		return
	}
	user := c.MustGet("currentUser").(models.User)
	uID := user.ID
	// Write access here means "create an app in this org" — a developer
	// only needs to belong to the org (read access). Org-level mutations
	// (rename, delete the org itself) are gated separately by CanWriteOrg.
	if !middlewares.CanReadOrg(user, &org) {
		conureerrors.AbortWithError(c, conureerrors.ErrNotAllowed)
		return
	}
	request := CreateApplicationRequest{}
	if err := c.BindJSON(&request); err != nil {
		conureerrors.AbortWithError(c, conureerrors.ErrInvalidRequest)
		return
	}
	application := models.NewApplication(c.Param("organizationID"), request.Name, uID.Hex())
	application.Description = request.Description
	if _, err := application.Create(a.MongoDB); err != nil {
		conureerrors.AbortWithError(c, err)
		return
	}
	c.JSON(http.StatusCreated, application)
}
