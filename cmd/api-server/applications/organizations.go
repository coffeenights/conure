package applications

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/coffeenights/conure/cmd/api-server/conureerrors"
	"github.com/coffeenights/conure/cmd/api-server/models"
)

func (a *ApiHandler) DetailOrganization(c *gin.Context) {
	organizationID := c.Param("organizationID")
	org := models.Organization{}
	_, err := org.GetById(a.MongoDB, organizationID)
	if err != nil {
		conureerrors.AbortWithError(c, conureerrors.ErrObjectNotFound)
		return
	}
	if org.AccountID != c.MustGet("currentUser").(models.User).ID {
		conureerrors.AbortWithError(c, conureerrors.ErrNotAllowed)
		return
	}
	response := OrganizationResponse{
		Organization: &org,
	}
	c.JSON(http.StatusOK, response)
}

func (a *ApiHandler) CreateOrganization(c *gin.Context) {
	request := CreateOrganizationRequest{}
	err := c.BindJSON(&request)
	if err != nil {
		conureerrors.AbortWithError(c, conureerrors.ErrInvalidRequest)
		return
	}
	org := request.ParseRequestToModel()
	org.AccountID = c.MustGet("currentUser").(models.User).ID
	_, err = org.Create(a.MongoDB)
	if err != nil {
		conureerrors.AbortWithError(c, conureerrors.ErrInvalidRequest)
		return
	}
	response := OrganizationResponse{
		Organization: org,
	}
	c.JSON(http.StatusCreated, response)
}
