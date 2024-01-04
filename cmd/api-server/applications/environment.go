package applications

import (
	k8sUtils "github.com/coffeenights/conure/internal/k8s"
	"github.com/gin-gonic/gin"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (a *AppHandler) ListEnvironments(c *gin.Context) {
	// creates the clientset
	genericClientset, err := k8sUtils.GetClientset()
	if err != nil {
		c.JSON(500, gin.H{
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
		c.JSON(500, gin.H{
			"error": err.Error(),
		})
		return
	}
	// return the information to the client
	c.JSON(200, gin.H{
		"namespaces": namespaces,
	})
}
