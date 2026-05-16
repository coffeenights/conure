package applications

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/coffeenights/conure/cmd/api-server/conureerrors"
	"github.com/coffeenights/conure/cmd/api-server/middlewares"
	"github.com/coffeenights/conure/cmd/api-server/models"
)

type ComponentDefinitionResponse struct {
	ID            string            `json:"id"`
	Name          string            `json:"name"`
	Type          string            `json:"type"`
	Engine        string            `json:"engine"`
	Description   string            `json:"description"`
	OCIRepository string            `json:"oci_repository"`
	OCITag        string            `json:"oci_tag"`
	IconURL       *string           `json:"icon_url"`
	Buildable     bool              `json:"buildable"`
	FieldRoles    map[string]string `json:"field_roles,omitempty"`
	// Source distinguishes a row inherited from the shipped defaults
	// ("default") from one the org created or overrode ("organization"), so
	// the UI can show which definitions are customizable vs. inherited.
	Source string `json:"source"`
}

type ComponentDefinitionListResponse struct {
	Definitions []ComponentDefinitionResponse `json:"definitions"`
}

// componentDefinitionRequest is the create/update body. Engine is optional and
// defaults to timoni (matching the CRD). Type is required.
type componentDefinitionRequest struct {
	Type               string                   `json:"type" binding:"required"`
	Description        string                   `json:"description"`
	Engine             string                   `json:"engine"`
	OCIRepository      string                   `json:"oci_repository"`
	OCITag             string                   `json:"oci_tag"`
	OCIDigest          string                   `json:"oci_digest"`
	OCIRegistry        string                   `json:"oci_registry"`
	RegistrySecretName string                   `json:"registry_secret_name"`
	Helm               *models.ComponentDefHelm `json:"helm"`
	Buildable          bool                     `json:"buildable"`
	FieldRoles         map[string]string        `json:"field_roles"`
	IconURL            *string                  `json:"icon_url"`
}

func toComponentDefinitionResponse(cd *models.ComponentDefinition) ComponentDefinitionResponse {
	source := "organization"
	if cd.OrganizationID == models.DefaultComponentDefinitionOwner {
		source = "default"
	}
	return ComponentDefinitionResponse{
		ID:            cd.ID.Hex(),
		Name:          cd.Type,
		Type:          cd.Type,
		Engine:        cd.EngineKey(),
		Description:   cd.Description,
		OCIRepository: cd.OCIRepository,
		OCITag:        cd.OCITag,
		IconURL:       cd.IconURL,
		Buildable:     cd.Buildable,
		FieldRoles:    cd.FieldRoles,
		Source:        source,
	}
}

// resolveOrg loads and authorizes the org for a component-definition request.
// read=true uses the read gate; otherwise the write gate (owner/admin). It
// aborts the request and returns ok=false on any failure so callers can just
// `if !ok { return }`.
func (a *ApiHandler) resolveOrg(c *gin.Context, read bool) (models.Organization, bool) {
	organizationID := c.Param("organizationID")
	org := models.Organization{}
	if _, err := org.GetById(a.MongoDB, organizationID); err != nil {
		conureerrors.AbortWithError(c, conureerrors.ErrObjectNotFound)
		return org, false
	}
	user := c.MustGet("currentUser").(models.User)
	allowed := middlewares.CanReadOrg(user, &org)
	if !read {
		allowed = middlewares.CanWriteOrg(user, &org)
	}
	if !allowed {
		conureerrors.AbortWithError(c, conureerrors.ErrNotAllowed)
		return org, false
	}
	return org, true
}

// ListComponentDefinitions returns the component types available for use in
// the given organization. Used by the CLI wizard to populate the type picker.
//
// Definitions are now org-scoped with MongoDB as the source of truth: this
// resolves the shipped defaults overlaid with the org's own
// overrides/tombstones (models.ResolveForOrg) — the exact same rule the
// deploy path uses to materialize the CRD into the cluster, so the picker and
// what the controller actually renders can't disagree.
//
// Path: GET /:organizationID/component-definitions
func (a *ApiHandler) ListComponentDefinitions(c *gin.Context) {
	org, ok := a.resolveOrg(c, true)
	if !ok {
		return
	}

	items, err := models.ResolveForOrg(c.Request.Context(), a.MongoDB, org.ID)
	if err != nil {
		conureerrors.AbortWithError(c, conureerrors.ErrInternalError)
		return
	}
	defs := make([]ComponentDefinitionResponse, len(items))
	for i := range items {
		defs[i] = toComponentDefinitionResponse(&items[i])
	}
	c.JSON(http.StatusOK, ComponentDefinitionListResponse{Definitions: defs})
}

// CreateComponentDefinition creates or overrides an org's definition for a
// (type, engine). If the org already has a row for that key it is updated
// (including un-hiding a tombstone); otherwise a new org-owned row is
// inserted. Shipped defaults are never mutated — an org row layered on top
// shadows the default for this org only.
//
// Path: POST /:organizationID/component-definitions
func (a *ApiHandler) CreateComponentDefinition(c *gin.Context) {
	org, ok := a.resolveOrg(c, false)
	if !ok {
		return
	}
	var req componentDefinitionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		conureerrors.AbortWithError(c, conureerrors.ErrInvalidRequest)
		return
	}

	cd := models.ComponentDefinition{}
	err := cd.GetOwnRow(c.Request.Context(), a.MongoDB, org.ID, req.Type, req.Engine)
	exists := err == nil
	if err != nil && err != conureerrors.ErrObjectNotFound {
		conureerrors.AbortWithError(c, conureerrors.ErrDatabaseError)
		return
	}

	applyRequest(&cd, &req)
	cd.OrganizationID = org.ID
	cd.Hidden = false

	if exists {
		if err := cd.Update(c.Request.Context(), a.MongoDB); err != nil {
			conureerrors.AbortWithError(c, conureerrors.ErrDatabaseError)
			return
		}
		c.JSON(http.StatusOK, toComponentDefinitionResponse(&cd))
		return
	}
	if _, err := cd.Create(c.Request.Context(), a.MongoDB); err != nil {
		conureerrors.AbortWithError(c, err)
		return
	}
	c.JSON(http.StatusCreated, toComponentDefinitionResponse(&cd))
}

// DeleteComponentDefinition removes an org-owned definition row. The path id
// must be an org-owned row (override or tombstone): default-owned rows are
// shared and read-only here — to remove an inherited default for this org,
// POST a tombstone via HideComponentDefinition instead. Deleting an override
// restores the inherited default; deleting a tombstone un-hides it.
//
// Path: DELETE /:organizationID/component-definitions/:definitionID
func (a *ApiHandler) DeleteComponentDefinition(c *gin.Context) {
	org, ok := a.resolveOrg(c, false)
	if !ok {
		return
	}
	cd := models.ComponentDefinition{}
	if _, err := cd.GetById(c.Request.Context(), a.MongoDB, c.Param("definitionID")); err != nil {
		conureerrors.AbortWithError(c, err)
		return
	}
	if cd.OrganizationID != org.ID {
		// Either it belongs to another org or it's a shared default. Don't
		// leak existence; treat as not found for this org.
		conureerrors.AbortWithError(c, conureerrors.ErrObjectNotFound)
		return
	}
	if err := cd.Delete(c.Request.Context(), a.MongoDB); err != nil {
		conureerrors.AbortWithError(c, conureerrors.ErrDatabaseError)
		return
	}
	c.Status(http.StatusNoContent)
}

// HideComponentDefinition writes (or refreshes) a tombstone for a (type,
// engine) in the org, suppressing the inherited default. Reversible: DELETE
// the resulting row to restore the default. Hiding an org override replaces
// it with a tombstone.
//
// Path: POST /:organizationID/component-definitions/hide
func (a *ApiHandler) HideComponentDefinition(c *gin.Context) {
	org, ok := a.resolveOrg(c, false)
	if !ok {
		return
	}
	var req struct {
		Type   string `json:"type" binding:"required"`
		Engine string `json:"engine"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		conureerrors.AbortWithError(c, conureerrors.ErrInvalidRequest)
		return
	}

	cd := models.ComponentDefinition{}
	err := cd.GetOwnRow(c.Request.Context(), a.MongoDB, org.ID, req.Type, req.Engine)
	exists := err == nil
	if err != nil && err != conureerrors.ErrObjectNotFound {
		conureerrors.AbortWithError(c, conureerrors.ErrDatabaseError)
		return
	}
	cd.OrganizationID = org.ID
	cd.Type = req.Type
	cd.Engine = req.Engine
	cd.Hidden = true
	if exists {
		if err := cd.Update(c.Request.Context(), a.MongoDB); err != nil {
			conureerrors.AbortWithError(c, conureerrors.ErrDatabaseError)
			return
		}
		c.JSON(http.StatusOK, toComponentDefinitionResponse(&cd))
		return
	}
	if _, err := cd.Create(c.Request.Context(), a.MongoDB); err != nil {
		conureerrors.AbortWithError(c, err)
		return
	}
	c.JSON(http.StatusCreated, toComponentDefinitionResponse(&cd))
}

// applyRequest copies the request body onto a definition row, preserving the
// row's identity fields (ID/OrganizationID/timestamps) so it works for both
// create and update.
func applyRequest(cd *models.ComponentDefinition, req *componentDefinitionRequest) {
	cd.Type = req.Type
	cd.Description = req.Description
	cd.Engine = req.Engine
	cd.OCIRepository = req.OCIRepository
	cd.OCITag = req.OCITag
	cd.OCIDigest = req.OCIDigest
	cd.OCIRegistry = req.OCIRegistry
	cd.RegistrySecretName = req.RegistrySecretName
	cd.Helm = req.Helm
	cd.Buildable = req.Buildable
	cd.FieldRoles = req.FieldRoles
	cd.IconURL = req.IconURL
}
