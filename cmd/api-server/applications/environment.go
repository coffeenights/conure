package applications

import (
	"github.com/coffeenights/conure/cmd/api-server/models"
	"github.com/gin-gonic/gin"
	"log"
	"net/http"
)

func (a *ApiHandler) CreateEnvironment(c *gin.Context) {
	request := CreateEnvironmentRequest{}
	err := c.BindJSON(&request)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	appHandler, err := NewApplicationHandler(a.MongoDB)
	if err != nil {
		log.Printf("Error creating application handler: %v\n", err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	if err = appHandler.GetApplicationByID(c.Param("applicationID")); err != nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	if appHandler.Model.AccountID != c.MustGet("currentUser").(models.User).ID {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
			"error": "You are not allowed to access this application",
		})
		return
	}

	if _, err = appHandler.Model.CreateEnvironment(a.MongoDB, request.Name); err != nil {
		log.Printf("Error creating environment: %v\n", err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	c.JSON(http.StatusCreated, gin.H{})
}

func (a *ApiHandler) DeleteEnvironment(c *gin.Context) {
	appHandler, err := NewApplicationHandler(a.MongoDB)
	if err != nil {
		log.Printf("Error creating application handler: %v\n", err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	if err = appHandler.GetApplicationByID(c.Param("applicationID")); err != nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	if appHandler.Model.AccountID != c.MustGet("currentUser").(models.User).ID {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
			"error": "You are not allowed to access this application",
		})
		return
	}

	if err = appHandler.Model.DeleteEnvironmentByName(a.MongoDB, c.Param("environment")); err != nil {
		log.Printf("Error deleting environment: %v\n", err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	c.JSON(http.StatusOK, gin.H{})
}
