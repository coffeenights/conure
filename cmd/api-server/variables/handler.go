package variables

import (
	"encoding/hex"
	"fmt"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"

	apiConfig "github.com/coffeenights/conure/cmd/api-server/config"
	"github.com/coffeenights/conure/cmd/api-server/conureerrors"
	"github.com/coffeenights/conure/cmd/api-server/database"
	"github.com/coffeenights/conure/cmd/api-server/middlewares"
	"github.com/coffeenights/conure/cmd/api-server/models"
)

type Handler struct {
	Config     *apiConfig.Config
	MongoDB    *database.MongoDB
	KeyStorage SecretKeyStorage
}

func NewVariablesHandler(config *apiConfig.Config, mongo *database.MongoDB, keyStorage SecretKeyStorage) *Handler {
	return &Handler{
		Config:     config,
		MongoDB:    mongo,
		KeyStorage: keyStorage,
	}
}

// requireOrgOwnership parses :organizationID, loads the org, and verifies the
// authenticated user owns it. On failure it writes the response and returns
// ok=false; callers must return immediately.
func (h *Handler) requireOrgOwnership(c *gin.Context) (primitive.ObjectID, bool) {
	orgID, err := primitive.ObjectIDFromHex(c.Param("organizationID"))
	if err != nil {
		conureerrors.AbortWithError(c, conureerrors.ErrInvalidRequest)
		return primitive.NilObjectID, false
	}
	user := c.MustGet("currentUser").(models.User)
	org := models.Organization{}
	if _, err := org.GetById(h.MongoDB, orgID.Hex()); err != nil {
		conureerrors.AbortWithError(c, err)
		return primitive.NilObjectID, false
	}
	if !middlewares.CanWriteOrg(user, &org) {
		conureerrors.AbortWithError(c, conureerrors.ErrNotAllowed)
		return primitive.NilObjectID, false
	}
	return orgID, true
}

func (h *Handler) ListOrganizationVariables(c *gin.Context) {
	var variable models.Variable

	organizationID, ok := h.requireOrgOwnership(c)
	if !ok {
		return
	}

	variables, err := variable.ListByOrg(h.MongoDB, organizationID)
	if err != nil {
		conureerrors.AbortWithError(c, conureerrors.ErrInvalidRequest)
		return
	}
	// Decrypt the values of the variables
	for i, v := range variables {
		if v.IsEncrypted {
			decrypted, err := DecryptValue(h.KeyStorage, v.Value)
			if err != nil {
				conureerrors.AbortWithError(c, fmt.Errorf("decrypting variable %q: %w", v.Name, err))
				return
			}
			variables[i].Value = decrypted
		}
	}

	c.JSON(http.StatusOK, variables)
}

func (h *Handler) ListEnvironmentVariables(c *gin.Context) {
	var variable models.Variable

	organizationID, ok := h.requireOrgOwnership(c)
	if !ok {
		return
	}
	applicationID, err := primitive.ObjectIDFromHex(c.Param("applicationID"))
	if err != nil {
		conureerrors.AbortWithError(c, conureerrors.ErrInvalidRequest)
		return
	}

	environmentID := c.Param("environmentID")

	variables, err := variable.ListByEnv(h.MongoDB, organizationID, applicationID, environmentID)
	if err != nil {
		conureerrors.AbortWithError(c, conureerrors.ErrInvalidRequest)
		return
	}
	// Decrypt the values of the variables
	for i, v := range variables {
		if v.IsEncrypted {
			decrypted, err := DecryptValue(h.KeyStorage, v.Value)
			if err != nil {
				conureerrors.AbortWithError(c, fmt.Errorf("decrypting variable %q: %w", v.Name, err))
				return
			}
			variables[i].Value = decrypted
		}
	}

	c.JSON(http.StatusOK, variables)
}

func (h *Handler) ListComponentVariables(c *gin.Context) {
	var variable models.Variable

	organizationID, ok := h.requireOrgOwnership(c)
	if !ok {
		return
	}
	applicationID, err := primitive.ObjectIDFromHex(c.Param("applicationID"))
	if err != nil {
		conureerrors.AbortWithError(c, conureerrors.ErrInvalidRequest)
		return
	}
	componentID, err := primitive.ObjectIDFromHex(c.Param("componentID"))
	if err != nil {
		conureerrors.AbortWithError(c, conureerrors.ErrInvalidRequest)
		return
	}
	environmentID := c.Param("environmentID")

	variables, err := variable.ListByComp(h.MongoDB, organizationID, applicationID, environmentID, componentID)
	if err != nil {
		conureerrors.AbortWithError(c, conureerrors.ErrInvalidRequest)
		return
	}
	// Decrypt the values of the variables
	for i, v := range variables {
		if v.IsEncrypted {
			decrypted, err := DecryptValue(h.KeyStorage, v.Value)
			if err != nil {
				conureerrors.AbortWithError(c, fmt.Errorf("decrypting variable %q: %w", v.Name, err))
				return
			}
			variables[i].Value = decrypted
		}
	}

	c.JSON(http.StatusOK, variables)
}

// ListEnvironmentVariablesAllScopes returns the merged set of org- and
// env-tier variables that would be effective for a component in this
// environment. Component-tier variables are not included here — pick the
// component-allscopes endpoint when a specific component is in play.
func (h *Handler) ListEnvironmentVariablesAllScopes(c *gin.Context) {
	var variable models.Variable

	organizationID, ok := h.requireOrgOwnership(c)
	if !ok {
		return
	}
	applicationID, err := primitive.ObjectIDFromHex(c.Param("applicationID"))
	if err != nil {
		conureerrors.AbortWithError(c, conureerrors.ErrInvalidRequest)
		return
	}
	environmentID := c.Param("environmentID")

	orgVars, err := variable.ListByOrg(h.MongoDB, organizationID)
	if err != nil {
		conureerrors.AbortWithError(c, conureerrors.ErrInvalidRequest)
		return
	}
	envVars, err := variable.ListByEnv(h.MongoDB, organizationID, applicationID, environmentID)
	if err != nil {
		conureerrors.AbortWithError(c, conureerrors.ErrInvalidRequest)
		return
	}

	merged := MergeAllScopes(orgVars, envVars, nil)
	for i, v := range merged {
		if v.IsEncrypted {
			decrypted, err := DecryptValue(h.KeyStorage, v.Value)
			if err != nil {
				conureerrors.AbortWithError(c, fmt.Errorf("decrypting variable %q: %w", v.Name, err))
				return
			}
			merged[i].Value = decrypted
		}
	}

	c.JSON(http.StatusOK, merged)
}

// ListComponentVariablesAllScopes returns the merged set of org-, env-, and
// component-tier variables that would be delivered to a component at render
// time. The Type field on each entry identifies which tier the winning value
// came from.
func (h *Handler) ListComponentVariablesAllScopes(c *gin.Context) {
	var variable models.Variable

	organizationID, ok := h.requireOrgOwnership(c)
	if !ok {
		return
	}
	applicationID, err := primitive.ObjectIDFromHex(c.Param("applicationID"))
	if err != nil {
		conureerrors.AbortWithError(c, conureerrors.ErrInvalidRequest)
		return
	}
	componentID, err := primitive.ObjectIDFromHex(c.Param("componentID"))
	if err != nil {
		conureerrors.AbortWithError(c, conureerrors.ErrInvalidRequest)
		return
	}
	environmentID := c.Param("environmentID")

	orgVars, err := variable.ListByOrg(h.MongoDB, organizationID)
	if err != nil {
		conureerrors.AbortWithError(c, conureerrors.ErrInvalidRequest)
		return
	}
	envVars, err := variable.ListByEnv(h.MongoDB, organizationID, applicationID, environmentID)
	if err != nil {
		conureerrors.AbortWithError(c, conureerrors.ErrInvalidRequest)
		return
	}
	compVars, err := variable.ListByComp(h.MongoDB, organizationID, applicationID, environmentID, componentID)
	if err != nil {
		conureerrors.AbortWithError(c, conureerrors.ErrInvalidRequest)
		return
	}

	merged := MergeAllScopes(orgVars, envVars, compVars)
	for i, v := range merged {
		if v.IsEncrypted {
			decrypted, err := DecryptValue(h.KeyStorage, v.Value)
			if err != nil {
				conureerrors.AbortWithError(c, fmt.Errorf("decrypting variable %q: %w", v.Name, err))
				return
			}
			merged[i].Value = decrypted
		}
	}

	c.JSON(http.StatusOK, merged)
}

func (h *Handler) CreateVariable(c *gin.Context) {
	var variable models.Variable

	if err := c.ShouldBindJSON(&variable); err != nil {
		conureerrors.AbortWithError(c, conureerrors.ErrInvalidRequest)
		return
	}

	if !variable.ValidateName() {
		conureerrors.AbortWithError(c, conureerrors.ErrInvalidRequest)
		return
	}

	orgID, ok := h.requireOrgOwnership(c)
	if !ok {
		return
	}
	variable.OrganizationID = orgID
	variable.Type = models.OrganizationType

	envID := c.Param("environmentID")
	if envID != "" {
		variable.Type = models.EnvironmentType
		variable.EnvironmentID = &envID
	}

	compID := c.Param("componentID")
	if compID != "" {
		variable.Type = models.ComponentType
		compID, err := primitive.ObjectIDFromHex(c.Param("componentID"))
		if err != nil {
			conureerrors.AbortWithError(c, conureerrors.ErrInvalidRequest)
			return
		}
		variable.ComponentID = &compID
	}

	if !variable.Type.IsValid() {
		conureerrors.AbortWithError(c, conureerrors.ErrInvalidRequest)
		return
	}

	appID := c.Param("applicationID")
	if appID != "" {
		appID, err := primitive.ObjectIDFromHex(c.Param("applicationID"))

		if err != nil {
			conureerrors.AbortWithError(c, conureerrors.ErrInvalidRequest)
			return
		}
		variable.ApplicationID = &appID
	}

	if err := checkVariable(h, variable); err != nil {
		conureerrors.AbortWithError(c, conureerrors.ErrInvalidRequest)
		return
	}

	if variable.IsEncrypted {
		encrypted, err := EncryptValue(h.KeyStorage, variable.Value)
		if err != nil {
			conureerrors.AbortWithError(c, fmt.Errorf("encrypting variable: %w", err))
			return
		}
		variable.Value = encrypted
	}

	// save the variable to the database
	_, err := variable.Create(h.MongoDB)
	if err != nil {
		conureerrors.AbortWithError(c, err)
		return
	}

	c.JSON(http.StatusCreated, variable)
}

func (h *Handler) DeleteVariable(c *gin.Context) {
	var variable models.Variable

	orgID, ok := h.requireOrgOwnership(c)
	if !ok {
		return
	}

	varID, err := primitive.ObjectIDFromHex(c.Param("variableID"))
	if err != nil {
		conureerrors.AbortWithError(c, conureerrors.ErrInvalidRequest)
		return
	}

	// Confirm the variable actually belongs to the org in the URL. Without
	// this, an owner of OrgA could delete a variable belonging to OrgB by
	// pairing their own org ID with another org's variable ID.
	if err := variable.GetByID(h.MongoDB, varID.Hex()); err != nil {
		conureerrors.AbortWithError(c, err)
		return
	}
	if variable.OrganizationID != orgID {
		conureerrors.AbortWithError(c, conureerrors.ErrObjectNotFound)
		return
	}

	err = variable.Delete(h.MongoDB)
	if err != nil {
		log.Printf("Error deleting variable: %v", err)
		conureerrors.AbortWithError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

func checkVariable(h *Handler, variable models.Variable) error {
	// When creating a new variable, the application ID is required for component and environment types
	if (variable.Type == models.ComponentType || variable.Type == models.EnvironmentType) && (variable.
		ApplicationID == nil || variable.EnvironmentID == nil) {
		return conureerrors.ErrInvalidRequest
	}
	// When creating a new variable, the componentID is required for component types
	if variable.Type == models.ComponentType && (variable.
		ApplicationID == nil || variable.EnvironmentID == nil || variable.ComponentID == nil) {
		return conureerrors.ErrInvalidRequest
	}

	variableDB := models.Variable{}
	if variable.Type == models.OrganizationType {
		err := variableDB.GetByOrgAndName(h.MongoDB, variable.OrganizationID, variable.Name)
		if err == nil {
			return conureerrors.ErrObjectAlreadyExists
		}
	}
	if variable.Type == models.EnvironmentType {
		err := variableDB.GetByAppIDAndEnvAndName(h.MongoDB, *variable.ApplicationID, models.EnvironmentType,
			variable.EnvironmentID, variable.Name)
		if err == nil {
			return conureerrors.ErrObjectAlreadyExists
		}
	}
	if variable.Type == models.ComponentType {
		err := variableDB.GetByAppIDAndEnvAndCompAndName(h.MongoDB, *variable.ApplicationID,
			models.ComponentType, variable.EnvironmentID, variable.ComponentID, variable.Name)
		if err == nil {
			return conureerrors.ErrObjectAlreadyExists
		}
	}
	return nil
}

func EncryptValue(storage SecretKeyStorage, value string) (string, error) {
	key, err := storage.Load()
	if err != nil {
		return "", fmt.Errorf("loading key: %w", err)
	}
	return encrypt(value, hex.EncodeToString(key))
}

func DecryptValue(storage SecretKeyStorage, value string) (string, error) {
	key, err := storage.Load()
	if err != nil {
		return "", fmt.Errorf("loading key: %w", err)
	}
	return decrypt(value, hex.EncodeToString(key))
}
