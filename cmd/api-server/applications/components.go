package applications

import (
	k8sUtils "github.com/coffeenights/conure/internal/k8s"
	"github.com/gin-gonic/gin"
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
	var applicationResponse ApplicationResponse
	applicationResponse.FromVelaClientsetToResponse(&application)
	response := ServiceComponentListResponse{
		Application: applicationResponse,
		Components:  componentResponse,
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
	_ = application
	labels := map[string]string{
		"conure.io/application-id": c.Param("applicationID"),
	}

	// Get deployment
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
	// deployment := deployments[0]

}
