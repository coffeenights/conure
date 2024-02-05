package applications

import (
	k8sUtils "github.com/coffeenights/conure/internal/k8s"
	"github.com/gin-gonic/gin"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"log"
	"net/http"
	"strings"
)

func (a *AppHandler) CreateEnvironment(c *gin.Context) {
	request := CreateEnvironmentRequest{}
	err := c.BindJSON(&request)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	// creates the clientset
	genericClientset, err := k8sUtils.GetClientset()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}
	options := metav1.CreateOptions{}
	namespace := corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: request.OrganizationID + "-" + request.ApplicationID + "-" + request.Name,
			Labels: map[string]string{
				"conure.io/application-id":  request.ApplicationID,
				"conure.io/organization-id": request.OrganizationID,
			},
		},
	}
	_, err = genericClientset.K8s.CoreV1().Namespaces().Create(c, &namespace, options)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}
	c.JSON(http.StatusCreated, gin.H{})
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
		LabelSelector: "conure.io/application-id=" + c.Param("applicationID") + ",conure.io/organization-id=" + c.Param("organizationID"),
	}
	// get the k8s namespaces information
	namespaces, err := genericClientset.K8s.CoreV1().Namespaces().List(c, labelSelector)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}
	environments := EnvironmentListResponse{}

	for i := range namespaces.Items {
		ns := namespaces.Items[i].ObjectMeta.Name
		nsParts := strings.Split(ns, "-")
		if len(nsParts) < 3 {
			log.Printf("namespace %s does not have the correct format", ns)
			continue
		}
		environmentNameParts := nsParts[2:]
		environmentName := strings.Join(environmentNameParts, "-")
		environments.Environments = append(environments.Environments, EnvironmentResponse{
			Name: environmentName,
		})
	}
	// return the information to the client
	c.JSON(http.StatusOK, environments)
}

func (a *AppHandler) DeleteEnvironment(c *gin.Context) {
	// creates the clientset
	genericClientset, err := k8sUtils.GetClientset()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}
	namespace := c.Param("organizationID") + "-" + c.Param("applicationID") + "-" + c.Param("environmentID")
	// get the k8s namespaces information
	err = genericClientset.K8s.CoreV1().Namespaces().Delete(c, namespace, metav1.DeleteOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{})
}
