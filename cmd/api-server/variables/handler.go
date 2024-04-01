package variables

import (
	"encoding/hex"
	"github.com/coffeenights/conure/cmd/api-server/models"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"

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
	var variable models.Variable
	user := c.MustGet("currentUser").(models.User)

	client := user.Client
	organizationID, err := primitive.ObjectIDFromHex(c.Param("organizationID"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": ErrInvalidIDValue.Error()})
		return
	}

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
	var variable models.Variable
	user := c.MustGet("currentUser").(models.User)

	client := user.Client
	organizationID, err := primitive.ObjectIDFromHex(c.Param("organizationID"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": ErrInvalidIDValue.Error()})
		return
	}
	applicationID, err := primitive.ObjectIDFromHex(c.Param("applicationID"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": ErrInvalidIDValue.Error()})
		return
	}

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
	var variable models.Variable
	user := c.MustGet("currentUser").(models.User)

	client := user.Client
	organizationID, err := primitive.ObjectIDFromHex(c.Param("organizationID"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": ErrInvalidIDValue.Error()})
		return
	}
	applicationID, err := primitive.ObjectIDFromHex(c.Param("applicationID"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": ErrInvalidIDValue.Error()})
		return
	}
	componentID, err := primitive.ObjectIDFromHex(c.Param("componentID"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": ErrInvalidIDValue.Error()})
		return
	}
	environmentID := c.Param("environmentID")

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
	var variable models.Variable
	user := c.MustGet("currentUser").(models.User)
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
	orgID, err := primitive.ObjectIDFromHex(c.Param("organizationID"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": ErrInvalidIDValue.Error()})
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
			c.JSON(http.StatusBadRequest, gin.H{"error": ErrInvalidIDValue.Error()})
			return
		}
		variable.ComponentID = &compID
	}

	if !variable.Type.IsValid() {
		c.JSON(http.StatusBadRequest, gin.H{"error": ErrVariableTypeNotValid.Error()})
		return
	}

	appID := c.Param("applicationID")
	if appID != "" {
		appID, err := primitive.ObjectIDFromHex(c.Param("applicationID"))

		if err != nil {
			log.Print(err)
			c.JSON(http.StatusBadRequest, gin.H{"error": ErrInvalidIDValue.Error()})
			return
		}
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

func checkVariable(h *Handler, variable models.Variable) error {
	// When creating a new variable, the application ID is required for component and environment types
	if (variable.Type == models.ComponentType || variable.Type == models.EnvironmentType) && (variable.
		ApplicationID == nil || variable.EnvironmentID == nil) {
		return ErrVariableTypeRequiresApplicationID
	}
	// When creating a new variable, the componentID is required for component types
	if variable.Type == models.ComponentType && (variable.
		ApplicationID == nil || variable.EnvironmentID == nil || variable.ComponentID == nil) {
		return ErrVariableTypeRequiresComponentID
	}

	variableDB := models.Variable{}
	if variable.Type == models.OrganizationType {
		err := variableDB.GetByOrgAndName(h.MongoDB, variable.Client, variable.OrganizationID, variable.Name)
		if err == nil {
			return ErrVariableAlreadyExists
		}
	}
	if variable.Type == models.EnvironmentType {
		err := variableDB.GetByAppIDAndEnvAndName(h.MongoDB, variable.Client, *variable.ApplicationID, models.EnvironmentType,
			variable.EnvironmentID, variable.Name)
		if err == nil {
			return ErrVariableAlreadyExists
		}
	}
	if variable.Type == models.ComponentType {
		err := variableDB.GetByAppIDAndEnvAndCompAndName(h.MongoDB, variable.Client, *variable.ApplicationID,
			models.ComponentType, variable.EnvironmentID, variable.ComponentID, variable.Name)
		if err == nil {
			return ErrVariableAlreadyExists
		}
	}
	return nil
}

func encryptValue(storage SecretKeyStorage, value string) string {
	key, err := storage.Load()
	if err != nil {
		log.Panic(err)
	}

	encryptedValue := encrypt(value, hex.EncodeToString(key))
	if err != nil {
		log.Panic(err)
	}

	return encryptedValue
}

func decryptValue(storage SecretKeyStorage, value string) string {
	key, err := storage.Load()
	if err != nil {
		log.Panic(err)
	}

	decryptedValue := decrypt(value, hex.EncodeToString(key))
	if err != nil {
		log.Panic(err)
	}

	return decryptedValue
}
