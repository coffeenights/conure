package environments

import (
	k8sUtil "github.com/coffeenights/conure/internal/k8s"
	"github.com/gin-gonic/gin"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func ListEnvironments(c *gin.Context) {
	// creates the clientset
	genericClientset, err := k8sUtil.GetClientset()
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

//func GetEnvironment(c *gin.Context) {
//	// creates the clientset
//	genericClientset, err := k8sUtil.GetClientset()
//	if err != nil {
//		c.JSON(500, gin.H{
//			"error": err.Error(),
//		})
//		return
//	}
//	// get the k8s namespaces information
//	namespace, err := genericClientset.K8s.CoreV1().Namespaces().Get(c.Param("name"), metav1.GetOptions{})
//	if err != nil {
//		c.JSON(500, gin.H{
//			"error": err.Error(),
//		})
//		return
//	}
//	// return the information to the client
//	c.JSON(200, gin.H{
//		"namespace": namespace,
//	})
//}
