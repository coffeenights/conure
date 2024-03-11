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

func (a *AppHandler) ListComponents(c *gin.Context) {
	clientset, err := k8sUtils.GetClientset()
	if err != nil {
		log.Printf("Error getting clientset: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	namespace := GetNamespaceFromParams(c)
	listOptions := metav1.ListOptions{
		LabelSelector: "conure.io/organization-id=" + c.Param("organizationID") + ",conure.io/application-id=" + c.Param("applicationID") + ",conure.io/environment=" + c.Param("environment"),
	}
	applications, err := clientset.Vela.CoreV1beta1().Applications(namespace).List(c, listOptions)
	if err != nil {
		log.Printf("Error getting applications: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}
	if len(applications.Items) == 0 {
		c.JSON(http.StatusNotFound, gin.H{})
		return
	}
	application := applications.Items[0]
	var componentResponse []ServiceComponentShortResponse
	for _, componentSpec := range application.Spec.Components {
		var component ServiceComponentShortResponse
		component.FromClientsetToResponse(componentSpec)
		componentResponse = append(componentResponse, component)
	}
	response := ServiceComponentListResponse{
		Components: componentResponse,
	}

	c.JSON(http.StatusOK, response)
}

func (a *AppHandler) DetailComponent(c *gin.Context) {
	clientset, err := k8sUtils.GetClientset()
	if err != nil {
		log.Printf("Error getting clientset: %v\n", err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	application := NewApplication(c.Param("organizationID"), c.Param("applicationID"), c.Param("environment"))
	namespace := application.getNamespace()

	labels := map[string]string{
		"conure.io/organization-id": c.Param("organizationID"),
		"conure.io/application-id":  c.Param("applicationID"),
		"conure.io/environment":     c.Param("environment"),
	}

	applicationDef, err := getApplicationByLabels(clientset, namespace, labels)
	if err != nil {
		switch {
		case errors.Is(err, ErrApplicationNotFound):
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

func (a *AppHandler) StatusComponent(c *gin.Context) {
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
	deployments, err := getDeploymentByLabels(clientset.K8s, namespace, labels)
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
