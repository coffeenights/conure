package applications

import "github.com/gin-gonic/gin"

const (
	DeploymentResourceType  = "deployment"
	StatefulSetResourceType = "statefulset"
	ServiceResourceType     = "service"
)

func GetNamespaceFromParams(c *gin.Context) string {
	return c.Param("organizationID") + "-" + c.Param("applicationID") + "-" + c.Param("environment")
}
