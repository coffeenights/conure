package applications

import (
	k8sUtils "github.com/coffeenights/conure/internal/k8s"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"log"
	"net/http"
)

func (a *ApiHandler) ListApplications(c *gin.Context) {
	// Escape the organizationID
	if _, err := primitive.ObjectIDFromHex(c.Param("organizationID")); err != nil {
		log.Printf("Error parsing organizationID: %v\n", err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	org := Organization{}
	_, err := org.GetById(a.MongoDB, c.Param("organizationID"))
	if err != nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	handlers, err := ListOrganizationApplications(c.Param("organizationID"), a.MongoDB)
	if err != nil {
		log.Printf("Error getting applications list: %v\n", err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	response := ApplicationListResponse{}
	response.Organization.ParseModelToResponse(&org)
	applicationResponses := make([]ApplicationResponse, len(handlers))
	for i, handler := range handlers {
		r := ApplicationResponse{
			Application: handler.Model,
		}
		applicationResponses[i] = r
	}
	response.Applications = applicationResponses
	c.JSON(http.StatusOK, response)
	return
}

func (a *ApiHandler) DetailApplication(c *gin.Context) {
	// Escape the organizationID
	if _, err := primitive.ObjectIDFromHex(c.Param("organizationID")); err != nil {
		log.Printf("Error parsing organizationID: %v\n", err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	// Escape the applicationID
	if _, err := primitive.ObjectIDFromHex(c.Param("applicationID")); err != nil {
		log.Printf("Error parsing applicationID: %v\n", err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	handler, err := NewApplicationHandler(a.MongoDB)
	err = handler.GetApplicationByID(c.Param("applicationID"))
	if err != nil {
		log.Printf("Error getting application: %v\n", err)
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	response := ApplicationResponse{
		Application: handler.Model,
	}
	c.JSON(http.StatusOK, response)
	return
}

func (a *ApiHandler) DetailApplicationOld(c *gin.Context) {
	clientset, err := k8sUtils.GetClientset()
	if err != nil {
		log.Printf("Error getting clientset: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}
	listOptions := metav1.ListOptions{
		LabelSelector: "conure.io/organization-id=" + c.Param("organizationID") + ",conure.io/application-id=" + c.Param("applicationID") + ",conure.io/environment=" + c.Param("environment"),
	}
	namespace := c.Param("organizationID") + "-" + c.Param("applicationID") + "-" + c.Param("environment")
	apps, err := clientset.Vela.CoreV1beta1().Applications(namespace).List(c, listOptions)
	if err != nil {
		log.Printf("Error getting application: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}
	if len(apps.Items) == 0 {
		c.JSON(http.StatusNotFound, gin.H{})
		return
	}
	app := apps.Items[0]

	listOptions = metav1.ListOptions{
		LabelSelector: "app.oam.dev/app-revision-hash=" + app.Status.LatestRevision.RevisionHash,
	}
	revisions, err := clientset.Vela.CoreV1beta1().ApplicationRevisions(app.Namespace).List(c, listOptions)
	if err != nil {
		log.Printf("Error getting application revision: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}
	if revisions.Items == nil {
		log.Fatal("Error getting application revision: revision not found")
	}
	rev := revisions.Items[0]

	var appResponse ApplicationResponseOld
	appResponse.FromVelaClientsetToResponse(&app, &rev)

	r := ApplicationResponse{
		//Application: appResponse,
	}
	c.JSON(http.StatusOK, r)
}
