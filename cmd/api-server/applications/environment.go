package applications

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/coffeenights/conure/cmd/api-server/conureerrors"
	"github.com/coffeenights/conure/cmd/api-server/models"
)

func (a *ApiHandler) CreateEnvironment(c *gin.Context) {
	request := CreateEnvironmentRequest{}
	err := c.BindJSON(&request)
	if err != nil {
		conureerrors.AbortWithError(c, conureerrors.ErrInvalidRequest)
		return
	}

	appHandler, err := NewApplicationHandler(a.MongoDB)
	if err != nil {
		log.Printf("Error creating application handler: %v\n", err)
		conureerrors.AbortWithError(c, err)
		return
	}
	if err = appHandler.GetApplicationByID(c.Param("applicationID")); err != nil {
		conureerrors.AbortWithError(c, conureerrors.ErrObjectNotFound)
		return
	}
	if appHandler.Model.AccountID != c.MustGet("currentUser").(models.User).ID {
		conureerrors.AbortWithError(c, conureerrors.ErrNotAllowed)
		return
	}

	if _, err = appHandler.Model.CreateEnvironment(a.MongoDB, request.Name); err != nil {
		log.Printf("Error creating environment: %v\n", err)
		conureerrors.AbortWithError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{})
}

func (a *ApiHandler) DeleteEnvironment(c *gin.Context) {
	appHandler, err := NewApplicationHandler(a.MongoDB)
	if err != nil {
		log.Printf("Error creating application handler: %v\n", err)
		conureerrors.AbortWithError(c, err)
		return
	}
	if err = appHandler.GetApplicationByID(c.Param("applicationID")); err != nil {
		conureerrors.AbortWithError(c, conureerrors.ErrObjectNotFound)
		return
	}
	if appHandler.Model.AccountID != c.MustGet("currentUser").(models.User).ID {
		conureerrors.AbortWithError(c, conureerrors.ErrNotAllowed)
		return
	}

	if err = appHandler.Model.DeleteEnvironmentByName(a.MongoDB, c.Param("environment")); err != nil {
		log.Printf("Error deleting environment: %v\n", err)
		conureerrors.AbortWithError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{})
}
