package variables

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"github.com/coffeenights/conure/cmd/api-server/auth"
	apiConfig "github.com/coffeenights/conure/cmd/api-server/config"
	"github.com/coffeenights/conure/cmd/api-server/database"
)

func cleanUpDB(mongo *database.MongoDB) {
	err := mongo.Client.Database(mongo.DBName).Drop(context.Background())
	if err != nil {
		panic(err)
	}
}
func setupTestHandler(router *gin.Engine, mongo *database.MongoDB, conf *apiConfig.Config, keyStorage SecretKeyStorage) {

	handler := NewVariablesHandler(conf, mongo, keyStorage)
	GenerateRoutes("/variables", router, handler)
}

func TestHandler_ListOrganizationVariables(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	payload := auth.JWTData{
		Email:  "test@test.com",
		Client: "test-client",
	}

	token, _ := auth.GenerateToken(1*time.Hour, payload, "test-secret")

	config := &apiConfig.Config{
		JWTSecret:          "test-secret",
		MongoDBURI:         "mongodb://localhost:27017",
		MongoDBName:        "conure-test",
		AuthStrategySystem: "local",
		AESStorageStrategy: "local",
	}
	mongo, _ := database.ConnectToMongoDB(config.MongoDBURI, config.MongoDBName)
	defer cleanUpDB(mongo)
	keyStorage := NewLocalSecretKey("secret.key")
	secretKeyBytes, _ := keyStorage.Load()
	_ = hex.EncodeToString(secretKeyBytes)

	user := auth.User{
		Email:  "test@test.com",
		Client: "test-client",
	}
	_ = user.Create(mongo)

	orgVar := &Variable{
		OrganizationID: "org1",
		Name:           "var1",
		Value:          "value1",
		IsEncrypted:    false,
		Client:         "test-client",
		Type:           OrganizationType,
	}
	_, _ = orgVar.Create(mongo)

	orgVar2 := &Variable{
		OrganizationID: "org1",
		Name:           "var2",
		Value:          encryptValue(keyStorage, "value2"),
		IsEncrypted:    true,
		Client:         "test-client",
		Type:           OrganizationType,
	}
	_, _ = orgVar2.Create(mongo)

	setupTestHandler(router, mongo, config, keyStorage)
	var variables []Variable

	req, _ := http.NewRequest("GET", "/variables/org1", nil)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	_ = json.Unmarshal(resp.Body.Bytes(), &variables)

	assert.Equal(t, http.StatusOK, resp.Code, "should return 200 OK")
	assert.Equal(t, 2, len(variables), "should return 2 results")
	assert.Equal(t, orgVar.OrganizationID, variables[0].OrganizationID, "should return the correct organization")
	assert.Equal(t, orgVar.Type, variables[0].Type, "should return the correct type of variable")
	assert.True(t, variables[1].IsEncrypted, "should return the correct type of variable")

	req, _ = http.NewRequest("GET", "/variables/fakeOrg", nil)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	_ = json.Unmarshal(resp.Body.Bytes(), &variables)

	assert.Equal(t, http.StatusOK, resp.Code, "should return 200 OK")
	assert.Equal(t, 0, len(variables), "should return 0 results")

	req, _ = http.NewRequest("GET", "/variables/fakeOrg", nil)
	req.Header.Set("Content-Type", "application/json")

	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	_ = json.Unmarshal(resp.Body.Bytes(), &variables)

	assert.Equal(t, http.StatusUnauthorized, resp.Code, "should return 401 Unauthorized")
}

func TestHandler_ListEnvironmentVariables(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	payload := auth.JWTData{
		Email:  "test@test.com",
		Client: "test-client",
	}

	token, _ := auth.GenerateToken(1*time.Hour, payload, "test-secret")

	config := &apiConfig.Config{
		JWTSecret:          "test-secret",
		MongoDBURI:         "mongodb://localhost:27017",
		MongoDBName:        "conure-test",
		AuthStrategySystem: "local",
	}
	mongo, _ := database.ConnectToMongoDB(config.MongoDBURI, config.MongoDBName)
	defer cleanUpDB(mongo)

	keyStorage := NewLocalSecretKey("secret.key")
	secretKeyBytes, _ := keyStorage.Load()
	_ = hex.EncodeToString(secretKeyBytes)

	user := auth.User{
		Email:  "test@test.com",
		Client: "test-client",
	}
	_ = user.Create(mongo)

	app1 := "app1"
	env1 := "env1"
	orgVar := &Variable{
		OrganizationID: "org1",
		EnvironmentID:  &env1,
		ApplicationID:  &app1,
		Name:           "var1",
		Value:          "value1",
		IsEncrypted:    false,
		Client:         "test-client",
		Type:           EnvironmentType,
	}
	_, _ = orgVar.Create(mongo)

	orgVar2 := &Variable{
		OrganizationID: "org1",
		EnvironmentID:  &env1,
		ApplicationID:  &app1,
		Name:           "var2",
		Value:          encryptValue(keyStorage, "value2"),
		IsEncrypted:    true,
		Client:         "test-client",
		Type:           EnvironmentType,
	}
	_, _ = orgVar2.Create(mongo)

	setupTestHandler(router, mongo, config, keyStorage)
	var variables []Variable

	urlFormat := "/variables/org1/%s/e/%s"
	url := fmt.Sprintf(urlFormat, app1, env1)
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	_ = json.Unmarshal(resp.Body.Bytes(), &variables)

	assert.Equal(t, http.StatusOK, resp.Code, "should return 200 OK")
	assert.Equal(t, 2, len(variables), "should return 2 results")
	assert.Equal(t, orgVar.OrganizationID, variables[0].OrganizationID, "should return the correct organization")
	assert.Equal(t, orgVar.Type, variables[0].Type, "should return the correct type of variable")
	assert.True(t, variables[1].IsEncrypted, "should return the correct type of variable")

	fakeURL := fmt.Sprintf(urlFormat, app1, "fakeEnv")
	req, _ = http.NewRequest("GET", fakeURL, nil)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	_ = json.Unmarshal(resp.Body.Bytes(), &variables)

	assert.Equal(t, http.StatusOK, resp.Code, "should return 200 OK")
	assert.Equal(t, 0, len(variables), "should return 0 results")

	req, _ = http.NewRequest("GET", fakeURL, nil)
	req.Header.Set("Content-Type", "application/json")

	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	_ = json.Unmarshal(resp.Body.Bytes(), &variables)

	assert.Equal(t, http.StatusUnauthorized, resp.Code, "should return 401 Unauthorized")

	fakeURL = fmt.Sprintf(urlFormat, "fakeApp", env1)
	req, _ = http.NewRequest("GET", fakeURL, nil)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	_ = json.Unmarshal(resp.Body.Bytes(), &variables)

	assert.Equal(t, http.StatusOK, resp.Code, "should return 200 OK")
	assert.Equal(t, 0, len(variables), "should return 0 results")
}

func TestHandler_ListComponentVariables(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	payload := auth.JWTData{
		Email:  "test@test.com",
		Client: "test-client",
	}

	token, _ := auth.GenerateToken(1*time.Hour, payload, "test-secret")

	config := &apiConfig.Config{
		JWTSecret:          "test-secret",
		MongoDBURI:         "mongodb://localhost:27017",
		MongoDBName:        "conure-test",
		AuthStrategySystem: "local",
	}
	mongo, _ := database.ConnectToMongoDB(config.MongoDBURI, config.MongoDBName)
	defer cleanUpDB(mongo)

	keyStorage := NewLocalSecretKey("secret.key")
	secretKeyBytes, _ := keyStorage.Load()
	_ = hex.EncodeToString(secretKeyBytes)

	user := auth.User{
		Email:  "test@test.com",
		Client: "test-client",
	}
	_ = user.Create(mongo)

	app1 := "app1"
	env1 := "env1"
	comp1 := "comp1"
	orgVar := &Variable{
		OrganizationID: "org1",
		EnvironmentID:  &env1,
		ApplicationID:  &app1,
		ComponentID:    &comp1,
		Name:           "var1",
		Value:          "value1",
		IsEncrypted:    false,
		Client:         "test-client",
		Type:           ComponentType,
	}
	_, _ = orgVar.Create(mongo)

	orgVar2 := &Variable{
		OrganizationID: "org1",
		EnvironmentID:  &env1,
		ApplicationID:  &app1,
		ComponentID:    &comp1,
		Name:           "var2",
		Value:          encryptValue(keyStorage, "value2"),
		IsEncrypted:    true,
		Client:         "test-client",
		Type:           ComponentType,
	}
	_, _ = orgVar2.Create(mongo)

	setupTestHandler(router, mongo, config, keyStorage)
	var variables []Variable

	urlFormat := "/variables/org1/%s/e/%s/c/%s"
	url := fmt.Sprintf(urlFormat, app1, env1, comp1)
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	_ = json.Unmarshal(resp.Body.Bytes(), &variables)

	assert.Equal(t, http.StatusOK, resp.Code, "should return 200 OK")
	assert.Equal(t, 2, len(variables), "should return 2 results")
	assert.Equal(t, orgVar.OrganizationID, variables[0].OrganizationID, "should return the correct organization")
	assert.Equal(t, orgVar.Type, variables[0].Type, "should return the correct type of variable")
	assert.True(t, variables[1].IsEncrypted, "should return the correct type of variable")

	fakeURL := fmt.Sprintf(urlFormat, app1, env1, "fakeComp")
	req, _ = http.NewRequest("GET", fakeURL, nil)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	_ = json.Unmarshal(resp.Body.Bytes(), &variables)

	assert.Equal(t, http.StatusOK, resp.Code, "should return 200 OK")
	assert.Equal(t, 0, len(variables), "should return 0 results")

	req, _ = http.NewRequest("GET", fakeURL, nil)
	req.Header.Set("Content-Type", "application/json")

	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	_ = json.Unmarshal(resp.Body.Bytes(), &variables)

	assert.Equal(t, http.StatusUnauthorized, resp.Code, "should return 401 Unauthorized")

	fakeURL = fmt.Sprintf(urlFormat, "fakeApp", env1, comp1)
	req, _ = http.NewRequest("GET", fakeURL, nil)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	_ = json.Unmarshal(resp.Body.Bytes(), &variables)

	assert.Equal(t, http.StatusOK, resp.Code, "should return 200 OK")
	assert.Equal(t, 0, len(variables), "should return 0 results")
}
