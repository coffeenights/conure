package applications

import (
	"log"

	"github.com/coffeenights/conure/cmd/api-server/conureerrors"
	"github.com/coffeenights/conure/cmd/api-server/database"
	"github.com/coffeenights/conure/cmd/api-server/middlewares"
	"github.com/coffeenights/conure/cmd/api-server/models"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type ApplicationHandler struct {
	ID             string
	OrganizationID string
	Model          *models.Application
	DB             *database.MongoDB
}

func NewApplicationHandler(db *database.MongoDB) (*ApplicationHandler, error) {
	return &ApplicationHandler{
		Model: &models.Application{},
		DB:    db,
	}, nil
}

func ListOrganizationApplications(organizationID string, db *database.MongoDB) ([]*ApplicationHandler, error) {
	apps, err := models.ApplicationList(db, organizationID)
	if err != nil {
		return nil, err
	}
	handlers := make([]*ApplicationHandler, len(apps))
	for i, app := range apps {
		handler, err := NewApplicationHandler(db)
		if err != nil {
			return nil, err
		}
		handler.Model = app
		handlers[i] = handler
	}
	return handlers, nil
}

func (ah *ApplicationHandler) GetApplicationByID(appID string) error {
	return ah.Model.GetByID(ah.DB, appID)
}

// getHandlerFromRoute resolves the application referenced by the route
// params and checks that the caller may read it. Read access is granted to
// admins, the app's creator, and any developer whose home organization
// matches the app's organization.
func getHandlerFromRoute(c *gin.Context, db *database.MongoDB) (*ApplicationHandler, error) {
	handler, _, err := resolveAppFromRoute(c, db)
	if err != nil {
		return nil, err
	}
	return handler, nil
}

// getHandlerFromRouteForWrite layers a write-tier check on top of the read
// resolution: developers can only mutate apps they personally created,
// admins can mutate anything.
func getHandlerFromRouteForWrite(c *gin.Context, db *database.MongoDB) (*ApplicationHandler, error) {
	handler, _, err := resolveAppFromRoute(c, db)
	if err != nil {
		return nil, err
	}
	if !middlewares.CanWriteOwned(c.MustGet("currentUser").(models.User), handler.Model.AccountID) {
		return nil, conureerrors.ErrNotAllowed
	}
	return handler, nil
}

// abortIfCannotWriteApp emits a 403 and returns true when the current
// user is not allowed to mutate the resolved application. Use after a read
// resolution (loadComponentEnv / getHandlerFromRoute) on write paths.
//
// Callers must pass a non-nil handler — every existing caller does, because
// the loader they use aborts before this is reached. We intentionally do
// not nil-check; a nil here is a caller bug and we want it to panic with a
// real stack trace rather than silently allowing the write to proceed.
func abortIfCannotWriteApp(c *gin.Context, handler *ApplicationHandler) bool {
	if !middlewares.CanWriteOwned(c.MustGet("currentUser").(models.User), handler.Model.AccountID) {
		conureerrors.AbortWithError(c, conureerrors.ErrNotAllowed)
		return true
	}
	return false
}

func resolveAppFromRoute(c *gin.Context, db *database.MongoDB) (*ApplicationHandler, *models.Organization, error) {
	// Translate raw "not a valid hex" errors from primitive.ObjectIDFromHex
	// into ErrInvalidRequest. Otherwise AbortWithError (and any upstream
	// caller) treats them as unknown and falls back to HTTP 500 — a bad
	// ID in the URL is a client problem, not a server problem.
	if _, err := primitive.ObjectIDFromHex(c.Param("organizationID")); err != nil {
		log.Printf("Error parsing organizationID: %v\n", err)
		return nil, nil, conureerrors.ErrInvalidRequest
	}
	if _, err := primitive.ObjectIDFromHex(c.Param("applicationID")); err != nil {
		log.Printf("Error parsing applicationID: %v\n", err)
		return nil, nil, conureerrors.ErrInvalidRequest
	}

	handler, err := NewApplicationHandler(db)
	if err != nil {
		log.Printf("Error creating application handler: %v\n", err)
		return nil, nil, err
	}
	if err = handler.GetApplicationByID(c.Param("applicationID")); err != nil {
		log.Printf("Error getting application: %v\n", err)
		return nil, nil, conureerrors.ErrObjectNotFound
	}
	org := models.Organization{}
	if _, err := org.GetById(db, handler.Model.OrganizationID.Hex()); err != nil {
		return nil, nil, conureerrors.ErrObjectNotFound
	}
	if !middlewares.CanReadOrg(c.MustGet("currentUser").(models.User), &org) {
		return nil, nil, conureerrors.ErrNotAllowed
	}
	return handler, &org, nil
}
