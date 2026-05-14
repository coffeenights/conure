package applications

import (
	"context"
	"errors"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/coffeenights/conure/cmd/api-server/conureerrors"
	"github.com/coffeenights/conure/cmd/api-server/database"
	"github.com/coffeenights/conure/cmd/api-server/models"
)

// resolveComponentEngine determines which rendering engine should own a new
// Component. It mirrors the controller-side lookup so the API rejects
// ambiguous or unsupported requests up-front:
//
//   - requested engine pinned + matches a definition for the type → use it.
//   - requested engine pinned + no matching definition → ErrUnsupportedComponentEngine.
//   - engine unset + exactly one definition matches the type → use its engine.
//   - engine unset + multiple definitions match → ErrAmbiguousComponentEngine.
//   - no definition at all for the type → return "" so the controller writes
//     a useful Rendered=Failed condition (the Mongo path has no ComponentDefinition
//     registry of its own to consult beyond this list).
func resolveComponentEngine(ctx context.Context, db *database.MongoDB, organizationID, compType, requested string) (string, error) {
	specs, err := models.ComponentTypeSpecList(ctx, db, organizationID)
	if err != nil {
		return "", conureerrors.ErrInternalError
	}
	var matches []*models.ComponentTypeSpec
	for _, s := range specs {
		if s.Type != compType {
			continue
		}
		if requested != "" && s.Engine != "" && s.Engine != requested {
			continue
		}
		matches = append(matches, s)
	}
	if requested != "" {
		// Caller pinned the engine. Accept it even if no definition is
		// known locally (admins might register definitions only on the
		// cluster side), but if we DO have local definitions for the type
		// and none match, reject.
		hasAnyForType := false
		for _, s := range specs {
			if s.Type == compType {
				hasAnyForType = true
				break
			}
		}
		if hasAnyForType && len(matches) == 0 {
			return "", conureerrors.ErrUnsupportedComponentEngine
		}
		return requested, nil
	}
	switch len(matches) {
	case 0:
		// No matching definition known to the API; pass through and let
		// the controller report the missing definition.
		return "", nil
	case 1:
		return matches[0].Engine, nil
	default:
		return "", conureerrors.ErrAmbiguousComponentEngine
	}
}

func getComponentFromRoute(c *gin.Context, db *database.MongoDB) (*models.Component, error) {
	component := &models.Component{}
	if err := component.GetByID(db, c.Param("componentID")); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) || errors.Is(err, primitive.ErrInvalidHex) {
			return nil, conureerrors.ErrObjectNotFound
		}
		log.Printf("Error getting component: %v\n", err)
		return nil, err
	}
	return component, nil
}

// ListComponents returns every component identity for an application along
// with per-environment presence and drift summary.
//
// Path: GET /:orgID/a/:appID/c
func (a *ApiHandler) ListComponents(c *gin.Context) {
	handler, err := getHandlerFromRoute(c, a.MongoDB)
	if err != nil {
		conureerrors.AbortWithError(c, err)
		return
	}
	dbComponents, err := handler.Model.ListComponents(a.MongoDB)
	if err != nil {
		log.Printf("Error listing components: %v\n", err)
		conureerrors.AbortWithError(c, err)
		return
	}

	envByID := map[string]models.Environment{}
	for _, env := range handler.Model.Environments {
		envByID[env.ID] = env
	}

	ctx := c.Request.Context()
	response := ComponentListResponse{Components: make([]ComponentResponse, len(dbComponents))}
	for i := range dbComponents {
		comp := dbComponents[i]
		envs, err := buildPresenceForComponent(ctx, a, &comp, handler.Model.Environments)
		if err != nil {
			log.Printf("Error rolling up environment presence: %v\n", err)
			conureerrors.AbortWithError(c, err)
			return
		}
		response.Components[i] = ComponentResponse{
			Component:    &comp,
			Environments: envs,
		}
	}
	c.JSON(http.StatusOK, response)
}

// GetComponent returns the component identity plus a per-env revision summary.
//
// Path: GET /:orgID/a/:appID/c/:componentID
func (a *ApiHandler) GetComponent(c *gin.Context) {
	handler, err := getHandlerFromRoute(c, a.MongoDB)
	if err != nil {
		conureerrors.AbortWithError(c, err)
		return
	}
	component, err := getComponentFromRoute(c, a.MongoDB)
	if err != nil {
		conureerrors.AbortWithError(c, err)
		return
	}
	if component.ApplicationID != handler.Model.ID {
		conureerrors.AbortWithError(c, conureerrors.ErrObjectNotFound)
		return
	}

	envs, err := buildPresenceForComponent(c.Request.Context(), a, component, handler.Model.Environments)
	if err != nil {
		log.Printf("Error rolling up environment presence: %v\n", err)
		conureerrors.AbortWithError(c, err)
		return
	}
	c.JSON(http.StatusOK, ComponentResponse{
		Component:    component,
		Environments: envs,
	})
}

// CreateComponent registers a new component identity and writes its first
// draft revision in the target environment.
//
// Path: POST /:orgID/a/:appID/c
func (a *ApiHandler) CreateComponent(c *gin.Context) {
	handler, err := getHandlerFromRouteForWrite(c, a.MongoDB)
	if err != nil {
		conureerrors.AbortWithError(c, err)
		return
	}

	var request CreateComponentRequest
	if err := c.BindJSON(&request); err != nil {
		log.Printf("Error binding request: %v\n", err)
		conureerrors.AbortWithError(c, conureerrors.ErrInvalidRequest)
		return
	}

	env, err := handler.Model.GetEnvironmentByName(a.MongoDB, request.Environment)
	if err != nil {
		conureerrors.AbortWithError(c, conureerrors.ErrObjectNotFound)
		return
	}

	// Resolve which engine renders this component. The CRD lookup will run
	// the same (type, engine) match later; we resolve here so the choice is
	// persisted on the Component identity and reused across deploys.
	engine, err := resolveComponentEngine(c.Request.Context(), a.MongoDB, handler.Model.OrganizationID.Hex(), request.Type, request.Engine)
	if err != nil {
		conureerrors.AbortWithError(c, err)
		return
	}

	component := &models.Component{
		Name:          request.Name,
		Type:          request.Type,
		Engine:        engine,
		Description:   request.Description,
		ApplicationID: handler.Model.ID,
	}
	if err := component.Create(a.MongoDB); err != nil {
		if errors.Is(err, conureerrors.ErrObjectAlreadyExists) {
			conureerrors.AbortWithError(c, err)
			return
		}
		log.Printf("Error creating component: %v\n", err)
		conureerrors.AbortWithError(c, err)
		return
	}

	uID := c.MustGet("currentUser").(models.User).ID
	rev := &models.ComponentRevision{
		ComponentID:   component.ID,
		EnvironmentID: env.ID,
		Values:        request.Values,
		CreatedBy:     uID,
	}
	if err := rev.CreateDraft(c.Request.Context(), a.MongoDB); err != nil {
		log.Printf("Error creating draft revision: %v\n", err)
		// Roll back identity to avoid orphan rows. The user will see the error
		// and the slate is clean for a retry.
		_ = component.Delete(a.MongoDB)
		conureerrors.AbortWithError(c, err)
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"component": component,
		"revision":  rev,
	})
}

// DeleteComponent wipes identity, every revision (draft + deployed history),
// and every Component CRD across all environments where the component has any
// revision. Idempotent against missing CRDs.
//
// Path: DELETE /:orgID/a/:appID/c/:componentID
func (a *ApiHandler) DeleteComponent(c *gin.Context) {
	handler, err := getHandlerFromRouteForWrite(c, a.MongoDB)
	if err != nil {
		conureerrors.AbortWithError(c, err)
		return
	}
	component, err := getComponentFromRoute(c, a.MongoDB)
	if err != nil {
		conureerrors.AbortWithError(c, err)
		return
	}
	if component.ApplicationID != handler.Model.ID {
		conureerrors.AbortWithError(c, conureerrors.ErrObjectNotFound)
		return
	}

	ctx := c.Request.Context()

	envIDs, err := models.EnvironmentsWithRevisions(ctx, a.MongoDB, component.ID)
	if err != nil {
		log.Printf("Error listing environments with revisions: %v\n", err)
		conureerrors.AbortWithError(c, err)
		return
	}
	envByID := map[string]models.Environment{}
	for _, env := range handler.Model.Environments {
		envByID[env.ID] = env
	}
	for _, envID := range envIDs {
		env, ok := envByID[envID]
		if !ok {
			// The env was removed from the application but revisions remained.
			// Skip K8s deletion — there's no namespace to target.
			continue
		}
		provider := newConureProvider(handler.Model, &env)
		if err := provider.DeleteComponentCRD(ctx, component.Name); err != nil {
			log.Printf("Error deleting component CRD in env %s: %v\n", env.Name, err)
			conureerrors.AbortWithError(c, err)
			return
		}
	}

	if _, err := models.DeleteAllRevisionsForComponent(ctx, a.MongoDB, component.ID); err != nil {
		log.Printf("Error deleting revisions: %v\n", err)
		conureerrors.AbortWithError(c, err)
		return
	}
	if err := component.Delete(a.MongoDB); err != nil {
		log.Printf("Error deleting component: %v\n", err)
		conureerrors.AbortWithError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// PromoteComponent copies the latest deployed revision in `from` to a new
// draft in `to`. Promote is the only supported way to seed a component into
// an environment that has no revisions yet.
//
// Path: POST /:orgID/a/:appID/c/:componentID/promote
func (a *ApiHandler) PromoteComponent(c *gin.Context) {
	handler, err := getHandlerFromRouteForWrite(c, a.MongoDB)
	if err != nil {
		conureerrors.AbortWithError(c, err)
		return
	}
	component, err := getComponentFromRoute(c, a.MongoDB)
	if err != nil {
		conureerrors.AbortWithError(c, err)
		return
	}
	if component.ApplicationID != handler.Model.ID {
		conureerrors.AbortWithError(c, conureerrors.ErrObjectNotFound)
		return
	}

	var req PromoteRequest
	if err := c.BindJSON(&req); err != nil {
		conureerrors.AbortWithError(c, conureerrors.ErrInvalidRequest)
		return
	}

	fromEnv, err := handler.Model.GetEnvironmentByName(a.MongoDB, req.From)
	if err != nil {
		conureerrors.AbortWithError(c, conureerrors.ErrObjectNotFound)
		return
	}
	toEnv, err := handler.Model.GetEnvironmentByName(a.MongoDB, req.To)
	if err != nil {
		conureerrors.AbortWithError(c, conureerrors.ErrObjectNotFound)
		return
	}

	ctx := c.Request.Context()
	source, err := models.LatestDeployed(ctx, a.MongoDB, component.ID, fromEnv.ID)
	if err != nil {
		if errors.Is(err, conureerrors.ErrObjectNotFound) {
			conureerrors.AbortWithError(c, conureerrors.ErrInvalidRequest)
			return
		}
		conureerrors.AbortWithError(c, err)
		return
	}

	uID := c.MustGet("currentUser").(models.User).ID
	rev := &models.ComponentRevision{
		ComponentID:   component.ID,
		EnvironmentID: toEnv.ID,
		Values:        source.Values,
		CreatedBy:     uID,
	}
	if err := rev.CreateDraft(ctx, a.MongoDB); err != nil {
		log.Printf("Error creating promoted draft: %v\n", err)
		conureerrors.AbortWithError(c, err)
		return
	}
	c.JSON(http.StatusCreated, rev)
}

// buildPresenceForComponent rolls up per-environment state for one component:
// deployed revision, draft revision, and live drift. Used by the app-wide
// list/detail handlers.
func buildPresenceForComponent(ctx context.Context, a *ApiHandler, component *models.Component, envs []models.Environment) ([]EnvironmentPresence, error) {
	out := make([]EnvironmentPresence, 0, len(envs))
	for _, env := range envs {
		presence := EnvironmentPresence{
			EnvironmentID:   env.ID,
			EnvironmentName: env.Name,
		}

		deployed, err := models.LatestDeployed(ctx, a.MongoDB, component.ID, env.ID)
		if err != nil && !errors.Is(err, conureerrors.ErrObjectNotFound) {
			return nil, err
		}
		if deployed != nil {
			presence.LatestDeployedVersion = deployed.Version
		}

		draft, err := models.LatestDraft(ctx, a.MongoDB, component.ID, env.ID)
		if err != nil && !errors.Is(err, conureerrors.ErrObjectNotFound) {
			return nil, err
		}
		if draft != nil {
			presence.HasDraft = true
			presence.LatestDraftVersion = draft.Version
		}

		live, err := liveComponentAndAbsorb(ctx, a, component, &env)
		if err != nil {
			return nil, err
		}
		if live != nil {
			presence.Active = true
			deployed, err = models.LatestDeployed(ctx, a.MongoDB, component.ID, env.ID)
			if err != nil && !errors.Is(err, conureerrors.ErrObjectNotFound) {
				return nil, err
			}
			if deployed != nil {
				presence.LatestDeployedVersion = deployed.Version
			}
		}

		out = append(out, presence)
	}
	return out, nil
}
