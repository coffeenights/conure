package applications

import (
	"github.com/coffeenights/conure/cmd/api-server/models"
	"github.com/gin-gonic/gin"
	"net/http"
)

func (a *ApiHandler) DetailOrganization(c *gin.Context) {
	organizationID := c.Param("organizationID")
	org := models.Organization{}
	_, err := org.GetById(a.MongoDB, organizationID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{})
		return
	}
	response := OrganizationResponse{}
	response.ParseModelToResponse(&org)
	c.JSON(http.StatusOK, response)
}

func (a *ApiHandler) CreateOrganization(c *gin.Context) {
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
