package applications

import (
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

	namespace := c.Param("organizationID") + "-" + c.Param("applicationID") + "-" + c.Param("environment")
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
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	namespace := c.Param("organizationID") + "-" + c.Param("applicationID") + "-" + c.Param("environment")
	listOptions := metav1.ListOptions{
		LabelSelector: "conure.io/organization-id=" + c.Param("organizationID") + ",conure.io/application-id=" + c.Param("applicationID") + ",conure.io/environment=" + c.Param("environment"),
	}

	// Get application manifest
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
	// Extract the component
	var componentSpec common.ApplicationComponent
	for _, comp := range application.Spec.Components {
		if comp.Name == c.Param("componentName") {
			componentSpec = comp
			break
		}
	}
	if componentSpec.Name == "" {
		c.JSON(http.StatusNotFound, gin.H{})
		return
	}

	// Extract the component Status
	var componentStatus common.ApplicationComponentStatus
	for _, comp := range application.Status.Services {
		if comp.Name == c.Param("componentName") {
			componentStatus = comp
			break
		}
	}

	if componentStatus.Name == "" {
		c.JSON(http.StatusNotFound, gin.H{})
		return
	}

	var componentResponse ServiceComponentResponse
	componentResponse.FromClientsetToResponse(componentSpec, componentStatus)
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
	namespace := c.Param("organizationID") + "-" + c.Param("applicationID") + "-" + c.Param("environment")
	labels := map[string]string{
		"conure.io/application-id": c.Param("applicationID"),
		"app.oam.dev/component":    c.Param("componentName"),
	}

	// Get deployment
	resource, err := GetResourceByLabel("statefulset", clientset.K8s, namespace, labels)
	_ = resource
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
