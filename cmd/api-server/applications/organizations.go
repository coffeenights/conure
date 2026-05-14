package applications

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/coffeenights/conure/cmd/api-server/conureerrors"
	"github.com/coffeenights/conure/cmd/api-server/middlewares"
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
	if !middlewares.CanReadOrg(c.MustGet("currentUser").(models.User), &org) {
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
	err := c.ShouldBind(&request)
	if err != nil {
		conureerrors.AbortWithError(c, err)
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

func (a *ApiHandler) ListOrganization(c *gin.Context) {
	user := c.MustGet("currentUser").(models.User)
	var orgs []*models.Organization
	var err error
	if user.IsAdmin() {
		orgs, err = models.OrganizationListAll(a.MongoDB)
	} else {
		orgs, err = models.OrganizationListForUser(a.MongoDB, user.ID, user.OrganizationID)
	}
	if err != nil {
		conureerrors.AbortWithError(c, conureerrors.ErrInternalError)
		return
	}

	orgResponses := make([]OrganizationResponse, len(orgs))
	for i, org := range orgs {
		r := OrganizationResponse{
			Organization: org,
		}
		orgResponses[i] = r
	}
	response := OrganizationListResponse{
		Organizations: orgResponses,
	}
	c.JSON(http.StatusOK, response)
}
