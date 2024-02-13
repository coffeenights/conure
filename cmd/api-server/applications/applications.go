package applications

import (
	"log"
	"net/http"
	"strings"

	k8sUtils "github.com/coffeenights/conure/internal/k8s"
	"github.com/gin-gonic/gin"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (a *AppHandler) ListApplications(c *gin.Context) {
	// q is the query param that represents the search term
	q := c.DefaultQuery("q", "")
	// creates the clientset
	clientset, err := k8sUtils.GetClientset()
	if err != nil {
		log.Printf("Error getting clientset: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	listOptions := metav1.ListOptions{
		LabelSelector: "conure.io/organization-id=" + c.Param("organizationID") + ",conure.io/main=true",
	}
	applications, err := clientset.Vela.CoreV1beta1().Applications("").List(c, listOptions)
	if err != nil {
		log.Printf("Error getting applications: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	var response []ApplicationResponse
	for _, app := range applications.Items { // Apply filtering based on the query parameter
		if q != "" && !strings.Contains(app.ObjectMeta.Name, q) {
			continue
		}

		var r ApplicationResponse
		r.FromVelaClientsetToResponse(&app)

		if err != nil {
			log.Printf("Error getting application: %v\n", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
			})
			return
		}

		response = append(response, r)
	}
	c.JSON(http.StatusOK, response)
}

//func (a *AppHandler) DetailApplications(c *gin.Context) {
//	// q is the query param that represents the search term
//	applicationName := c.Param("applicationName")
//
//	// creates the clientset
//	clientset, err := k8sUtils.GetClientset()
//	if err != nil {
//		log.Fatal(err.Error())
//	}
//
//	application, err := clientset.Vela.CoreV1beta1().Applications("default").Get(c, applicationName, metav1.GetOptions{})
//	if err != nil {
//		log.Fatal(err.Error())
//	}
//
//	var response ApplicationResponse
//	response.FromVelaClientsetToResponse(application)
//
//	labels := map[string]string{
//		"app.oam.dev/name": application.Name,
//	}
//	deployments, err := getDeploymentByLabels(clientset.K8s, "default", labels)
//	if err != nil {
//		fmt.Printf("Error getting deployment: %v\n", err)
//	}
//
//	for _, deployment := range deployments {
//		services, err := getServicesByLabels(clientset.K8s, "default", labels)
//		if err != nil {
//			fmt.Printf("Error getting services: %v\n", err)
//		}
//
//		var c ServiceComponentResponse
//		c.FromClientsetToResponse(deployment, services)
//		//response.Components = append(response.Components, c)
//	}
//	//response.TotalComponents = len(response.Components)
//
//	c.JSON(http.StatusOK, response)
//}
