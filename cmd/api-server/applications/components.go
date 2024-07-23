package applications

import (
	"errors"
	"github.com/coffeenights/conure/cmd/api-server/conureerrors"
	"github.com/coffeenights/conure/cmd/api-server/database"
	"github.com/coffeenights/conure/cmd/api-server/models"
	"github.com/coffeenights/conure/cmd/api-server/providers"
	k8sUtils "github.com/coffeenights/conure/internal/k8s"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"io"
	"log"
	"net/http"
	"strings"
)

func getComponentFromRoute(c *gin.Context, db *database.MongoDB) (*models.Component, error) {
	component := &models.Component{}
	err := component.GetByID(db, c.Param("componentID"))
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) || errors.Is(err, primitive.ErrInvalidHex) {
			return nil, conureerrors.ErrObjectNotFound
		}
		log.Printf("Error getting components: %v\n", err)
		return nil, err
	}
	return component, nil
}

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
	for i, _ := range components {
		response.Components[i] = ComponentResponse{
			Component: &components[i],
		}
	}
	c.JSON(http.StatusOK, response)
}

func (a *ApiHandler) DetailComponent(c *gin.Context) {
	_, err := getHandlerFromRoute(c, a.MongoDB)
	if err != nil {
		conureerrors.AbortWithError(c, err)
		return
	}
	component, err := getComponentFromRoute(c, a.MongoDB)
	if err != nil {
		conureerrors.AbortWithError(c, err)
		return
	}
	var response ComponentResponse
	response.Component = component
	c.JSON(http.StatusOK, response)
}

func (a *ApiHandler) statusLoad(c *gin.Context, component *models.Component) (ProviderStatus, error) {
	handler, err := getHandlerFromRoute(c, a.MongoDB)
	if err != nil {
		conureerrors.AbortWithError(c, err)
		return nil, err
	}
	componentFound, err := getComponentFromRoute(c, a.MongoDB)
	if err != nil {
		log.Printf("Error getting components: %v\n", err)
		return nil, err
	}
	*component = *componentFound

	// Get environment
	env, err := handler.Model.GetEnvironmentByName(a.MongoDB, c.Param("environment"))
	if err != nil {
		return nil, err
	}

	status, err := handler.Status(env)
	if errors.Is(err, k8sUtils.ErrApplicationNotFound) {
		return nil, conureerrors.ErrApplicationNotDeployed
	} else if err != nil {
		log.Printf("Error getting status: %v\n", err)
		return nil, err
	}

	return status, nil
}

func (a *ApiHandler) StatusComponentHealth(c *gin.Context) {
	var component models.Component
	status, err := a.statusLoad(c, &component)
	if err != nil {
		conureerrors.AbortWithError(c, err)
		return
	}

	response, err := status.GetComponentStatus(component.Name)
	if err != nil {
		log.Printf("Error getting component status: %v\n", err)
		conureerrors.AbortWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, response)
}

func (a *ApiHandler) StatusComponent(c *gin.Context) {
	var component models.Component
	status, err := a.statusLoad(c, &component)
	if err != nil {
		conureerrors.AbortWithError(c, err)
		return
	}

	var response ComponentStatusResponse

	// Create a channel to collect the results
	results := make(chan error)
	// Create a goroutine for each property
	go func() {
		response.Properties.ResourcesProperties, err = status.GetResourcesProperties(component.Name)
		results <- err
	}()
	go func() {
		response.Properties.NetworkProperties, err = status.GetNetworkProperties(component.Name)
		results <- err
	}()
	go func() {
		response.Properties.StorageProperties, err = status.GetStorageProperties(component.Name)
		results <- err
	}()
	go func() {
		response.Properties.SourceProperties, err = status.GetSourceProperties(component.Name)
		results <- err
	}()
	go func() {
		response.Properties.Health, err = status.GetComponentStatus(component.Name)
		results <- err
	}()
	// Collect the results
	for i := 0; i < 5; i++ {
		err := <-results
		if err != nil {
			log.Printf("Error getting properties: %v\n", err)
			conureerrors.AbortWithError(c, err)
			return
		}
	}
	close(results)
	response.Component.Component = &component
	c.JSON(http.StatusOK, response)
}

func (a *ApiHandler) CreateComponent(c *gin.Context) {
	handler, err := getHandlerFromRoute(c, a.MongoDB)
	if err != nil {
		conureerrors.AbortWithError(c, err)
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
		Settings:      request.Settings,
	}
	err = component.Create(a.MongoDB)
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

func (a *ApiHandler) UpdateComponent(c *gin.Context) {
	handler, err := getHandlerFromRoute(c, a.MongoDB)
	if err != nil {
		conureerrors.AbortWithError(c, err)
		return
	}

	component, err := getComponentFromRoute(c, a.MongoDB)
	if err != nil {
		conureerrors.AbortWithError(c, err)
		return
	}

	var request CreateComponentRequest
	err = c.BindJSON(&request)
	if err != nil {
		log.Printf("Error binding request: %v\n", err)
		conureerrors.AbortWithError(c, conureerrors.ErrInvalidRequest)
		return
	}

	component.Name = request.Name
	component.Type = request.Type
	component.Description = request.Description
	component.ApplicationID = handler.Model.ID
	component.Settings = request.Settings

	err = component.Update(a.MongoDB)
	if errors.Is(err, conureerrors.ErrObjectAlreadyExists) {
		conureerrors.AbortWithError(c, err)
		return
	} else if err != nil {
		log.Printf("Error updating component: %v\n", err)
		conureerrors.AbortWithError(c, err)
		return
	}
	c.JSON(http.StatusOK, component)

}

func (a *ApiHandler) DeleteComponent(c *gin.Context) {
	_, err := getHandlerFromRoute(c, a.MongoDB)
	if err != nil {
		conureerrors.AbortWithError(c, err)
		return
	}

	component, err := getComponentFromRoute(c, a.MongoDB)
	if err != nil {
		conureerrors.AbortWithError(c, err)
		return
	}

	err = component.Delete(a.MongoDB)
	if err != nil {
		log.Printf("Error deleting component: %v\n", err)
		conureerrors.AbortWithError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (a *ApiHandler) ComponentPods(c *gin.Context) {
	var response ComponentPodsResponse
	var component models.Component
	status, err := a.statusLoad(c, &component)
	if err != nil {
		conureerrors.AbortWithError(c, err)
		return
	}

	pods, err := status.GetPodList(component.Name)
	if err != nil {
		log.Printf("Error getting pods: %v\n", err)
		conureerrors.AbortWithError(c, err)
		return
	}

	response.Pods = pods
	c.JSON(http.StatusOK, response)
}

func (a *ApiHandler) StreamLogs(c *gin.Context) {
	var component models.Component
	status, err := a.statusLoad(c, &component)
	if err != nil {
		conureerrors.AbortWithError(c, err)
		return
	}

	podQuery, ok := c.GetQuery("pods")
	if !ok || podQuery == "" {
		conureerrors.AbortWithError(c, conureerrors.ErrInvalidRequest)
		return
	}

	pods := strings.Split(podQuery, ",")

	logStream := providers.NewLogStream()
	for _, podName := range pods {
		go status.StreamLogs(c.Request.Context(), podName, logStream, 20)
	}

	// Set necessary headers for SSE
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Stream(func(w io.Writer) bool {
		// Stream message to client from message channel
		select {
		case msg := <-logStream.Stream:
			c.SSEvent("message", msg)
			return true
		case err := <-logStream.Error:
			conureerrors.AbortWithError(c, err)
			c.SSEvent("error", err.Error())
			return false
		}
	})
}
