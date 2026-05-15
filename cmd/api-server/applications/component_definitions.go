package applications

import (
	"context"
	"net/http"
	"sort"

	"github.com/gin-gonic/gin"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	conurev1alpha1 "github.com/coffeenights/conure/apis/core/v1alpha1"
	"github.com/coffeenights/conure/cmd/api-server/conureerrors"
	"github.com/coffeenights/conure/cmd/api-server/middlewares"
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

// listClusterComponentDefinitions returns all ComponentDefinition CRDs in the
// cluster. ComponentDefinitions are cluster-scoped (not per-org); this is the
// single source of truth shared by the CLI type picker and engine resolution
// on component create, replacing the old per-org MongoDB componenttypespecs
// collection.
func (a *ApiHandler) listClusterComponentDefinitions(ctx context.Context) ([]conurev1alpha1.ComponentDefinition, error) {
	clientset, err := a.kubeClient()
	if err != nil {
		return nil, err
	}
	list, err := clientset.Conure.CoreV1alpha1().ComponentDefinitions().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return list.Items, nil
}

// ListComponentDefinitions returns the component types available for use in
// the given organization. Used by the CLI wizard to populate the type picker.
//
// Component definitions are cluster-scoped ComponentDefinition CRDs (the same
// objects the controller resolves at render time), so this reads them live
// from the cluster rather than from a per-org MongoDB collection. The org is
// still resolved and authorized so the endpoint keeps its 404/403 contract.
//
// Path: GET /:organizationID/component-definitions
func (a *ApiHandler) ListComponentDefinitions(c *gin.Context) {
	organizationID := c.Param("organizationID")
	org := models.Organization{}
	if _, err := org.GetById(a.MongoDB, organizationID); err != nil {
		conureerrors.AbortWithError(c, conureerrors.ErrObjectNotFound)
		return
	}
	if !middlewares.CanReadOrg(c.MustGet("currentUser").(models.User), &org) {
		conureerrors.AbortWithError(c, conureerrors.ErrNotAllowed)
		return
	}

	items, err := a.listClusterComponentDefinitions(c.Request.Context())
	if err != nil {
		conureerrors.AbortWithError(c, conureerrors.ErrInternalError)
		return
	}

	defs := make([]ComponentDefinitionResponse, len(items))
	for i := range items {
		cd := &items[i]
		defs[i] = ComponentDefinitionResponse{
			ID:   cd.Name,
			Name: cd.Name,
			Type: cd.Spec.ComponentType,
			// Engine is optional on the CRD; the CLI treats empty as timoni.
			Engine:        string(cd.Spec.Engine),
			Description:   cd.Spec.Description,
			OCIRepository: cd.Spec.OCIRepository,
			OCITag:        cd.Spec.OCITag,
			// ComponentDefinition has no icon field; CLI handles a nil icon.
			IconURL: nil,
		}
	}
	// Stable order so the CLI picker doesn't reshuffle between calls
	// (List does not guarantee ordering).
	sort.Slice(defs, func(i, j int) bool { return defs[i].Name < defs[j].Name })

	c.JSON(http.StatusOK, ComponentDefinitionListResponse{Definitions: defs})
}
