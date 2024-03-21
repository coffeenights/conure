package applications

import (
	"errors"
	k8sUtils "github.com/coffeenights/conure/internal/k8s"
	"github.com/gin-gonic/gin"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	component, err = component.GetByID(a.MongoDB, c.Param("componentID"))
	if err != nil {
		log.Printf("Error getting components: %v\n", err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	var response ComponentResponse
	response.Component = component
	c.JSON(http.StatusOK, response)
}

func (a *ApiHandler) StatusComponent(c *gin.Context) {
	// obtain the deployment related to the component
	clientset, err := k8sUtils.GetClientset()
	if err != nil {
		log.Printf("Error getting clientset: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}
	namespace := GetNamespaceFromParams(c)
	labels := map[string]string{
		"conure.io/application-id": c.Param("applicationID"),
		"app.oam.dev/component":    c.Param("componentID"),
	}

	cd, err := clientset.Vela.CoreV1beta1().ComponentDefinitions("vela-system").Get(c, "webservice", metav1.GetOptions{})
	_ = cd
	configmap, err := clientset.K8s.CoreV1().ConfigMaps("vela-system").Get(c, "webservice", metav1.GetOptions{})
	_ = configmap
	deployments, err := k8sUtils.GetDeploymentByLabels(clientset.K8s, namespace, labels)
	if err != nil {
		log.Printf("Error getting deployments: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}
	if len(deployments) == 0 {
		c.JSON(http.StatusNotFound, gin.H{})
		return
	}
	deployment := deployments[0]
	var statusResponse ServiceComponentStatusResponse
	statusResponse.FromClientsetToResponse(deployment)
	c.JSON(http.StatusOK, statusResponse)
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
	component := NewComponent(app, request.Name, request.Type)
	component.Description = request.Description
	component.Properties = request.Properties
	_, err = component.Create(a.MongoDB)
	if err != nil {
		log.Printf("Error creating component: %v\n", err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	} else if errors.Is(err, ErrDuplicateDocument) {
		c.AbortWithStatusJSON(http.StatusConflict, gin.H{
			"error": err.Error(),
		})
		return
	}
	c.JSON(http.StatusCreated, component)
}
