package variables

import (
	"encoding/hex"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"

	apiConfig "github.com/coffeenights/conure/cmd/api-server/config"
	"github.com/coffeenights/conure/cmd/api-server/conureerrors"
	"github.com/coffeenights/conure/cmd/api-server/database"
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

func (h *Handler) ListOrganizationVariables(c *gin.Context) {
	var variable models.Variable

	organizationID, err := primitive.ObjectIDFromHex(c.Param("organizationID"))
	if err != nil {
		conureerrors.AbortWithError(c, conureerrors.ErrInvalidRequest)
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
			variables[i].Value = DecryptValue(h.KeyStorage, v.Value)
		}
	}

	c.JSON(http.StatusOK, variables)
}

func (h *Handler) ListEnvironmentVariables(c *gin.Context) {
	var variable models.Variable

	organizationID, err := primitive.ObjectIDFromHex(c.Param("organizationID"))
	if err != nil {
		conureerrors.AbortWithError(c, conureerrors.ErrInvalidRequest)
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
			variables[i].Value = DecryptValue(h.KeyStorage, v.Value)
		}
	}

	c.JSON(http.StatusOK, variables)
}

func (h *Handler) ListComponentVariables(c *gin.Context) {
	var variable models.Variable

	organizationID, err := primitive.ObjectIDFromHex(c.Param("organizationID"))
	if err != nil {
		conureerrors.AbortWithError(c, conureerrors.ErrInvalidRequest)
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
			variables[i].Value = DecryptValue(h.KeyStorage, v.Value)
		}
	}

	c.JSON(http.StatusOK, variables)
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

	orgID, err := primitive.ObjectIDFromHex(c.Param("organizationID"))
	if err != nil {
		conureerrors.AbortWithError(c, conureerrors.ErrInvalidRequest)
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
		variable.Value = EncryptValue(h.KeyStorage, variable.Value)
	}

	// save the variable to the database
	_, err = variable.Create(h.MongoDB)
	if err != nil {
		conureerrors.AbortWithError(c, err)
		return
	}

	c.JSON(http.StatusCreated, variable)
}

func (h *Handler) DeleteVariable(c *gin.Context) {
	var variable models.Variable
	user := c.MustGet("currentUser").(models.User)

	orgID, err := primitive.ObjectIDFromHex(c.Param("organizationID"))
	if err != nil {
		conureerrors.AbortWithError(c, conureerrors.ErrInvalidRequest)
		return
	}

	varID, err := primitive.ObjectIDFromHex(c.Param("variableID"))
	if err != nil {
		conureerrors.AbortWithError(c, conureerrors.ErrInvalidRequest)
		return
	}

	org := models.Organization{}
	_, err = org.GetById(h.MongoDB, orgID.Hex())
	if err != nil {
		log.Printf("Error getting organization: %v", err)
		conureerrors.AbortWithError(c, err)
		return
	}

	if org.AccountID != user.ID {
		conureerrors.AbortWithError(c, conureerrors.ErrNotAllowed)
		return
	}

	variable.ID = varID
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

func EncryptValue(storage SecretKeyStorage, value string) string {
	key, err := storage.Load()
	if err != nil {
		log.Panic(err)
	}

	encryptedValue := encrypt(value, hex.EncodeToString(key))
	return encryptedValue
}

func DecryptValue(storage SecretKeyStorage, value string) string {
	key, err := storage.Load()
	if err != nil {
		log.Panic(err)
	}

	decryptedValue := decrypt(value, hex.EncodeToString(key))

	return decryptedValue
}
