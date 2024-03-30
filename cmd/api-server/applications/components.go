package applications

import (
	"errors"
	k8sUtils "github.com/coffeenights/conure/internal/k8s"
	"github.com/gin-gonic/gin"
	"log"
	"net/http"
)

func (a *ApiHandler) ListComponents(c *gin.Context) {
	handler, err := NewApplicationHandler(a.MongoDB)
	if err != nil {
		log.Printf("Error creating application handler: %v\n", err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	_, err = handler.Model.GetByID(a.MongoDB, c.Param("applicationID"))
	if err != nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	components, err := handler.Model.ListComponents(a.MongoDB)
	if err != nil {
		log.Printf("Error getting components: %v\n", err)
		c.AbortWithStatus(http.StatusInternalServerError)
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
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	_, err = handler.Model.GetByID(a.MongoDB, c.Param("applicationID"))
	if err != nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	component := &Component{}
	_, err = component.GetByID(a.MongoDB, c.Param("componentID"))
	if err != nil {
		if err.Error() == "mongo: no documents in result" {
			c.AbortWithStatus(http.StatusNotFound)
			return
		}
		log.Printf("Error getting components: %v\n", err)
		c.AbortWithStatus(http.StatusInternalServerError)
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
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	_, err = handler.Model.GetByID(a.MongoDB, c.Param("applicationID"))
	if err != nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	component := &Component{}
	_, err = component.GetByID(a.MongoDB, c.Param("componentID"))
	if err != nil {
		if err.Error() == "mongo: no documents in result" {
			c.AbortWithStatus(http.StatusNotFound)
			return
		}
		log.Printf("Error getting components: %v\n", err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	// Get environment
	env, err := handler.Model.GetEnvironmentByName(a.MongoDB, c.Param("environment"))
	if errors.Is(err, ErrDocumentNotFound) {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	var response ComponentStatusResponse

	status, err := handler.Status(env)
	if errors.Is(err, k8sUtils.ErrApplicationNotFound) {
		log.Printf("Error getting status: %v\n", err)
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{
			"error": err.Error(),
		})
		return
	} else if err != nil {
		log.Printf("Error getting status: %v\n", err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return

	}
	response.Properties.ResourcesProperties, err = status.GetResourcesProperties(component.ID)
	if err != nil {
		log.Printf("Error getting resources properties: %v\n", err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	response.Properties.NetworkProperties, err = status.GetNetworkProperties(component.ID)
	if err != nil {
		log.Printf("Error getting network properties: %v\n", err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	response.Properties.StorageProperties, err = status.GetStorageProperties(component.ID)
	if err != nil {
		log.Printf("Error getting storage properties: %v\n", err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	response.Properties.SourceProperties, err = status.GetSourceProperties(component.ID)
	if err != nil {
		log.Printf("Error getting source properties: %v\n", err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	response.Component.Component = component
	c.JSON(http.StatusOK, response)

}

func (a *ApiHandler) CreateComponent(c *gin.Context) {
	handler, err := NewApplicationHandler(a.MongoDB)
	if err != nil {
		log.Printf("Error creating application handler: %v\n", err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	app, err := handler.Model.GetByID(a.MongoDB, c.Param("applicationID"))
	if err != nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	var request CreateComponentRequest
	err = c.BindJSON(&request)
	if err != nil {
		log.Printf("Error binding request: %v\n", err)
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	component := NewComponent(app, request.ID, request.Type)
	component.Description = request.Description
	component.Properties = request.Properties
	component.Traits = request.Traits
	_, err = component.Create(a.MongoDB)
	if errors.Is(err, ErrDuplicateDocument) {
		c.AbortWithStatusJSON(http.StatusConflict, gin.H{
			"error": err.Error(),
		})
		return
	} else if err != nil {
		log.Printf("Error creating component: %v\n", err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	c.JSON(http.StatusCreated, component)
}
