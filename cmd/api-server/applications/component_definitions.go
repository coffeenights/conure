package applications

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/coffeenights/conure/cmd/api-server/conureerrors"
	"github.com/coffeenights/conure/cmd/api-server/models"
)

type ComponentDefinitionResponse struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`
	Type          string  `json:"type"`
	Engine        string  `json:"engine"`
	Description   string  `json:"description"`
	OCIRepository string  `json:"oci_repository"`
	OCITag        string  `json:"oci_tag"`
	IconURL       *string `json:"icon_url"`
}

type ComponentDefinitionListResponse struct {
	Definitions []ComponentDefinitionResponse `json:"definitions"`
}

// ListComponentDefinitions returns the component types available for use in
// the given organization. Used by the CLI wizard to populate the type picker.
//
// Path: GET /:orgID/component-definitions
func (a *ApiHandler) ListComponentDefinitions(c *gin.Context) {
	organizationID := c.Param("organizationID")
	org := models.Organization{}
	if _, err := org.GetById(a.MongoDB, organizationID); err != nil {
		conureerrors.AbortWithError(c, conureerrors.ErrObjectNotFound)
		return
	}
	if org.AccountID != c.MustGet("currentUser").(models.User).ID {
		conureerrors.AbortWithError(c, conureerrors.ErrNotAllowed)
		return
	}

	specs, err := models.ComponentTypeSpecList(c.Request.Context(), a.MongoDB, organizationID)
	if err != nil {
		conureerrors.AbortWithError(c, conureerrors.ErrInternalError)
		return
	}

	defs := make([]ComponentDefinitionResponse, len(specs))
	for i, s := range specs {
		defs[i] = ComponentDefinitionResponse{
			ID:            s.ID.Hex(),
			Name:          s.Name,
			Type:          s.Type,
			Engine:        s.Engine,
			Description:   s.Description,
			OCIRepository: s.OCIRepository,
			OCITag:        s.OCITag,
			IconURL:       s.IconURL,
		}
	}
	c.JSON(http.StatusOK, ComponentDefinitionListResponse{Definitions: defs})
}
