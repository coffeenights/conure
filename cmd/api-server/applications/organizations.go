package applications

import (
	"errors"
	"github.com/go-playground/validator/v10"
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
	err := c.ShouldBind(&request)
	var validationErr validator.ValidationErrors
	if errors.As(err, &validationErr) {
		conureerrors.AbortWithValidationError(c, validationErr)
		return
	} else if err != nil {
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
	uID := c.MustGet("currentUser").(models.User).ID
	orgs, err := models.OrganizationList(a.MongoDB, uID.Hex())
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
