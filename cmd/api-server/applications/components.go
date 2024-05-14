package applications

import (
	"errors"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/coffeenights/conure/cmd/api-server/conureerrors"
	"github.com/coffeenights/conure/cmd/api-server/models"
	k8sUtils "github.com/coffeenights/conure/internal/k8s"
)

func (a *ApiHandler) ListComponents(c *gin.Context) {
	application := &models.Application{}
	err := application.GetByID(a.MongoDB, c.Param("applicationID"))
	if errors.Is(err, conureerrors.ErrObjectNotFound) {
		conureerrors.AbortWithError(c, err)
		return
	} else if err != nil {
		log.Printf("Error getting application: %v\n", err)
		conureerrors.AbortWithError(c, err)
		return
	}
	if application.AccountID != c.MustGet("currentUser").(models.User).ID {
		conureerrors.AbortWithError(c, conureerrors.ErrNotAllowed)
		return
	}
	components, err := application.ListComponents(a.MongoDB)
	if err != nil {
		log.Printf("Error getting components: %v\n", err)
		conureerrors.AbortWithError(c, err)
		return
	}
	var response ComponentListResponse
	response.Components = make([]ComponentResponse, len(components))
	for i, component := range components {
		response.Components[i] = ComponentResponse{
			Component: &component,
		}
	}
	c.JSON(http.StatusOK, response)
}

func (a *ApiHandler) DetailComponent(c *gin.Context) {
	handler, err := NewApplicationHandler(a.MongoDB)
	if err != nil {
		log.Printf("Error creating application handler: %v\n", err)
		conureerrors.AbortWithError(c, err)
		return
	}
	err = handler.Model.GetByID(a.MongoDB, c.Param("applicationID"))
	if errors.Is(err, conureerrors.ErrObjectNotFound) {
		conureerrors.AbortWithError(c, err)
		return
	} else if err != nil {
		log.Printf("Error getting application: %v\n", err)
		conureerrors.AbortWithError(c, err)
		return
	}

	if handler.Model.AccountID != c.MustGet("currentUser").(models.User).ID {
		conureerrors.AbortWithError(c, conureerrors.ErrNotAllowed)

		return
	}

	component := &models.Component{}
	_, err = component.GetByID(a.MongoDB, c.Param("componentID"))
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) || errors.Is(err, primitive.ErrInvalidHex) {
			conureerrors.AbortWithError(c, conureerrors.ErrObjectNotFound)
			return
		}
		log.Printf("Error getting components: %v\n", err)
		conureerrors.AbortWithError(c, err)
		return
	}
	var response ComponentResponse
	response.Component = component
	c.JSON(http.StatusOK, response)
}

func (a *ApiHandler) StatusComponent(c *gin.Context) {
	handler, err := NewApplicationHandler(a.MongoDB)
	if err != nil {
		log.Printf("Error creating application handler: %v\n", err)
		conureerrors.AbortWithError(c, err)
		return
	}
	err = handler.Model.GetByID(a.MongoDB, c.Param("applicationID"))
	if errors.Is(err, conureerrors.ErrObjectNotFound) {
		conureerrors.AbortWithError(c, err)
		return
	} else if err != nil {
		log.Printf("Error getting application: %v\n", err)
		conureerrors.AbortWithError(c, err)
		return
	}
	if handler.Model.AccountID != c.MustGet("currentUser").(models.User).ID {
		conureerrors.AbortWithError(c, conureerrors.ErrNotAllowed)

		return
	}

	component := &models.Component{}
	_, err = component.GetByID(a.MongoDB, c.Param("componentID"))
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) || errors.Is(err, primitive.ErrInvalidHex) {
			conureerrors.AbortWithError(c, conureerrors.ErrObjectNotFound)
			return
		}
		log.Printf("Error getting components: %v\n", err)
		conureerrors.AbortWithError(c, err)
		return
	}

	// Get environment
	env, err := handler.Model.GetEnvironmentByName(a.MongoDB, c.Param("environment"))
	if errors.Is(err, conureerrors.ErrObjectNotFound) {
		conureerrors.AbortWithError(c, err)
		return
	}

	var response ComponentStatusResponse

	status, err := handler.Status(env)
	if errors.Is(err, k8sUtils.ErrApplicationNotFound) {
		log.Printf("Error getting status: %v\n", err)
		conureerrors.AbortWithError(c, conureerrors.ErrObjectNotFound)
		return
	} else if err != nil {
		log.Printf("Error getting status: %v\n", err)
		conureerrors.AbortWithError(c, err)
		return
	}
	response.Properties.ResourcesProperties, err = status.GetResourcesProperties(component.Name)
	if err != nil {
		log.Printf("Error getting resources properties: %v\n", err)
		conureerrors.AbortWithError(c, err)
		return
	}
	response.Properties.NetworkProperties, err = status.GetNetworkProperties(component.Name)
	if err != nil {
		log.Printf("Error getting network properties: %v\n", err)
		conureerrors.AbortWithError(c, err)
		return
	}
	response.Properties.StorageProperties, err = status.GetStorageProperties(component.Name)
	if err != nil {
		log.Printf("Error getting storage properties: %v\n", err)
		conureerrors.AbortWithError(c, err)
		return
	}
	response.Properties.SourceProperties, err = status.GetSourceProperties(component.Name)
	if err != nil {
		log.Printf("Error getting source properties: %v\n", err)
		conureerrors.AbortWithError(c, err)
		return
	}
	response.Properties.Status, err = status.GetComponentStatus(component.Name)
	if err != nil {
		log.Printf("Error getting component status: %v\n", err)
		conureerrors.AbortWithError(c, err)
		return
	}

	response.Component.Component = component
	c.JSON(http.StatusOK, response)

}

func (a *ApiHandler) CreateComponent(c *gin.Context) {
	handler, err := NewApplicationHandler(a.MongoDB)
	if err != nil {
		log.Printf("Error creating application handler: %v\n", err)
		conureerrors.AbortWithError(c, err)
		return
	}
	err = handler.Model.GetByID(a.MongoDB, c.Param("applicationID"))
	if errors.Is(err, conureerrors.ErrObjectNotFound) {
		conureerrors.AbortWithError(c, err)
		return
	} else if err != nil {
		log.Printf("Error getting application: %v\n", err)
		conureerrors.AbortWithError(c, err)
		return
	}
	if handler.Model.AccountID != c.MustGet("currentUser").(models.User).ID {
		conureerrors.AbortWithError(c, conureerrors.ErrNotAllowed)
		return
	}
	var request CreateComponentRequest
	err = c.BindJSON(&request)
	if err != nil {
		log.Printf("Error binding request: %v\n", err)
		conureerrors.AbortWithError(c, conureerrors.ErrInvalidRequest)
		return
	}
	component := models.Component{
		Name:          request.Name,
		Type:          request.Type,
		Description:   request.Description,
		ApplicationID: handler.Model.ID,
		Properties:    request.Properties,
		Traits:        request.Traits,
	}
	_, err = component.Create(a.MongoDB)
	if errors.Is(err, conureerrors.ErrObjectAlreadyExists) {
		conureerrors.AbortWithError(c, err)
		return
	} else if err != nil {
		log.Printf("Error creating component: %v\n", err)
		conureerrors.AbortWithError(c, err)
		return
	}
	c.JSON(http.StatusCreated, component)
}
