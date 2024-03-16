package applications

import (
	"errors"
	k8sUtils "github.com/coffeenights/conure/internal/k8s"
	"github.com/gin-gonic/gin"
	"github.com/oam-dev/kubevela-core-api/apis/core.oam.dev/common"
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
			&component,
		}
	}
	c.JSON(http.StatusOK, response)
}

func (a *ApiHandler) DetailComponent(c *gin.Context) {
	clientset, err := k8sUtils.GetClientset()
	if err != nil {
		log.Printf("Error getting clientset: %v\n", err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	application := NewApplication(c.Param("organizationID"), "", "")
	namespace := application.GetNamespace()

	labels := map[string]string{
		"conure.io/organization-id": c.Param("organizationID"),
		"conure.io/application-id":  c.Param("applicationID"),
		"conure.io/environment":     c.Param("environment"),
	}

	applicationDef, err := k8sUtils.GetApplicationByLabels(clientset, namespace, labels)
	if err != nil {
		switch {
		case errors.Is(err, k8sUtils.ErrApplicationNotFound):
			c.AbortWithStatus(http.StatusNotFound)
			return
		default:
			log.Printf("Error getting application: %v\n", err)
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
	}

	// Extract the component
	var componentSpec common.ApplicationComponent
	for _, comp := range applicationDef.Spec.Components {
		if comp.Name == c.Param("componentName") {
			componentSpec = comp
			break
		}
	}
	if componentSpec.Name == "" {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	// Extract the component Status
	var componentStatus common.ApplicationComponentStatus
	for _, comp := range applicationDef.Status.Services {
		if comp.Name == c.Param("componentName") {
			componentStatus = comp
			break
		}
	}

	if componentStatus.Name == "" {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	componentProperties := ComponentProperties{
		Name: componentSpec.Name,
		Type: componentSpec.Type,
	}
	wl, err := NewK8sWorkload(application, &componentSpec, &componentStatus)
	if err != nil {
		log.Printf("Error creating workload: %v\n", err)
	}
	componentProperties.NetworkProperties, err = wl.GetNetworkProperties()
	if err != nil {
		log.Printf("Error extracting network properties: %v\n", err)
	}
	componentProperties.ResourcesProperties, err = wl.GetResourcesProperties()
	if err != nil {
		log.Printf("Error extracting resources properties: %v\n", err)
	}

	componentProperties.StorageProperties, err = wl.GetStorageProperties()
	if err != nil {
		log.Printf("Error extracting storage properties: %v\n", err)
	}

	componentProperties.SourceProperties, err = wl.GetSourceProperties()
	if err != nil {
		log.Printf("Error extracting source properties: %v\n", err)
	}

	var componentResponse ServiceComponentResponse
	componentResponse.ComponentProperties = componentProperties
	c.JSON(http.StatusOK, componentResponse)
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
		"app.oam.dev/component":    c.Param("componentName"),
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
