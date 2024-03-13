package variables

import (
	"encoding/hex"
	"net/http"

	"github.com/coffeenights/conure/cmd/api-server/auth"
	"github.com/gin-gonic/gin"

	apiConfig "github.com/coffeenights/conure/cmd/api-server/config"
	"github.com/coffeenights/conure/cmd/api-server/database"
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
	var variable Variable
	user := c.MustGet("currentUser").(auth.User)

	client := user.Client
	organizationID := c.Param("organizationID")

	variables, err := variable.ListByOrg(h.MongoDB, client, organizationID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	// Decrypt the values of the variables
	for i, v := range variables {
		if v.IsEncrypted {
			variables[i].Value = decryptValue(h.KeyStorage, v.Value)
		}
	}

	c.JSON(http.StatusOK, variables)
}

func (h *Handler) ListEnvironmentVariables(c *gin.Context) {
	var variable Variable
	user := c.MustGet("currentUser").(auth.User)

	client := user.Client
	organizationID := c.Param("organizationID")
	applicationID := c.Param("applicationID")
	environmentID := c.Param("environmentID")

	variables, err := variable.ListByEnv(h.MongoDB, client, organizationID, applicationID, environmentID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	// Decrypt the values of the variables
	for i, v := range variables {
		if v.IsEncrypted {
			variables[i].Value = decryptValue(h.KeyStorage, v.Value)
		}
	}

	c.JSON(http.StatusOK, variables)
}

func (h *Handler) ListComponentVariables(c *gin.Context) {
	var variable Variable
	user := c.MustGet("currentUser").(auth.User)

	client := user.Client
	organizationID := c.Param("organizationID")
	applicationID := c.Param("applicationID")
	environmentID := c.Param("environmentID")
	componentID := c.Param("componentID")

	variables, err := variable.ListByComp(h.MongoDB, client, organizationID, applicationID, environmentID, componentID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	// Decrypt the values of the variables
	for i, v := range variables {
		if v.IsEncrypted {
			variables[i].Value = decryptValue(h.KeyStorage, v.Value)
		}
	}

	c.JSON(http.StatusOK, variables)
}

func (h *Handler) CreateVariable(c *gin.Context) {
	var variable Variable
	user := c.MustGet("currentUser").(auth.User)
	client := user.Client

	if err := c.ShouldBindJSON(&variable); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if !variable.ValidateName() {
		c.JSON(http.StatusBadRequest, gin.H{"error": ErrVariableNameNotValid.Error()})
		return
	}

	variable.Client = client
	variable.OrganizationID = c.Param("organizationID")
	variable.Type = OrganizationType

	envID := c.Param("environmentID")
	if envID != "" {
		variable.Type = EnvironmentType
		variable.EnvironmentID = &envID
	}

	componentID := c.Param("componentID")
	if componentID != "" {
		variable.Type = ComponentType
		variable.ComponentID = &componentID
	}

	if !variable.Type.IsValid() {
		c.JSON(http.StatusBadRequest, gin.H{"error": ErrVariableTypeNotValid.Error()})
		return
	}

	appID := c.Param("applicationID")
	if appID != "" {
		variable.ApplicationID = &appID
	}

	if err := checkVariable(h, variable); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if variable.IsEncrypted {
		variable.Value = encryptValue(h.KeyStorage, variable.Value)
	}

	// save the variable to the database
	_, _ = variable.Create(h.MongoDB)

	c.JSON(http.StatusCreated, variable)
}

func checkVariable(h *Handler, variable Variable) error {
	// When creating a new variable, the application ID is required for component and environment types
	if (variable.Type == ComponentType || variable.Type == EnvironmentType) && (variable.
		ApplicationID == nil || variable.EnvironmentID == nil) {
		return ErrVariableTypeRequiresApplicationID
	}
	// When creating a new variable, the componentID is required for component types
	if variable.Type == ComponentType && (variable.
		ApplicationID == nil || variable.EnvironmentID == nil || variable.ComponentID == nil) {
		return ErrVariableTypeRequiresComponentID
	}

	variableDB := Variable{}
	if variable.Type == OrganizationType {
		err := variableDB.GetByOrgAndName(h.MongoDB, variable.Client, variable.OrganizationID, variable.Name)
		if err == nil {
			return ErrVariableAlreadyExists
		}
	}
	if variable.Type == EnvironmentType {
		err := variableDB.GetByAppIDAndEnvAndName(h.MongoDB, variable.Client, *variable.ApplicationID, EnvironmentType,
			variable.EnvironmentID, variable.Name)
		if err == nil {
			return ErrVariableAlreadyExists
		}
	}
	if variable.Type == ComponentType {
		err := variableDB.GetByAppIDAndEnvAndCompAndName(h.MongoDB, variable.Client, *variable.ApplicationID,
			ComponentType, variable.EnvironmentID, variable.ComponentID, variable.Name)
		if err == nil {
			return ErrVariableAlreadyExists
		}
	}
	return nil
}

func encryptValue(storage SecretKeyStorage, value string) string {
	key, err := storage.Load()
	if err != nil {
		panic(err)
	}

	encryptedValue := encrypt(value, hex.EncodeToString(key))
	if err != nil {
		panic(err)
	}

	return encryptedValue
}

func decryptValue(storage SecretKeyStorage, value string) string {
	key, err := storage.Load()
	if err != nil {
		panic(err)
	}

	decryptedValue := decrypt(value, hex.EncodeToString(key))
	if err != nil {
		panic(err)
	}

	return decryptedValue
}
