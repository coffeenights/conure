package applications

import (
	k8sUtils "github.com/coffeenights/conure/internal/k8s"
	"github.com/gin-gonic/gin"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"net/http"
)

func (a *AppHandler) GetOrganization(c *gin.Context) {
	organizationID := c.Param("organizationId")
	org := Organization{}
	_, err := org.GetById(a.MongoDB, organizationID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{})
		return
	}
	response := OrganizationResponse{}
	response.ParseModelToResponse(&org)
	c.JSON(http.StatusOK, response)
}

func (a *AppHandler) CreateOrganization(c *gin.Context) {
	request := CreateOrganizationRequest{}
	err := c.BindJSON(&request)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}
	org := request.ParseRequestToModel()
	_, err = org.Create(a.MongoDB)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}
	response := OrganizationResponse{}
	response.ParseModelToResponse(org)
	c.JSON(http.StatusCreated, response)
}

func (a *AppHandler) ListEnvironments(c *gin.Context) {
	// creates the clientset
	genericClientset, err := k8sUtils.GetClientset()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}
	labelSelector := metav1.ListOptions{
		LabelSelector: "usage.oam.dev/control-plane=env",
	}
	// get the k8s namespaces information
	namespaces, err := genericClientset.K8s.CoreV1().Namespaces().List(c, labelSelector)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}
	// return the information to the client
	c.JSON(http.StatusOK, gin.H{
		"namespaces": namespaces,
	})
}
