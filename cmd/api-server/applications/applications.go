package applications

import (
	"log"
	"net/http"
	"strings"

	k8sUtils "github.com/coffeenights/conure/internal/k8s"
	"github.com/gin-gonic/gin"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (a *ApiHandler) ListApplications(c *gin.Context) {
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
		// Get revision
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

		var r ApplicationResponse
		r.FromVelaClientsetToResponse(&app, &rev)

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

func (a *ApiHandler) DetailApplication(c *gin.Context) {
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

	var appResponse ApplicationResponse
	appResponse.FromVelaClientsetToResponse(&app, &rev)

	r := ApplicationDetailsResponse{
		Application: appResponse,
	}
	c.JSON(http.StatusOK, r)
}
