package variables

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/coffeenights/conure/cmd/api-server/auth"
	"github.com/coffeenights/conure/cmd/api-server/conureerrors"
	"github.com/coffeenights/conure/cmd/api-server/models"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson/primitive"

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

	user := models.User{
		Email:  "test@test.com",
		Client: "test-client",
	}
	_ = user.Create(mongo)
	orgID := primitive.NewObjectID()
	orgVar := &models.Variable{
		OrganizationID: orgID,
		Name:           "var1",
		Value:          "value1",
		IsEncrypted:    false,
		Type:           models.OrganizationType,
	}
	_, _ = orgVar.Create(mongo)

	orgVar2 := &models.Variable{
		OrganizationID: orgID,
		Name:           "var2",
		Value:          EncryptValue(keyStorage, "value2"),
		IsEncrypted:    true,
		Type:           models.OrganizationType,
	}
	_, _ = orgVar2.Create(mongo)

	setupTestHandler(router, mongo, config, keyStorage)
	var variables []models.Variable

	req, _ := http.NewRequest("GET", "/variables/"+orgID.Hex(), nil)
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "auth", Value: token})

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
	req.AddCookie(&http.Cookie{Name: "auth", Value: token})

	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	_ = json.Unmarshal(resp.Body.Bytes(), &variables)

	assert.Equal(t, http.StatusBadRequest, resp.Code, "should return 400 Bad Request")

	req, _ = http.NewRequest("GET", "/variables/"+primitive.NewObjectID().Hex(), nil)
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "auth", Value: token})

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

	user := models.User{
		Email:  "test@test.com",
		Client: "test-client",
	}
	_ = user.Create(mongo)

	orgID1 := primitive.NewObjectID()
	app1 := primitive.NewObjectID()
	env1 := "env1"
	orgVar := &models.Variable{
		OrganizationID: orgID1,
		EnvironmentID:  &env1,
		ApplicationID:  &app1,
		Name:           "var1",
		Value:          "value1",
		IsEncrypted:    false,
		Type:           models.EnvironmentType,
	}
	_, _ = orgVar.Create(mongo)

	orgVar2 := &models.Variable{
		OrganizationID: orgID1,
		EnvironmentID:  &env1,
		ApplicationID:  &app1,
		Name:           "var2",
		Value:          EncryptValue(keyStorage, "value2"),
		IsEncrypted:    true,
		Type:           models.EnvironmentType,
	}
	_, _ = orgVar2.Create(mongo)

	setupTestHandler(router, mongo, config, keyStorage)
	var variables []models.Variable

	urlFormat := "/variables/%s/%s/e/%s"
	url := fmt.Sprintf(urlFormat, orgID1.Hex(), app1.Hex(), env1)
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "auth", Value: token})

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	_ = json.Unmarshal(resp.Body.Bytes(), &variables)

	assert.Equal(t, http.StatusOK, resp.Code, "should return 200 OK")
	assert.Equal(t, 2, len(variables), "should return 2 results")
	assert.Equal(t, orgVar.OrganizationID, variables[0].OrganizationID, "should return the correct organization")
	assert.Equal(t, orgVar.Type, variables[0].Type, "should return the correct type of variable")
	assert.True(t, variables[1].IsEncrypted, "should return the correct type of variable")

	fakeURL := fmt.Sprintf(urlFormat, orgID1.Hex(), app1.Hex(), "fakeEnv")
	req, _ = http.NewRequest("GET", fakeURL, nil)
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "auth", Value: token})

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

	fakeURL = fmt.Sprintf(urlFormat, orgID1.Hex(), "fakeApp", env1)
	req, _ = http.NewRequest("GET", fakeURL, nil)
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "auth", Value: token})

	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	_ = json.Unmarshal(resp.Body.Bytes(), &variables)

	assert.Equal(t, http.StatusBadRequest, resp.Code, "should return 400 Bad Request")

	fakeURL = fmt.Sprintf(urlFormat, "fakeOrg", primitive.NewObjectID().Hex(), env1)
	req, _ = http.NewRequest("GET", fakeURL, nil)
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "auth", Value: token})

	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	_ = json.Unmarshal(resp.Body.Bytes(), &variables)

	assert.Equal(t, http.StatusBadRequest, resp.Code, "should return 400 Bad Request")

	fakeURL = fmt.Sprintf(urlFormat, orgID1.Hex(), primitive.NewObjectID().Hex(), env1)
	req, _ = http.NewRequest("GET", fakeURL, nil)
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "auth", Value: token})

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

	user := models.User{
		Email:  "test@test.com",
		Client: "test-client",
	}
	_ = user.Create(mongo)

	orgID1 := primitive.NewObjectID()
	app1 := primitive.NewObjectID()
	env1 := "env1"
	comp1 := primitive.NewObjectID()
	orgVar := &models.Variable{
		OrganizationID: orgID1,
		EnvironmentID:  &env1,
		ApplicationID:  &app1,
		ComponentID:    &comp1,
		Name:           "var1",
		Value:          "value1",
		IsEncrypted:    false,
		Type:           models.ComponentType,
	}
	_, _ = orgVar.Create(mongo)

	orgVar2 := &models.Variable{
		OrganizationID: orgID1,
		EnvironmentID:  &env1,
		ApplicationID:  &app1,
		ComponentID:    &comp1,
		Name:           "var2",
		Value:          EncryptValue(keyStorage, "value2"),
		IsEncrypted:    true,
		Type:           models.ComponentType,
	}
	_, _ = orgVar2.Create(mongo)

	setupTestHandler(router, mongo, config, keyStorage)
	var variables []models.Variable

	urlFormat := "/variables/%s/%s/e/%s/c/%s"
	url := fmt.Sprintf(urlFormat, orgID1.Hex(), app1.Hex(), env1, comp1.Hex())
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "auth", Value: token})

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	_ = json.Unmarshal(resp.Body.Bytes(), &variables)

	assert.Equal(t, http.StatusOK, resp.Code, "should return 200 OK")
	assert.Equal(t, 2, len(variables), "should return 2 results")
	assert.Equal(t, orgVar.OrganizationID, variables[0].OrganizationID, "should return the correct organization")
	assert.Equal(t, orgVar.Type, variables[0].Type, "should return the correct type of variable")
	assert.True(t, variables[1].IsEncrypted, "should return the correct type of variable")

	fakeURL := fmt.Sprintf(urlFormat, orgID1.Hex(), app1.Hex(), env1, primitive.NewObjectID().Hex())
	req, _ = http.NewRequest("GET", fakeURL, nil)
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "auth", Value: token})

	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	_ = json.Unmarshal(resp.Body.Bytes(), &variables)

	assert.Equal(t, http.StatusOK, resp.Code, "should return 200 OK")
	assert.Equal(t, 0, len(variables), "should return 0 results")

	fakeURL = fmt.Sprintf(urlFormat, orgID1.Hex(), app1.Hex(), env1, "fakeComp")
	req, _ = http.NewRequest("GET", fakeURL, nil)
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "auth", Value: token})

	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	_ = json.Unmarshal(resp.Body.Bytes(), &variables)

	assert.Equal(t, http.StatusBadRequest, resp.Code, "should return 400 OK")

	fakeURL = fmt.Sprintf(urlFormat, "fakeOrg", app1.Hex(), env1, primitive.NewObjectID().Hex())
	req, _ = http.NewRequest("GET", fakeURL, nil)
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "auth", Value: token})

	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	_ = json.Unmarshal(resp.Body.Bytes(), &variables)

	assert.Equal(t, http.StatusBadRequest, resp.Code, "should return 400 Bad Request")

	req, _ = http.NewRequest("GET", fakeURL, nil)
	req.Header.Set("Content-Type", "application/json")

	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	_ = json.Unmarshal(resp.Body.Bytes(), &variables)

	assert.Equal(t, http.StatusUnauthorized, resp.Code, "should return 401 Unauthorized")

	fakeURL = fmt.Sprintf(urlFormat, orgID1.Hex(), "fakeApp", env1, comp1)
	req, _ = http.NewRequest("GET", fakeURL, nil)
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "auth", Value: token})

	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	_ = json.Unmarshal(resp.Body.Bytes(), &variables)

	assert.Equal(t, http.StatusBadRequest, resp.Code, "should return 400 Bad Request")

	fakeURL = fmt.Sprintf(urlFormat, orgID1.Hex(), primitive.NewObjectID().Hex(), env1, comp1.Hex())
	req, _ = http.NewRequest("GET", fakeURL, nil)
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "auth", Value: token})

	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	_ = json.Unmarshal(resp.Body.Bytes(), &variables)

	assert.Equal(t, http.StatusOK, resp.Code, "should return 200 OK")
	assert.Equal(t, 0, len(variables), "should return 0 results")
}

func TestHandler_CreateVariableOrg(t *testing.T) {
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

	user := models.User{
		Email:  "test@test.com",
		Client: "test-client",
	}
	_ = user.Create(mongo)

	setupTestHandler(router, mongo, config, keyStorage)
	newVar := models.Variable{
		Name:        "newVar",
		Value:       "value2",
		IsEncrypted: true,
	}

	jsonVar, _ := json.Marshal(newVar)
	var result models.Variable
	orgID1 := primitive.NewObjectID()

	req, _ := http.NewRequest("POST", "/variables/"+orgID1.Hex(), bytes.NewBuffer(jsonVar))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "auth", Value: token})

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	_ = json.Unmarshal(resp.Body.Bytes(), &result)

	assert.Equal(t, http.StatusCreated, resp.Code, "should return 201 Created")
	assert.Equal(t, orgID1, result.OrganizationID, "should return the correct organization")
	assert.Equal(t, models.OrganizationType, result.Type, "should return the correct type of variable")
	assert.NotEqual(t, newVar.Value, result.Value, "should return the encrypted value")
	assert.True(t, result.IsEncrypted, "should return the correct type of variable")

	newVar = models.Variable{
		Name:        "newVar2",
		Value:       "value2",
		IsEncrypted: false,
	}

	jsonVar, _ = json.Marshal(newVar)

	req, _ = http.NewRequest("POST", "/variables/"+orgID1.Hex(), bytes.NewBuffer(jsonVar))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "auth", Value: token})

	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	_ = json.Unmarshal(resp.Body.Bytes(), &result)

	assert.Equal(t, http.StatusCreated, resp.Code, "should return 201 Created")
	assert.Equal(t, orgID1, result.OrganizationID, "should return the correct organization")
	assert.Equal(t, models.OrganizationType, result.Type, "should return the correct type of variable")
	assert.Equal(t, newVar.Value, result.Value, "should return the encrypted value")
	assert.False(t, result.IsEncrypted, "should return the correct type of variable")

	req, _ = http.NewRequest("POST", "/variables/org1", bytes.NewBuffer(jsonVar))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "auth", Value: token})

	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	_ = json.Unmarshal(resp.Body.Bytes(), &result)

	assert.Equal(t, http.StatusBadRequest, resp.Code, "should return 401 BadRequest")

	req, _ = http.NewRequest("POST", "/variables/"+orgID1.Hex(), bytes.NewBuffer(jsonVar))
	req.Header.Set("Content-Type", "application/json")

	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	_ = json.Unmarshal(resp.Body.Bytes(), &result)

	assert.Equal(t, http.StatusUnauthorized, resp.Code, "should return 401 Unauthorized")

	req, _ = http.NewRequest("POST", "/variables/invalidID", bytes.NewBuffer(jsonVar))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "auth", Value: token})

	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	_ = json.Unmarshal(resp.Body.Bytes(), &result)

	assert.Equal(t, http.StatusBadRequest, resp.Code, "should return 400 BadRequest")

	newVar = models.Variable{
		Name: "newVarX",
	}
	jsonVar, _ = json.Marshal(newVar)
	req, _ = http.NewRequest("POST", "/variables/"+orgID1.Hex(), bytes.NewBuffer(jsonVar))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "auth", Value: token})

	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	_ = json.Unmarshal(resp.Body.Bytes(), &result)

	assert.Equal(t, http.StatusBadRequest, resp.Code, "should return 400 BadRequest")

	newVar = models.Variable{
		Name:        "Incorrect Variable $$$",
		Value:       "value2",
		IsEncrypted: false,
	}
	jsonVar, _ = json.Marshal(newVar)
	req, _ = http.NewRequest("POST", "/variables/"+orgID1.Hex(), bytes.NewBuffer(jsonVar))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "auth", Value: token})

	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	_ = json.Unmarshal(resp.Body.Bytes(), &result)

	assert.Equal(t, http.StatusBadRequest, resp.Code, "should return 400 BadRequest")
}

func TestHandler_CreateVariableEnv(t *testing.T) {
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

	user := models.User{
		Email:  "test@test.com",
		Client: "test-client",
	}
	_ = user.Create(mongo)

	setupTestHandler(router, mongo, config, keyStorage)
	newVar := models.Variable{
		Name:        "newVar",
		Value:       "value2",
		IsEncrypted: true,
	}
	orgID1 := primitive.NewObjectID()
	appID1 := primitive.NewObjectID()

	jsonVar, _ := json.Marshal(newVar)
	var result models.Variable

	urlFormat := "/variables/%s/%s/e/%s"
	url := fmt.Sprintf(urlFormat, orgID1.Hex(), appID1.Hex(), "env1")
	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(jsonVar))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "auth", Value: token})

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	_ = json.Unmarshal(resp.Body.Bytes(), &result)

	assert.Equal(t, http.StatusCreated, resp.Code, "should return 201 Created")
	assert.Equal(t, orgID1, result.OrganizationID, "should return the correct organization")
	assert.Equal(t, appID1, *result.ApplicationID, "should return the correct application")
	assert.Equal(t, "env1", *result.EnvironmentID, "should return the correct environment")
	assert.Equal(t, models.EnvironmentType, result.Type, "should return the correct type of variable")
	assert.NotEqual(t, newVar.Value, result.Value, "should return the encrypted value")
	assert.True(t, result.IsEncrypted, "should return the correct type of variable")

	newVar = models.Variable{
		Name:        "newVar2",
		Value:       "value2",
		IsEncrypted: false,
	}

	jsonVar, _ = json.Marshal(newVar)

	req, _ = http.NewRequest("POST", url, bytes.NewBuffer(jsonVar))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "auth", Value: token})

	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	_ = json.Unmarshal(resp.Body.Bytes(), &result)

	assert.Equal(t, http.StatusCreated, resp.Code, "should return 201 Created")
	assert.Equal(t, orgID1, result.OrganizationID, "should return the correct organization")
	assert.Equal(t, appID1, *result.ApplicationID, "should return the correct application")
	assert.Equal(t, "env1", *result.EnvironmentID, "should return the correct environment")
	assert.Equal(t, models.EnvironmentType, result.Type, "should return the correct type of variable")
	assert.Equal(t, newVar.Value, result.Value, "should return the encrypted value")
	assert.False(t, result.IsEncrypted, "should return the correct type of variable")

	req, _ = http.NewRequest("POST", url, bytes.NewBuffer(jsonVar))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "auth", Value: token})

	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	_ = json.Unmarshal(resp.Body.Bytes(), &result)

	assert.Equal(t, http.StatusBadRequest, resp.Code, "should return 401 BadRequest")

	req, _ = http.NewRequest("POST", url, bytes.NewBuffer(jsonVar))
	req.Header.Set("Content-Type", "application/json")

	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	_ = json.Unmarshal(resp.Body.Bytes(), &result)

	assert.Equal(t, http.StatusUnauthorized, resp.Code, "should return 401 Unauthorized")

	newVar = models.Variable{
		Name: "newVarX",
	}
	jsonVar, _ = json.Marshal(newVar)
	req, _ = http.NewRequest("POST", url, bytes.NewBuffer(jsonVar))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "auth", Value: token})

	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	_ = json.Unmarshal(resp.Body.Bytes(), &result)

	assert.Equal(t, http.StatusBadRequest, resp.Code, "should return 400 BadRequest")

	newVar = models.Variable{
		Name:        "Incorrect Variable $$$",
		Value:       "value2",
		IsEncrypted: false,
	}
	jsonVar, _ = json.Marshal(newVar)
	req, _ = http.NewRequest("POST", url, bytes.NewBuffer(jsonVar))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "auth", Value: token})

	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	_ = json.Unmarshal(resp.Body.Bytes(), &result)

	assert.Equal(t, http.StatusBadRequest, resp.Code, "should return 400 BadRequest")
}

func TestHandler_CreateVariableComp(t *testing.T) {
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

	user := models.User{
		Email:  "test@test.com",
		Client: "test-client",
	}
	_ = user.Create(mongo)

	setupTestHandler(router, mongo, config, keyStorage)
	newVar := models.Variable{
		Name:        "newVar",
		Value:       "value2",
		IsEncrypted: true,
	}

	orgID1 := primitive.NewObjectID()
	appID1 := primitive.NewObjectID()
	compID1 := primitive.NewObjectID()

	jsonVar, _ := json.Marshal(newVar)
	var result models.Variable

	urlFormat := "/variables/%s/%s/e/%s/c/%s"
	url := fmt.Sprintf(urlFormat, orgID1.Hex(), appID1.Hex(), "env1", compID1.Hex())
	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(jsonVar))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "auth", Value: token})

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	_ = json.Unmarshal(resp.Body.Bytes(), &result)

	assert.Equal(t, http.StatusCreated, resp.Code, "should return 201 Created")
	assert.Equal(t, orgID1, result.OrganizationID, "should return the correct organization")
	assert.Equal(t, appID1, *result.ApplicationID, "should return the correct application")
	assert.Equal(t, "env1", *result.EnvironmentID, "should return the correct environment")
	assert.Equal(t, compID1, *result.ComponentID, "should return the correct component")
	assert.Equal(t, models.ComponentType, result.Type, "should return the correct type of variable")
	assert.NotEqual(t, newVar.Value, result.Value, "should return the encrypted value")
	assert.True(t, result.IsEncrypted, "should return the correct type of variable")

	newVar = models.Variable{
		Name:        "newVar2",
		Value:       "value2",
		IsEncrypted: false,
	}

	jsonVar, _ = json.Marshal(newVar)

	req, _ = http.NewRequest("POST", url, bytes.NewBuffer(jsonVar))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "auth", Value: token})

	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	_ = json.Unmarshal(resp.Body.Bytes(), &result)

	assert.Equal(t, http.StatusCreated, resp.Code, "should return 201 Created")
	assert.Equal(t, orgID1, result.OrganizationID, "should return the correct organization")
	assert.Equal(t, appID1, *result.ApplicationID, "should return the correct application")
	assert.Equal(t, "env1", *result.EnvironmentID, "should return the correct environment")
	assert.Equal(t, compID1, *result.ComponentID, "should return the correct component")
	assert.Equal(t, models.ComponentType, result.Type, "should return the correct type of variable")
	assert.Equal(t, newVar.Value, result.Value, "should return the encrypted value")
	assert.False(t, result.IsEncrypted, "should return the correct type of variable")

	req, _ = http.NewRequest("POST", url, bytes.NewBuffer(jsonVar))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "auth", Value: token})

	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	_ = json.Unmarshal(resp.Body.Bytes(), &result)

	assert.Equal(t, http.StatusBadRequest, resp.Code, "should return 400 BadRequest")

	req, _ = http.NewRequest("POST", url, bytes.NewBuffer(jsonVar))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "auth", Value: token})

	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	_ = json.Unmarshal(resp.Body.Bytes(), &result)

	assert.Equal(t, http.StatusBadRequest, resp.Code, "should return 400 BadRequest")

	req, _ = http.NewRequest("POST", url, bytes.NewBuffer(jsonVar))
	req.Header.Set("Content-Type", "application/json")

	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	_ = json.Unmarshal(resp.Body.Bytes(), &result)

	assert.Equal(t, http.StatusUnauthorized, resp.Code, "should return 401 Unauthorized")

	fakeURL := fmt.Sprintf(urlFormat, orgID1.Hex(), appID1.Hex(), "env1", "fakeComp")
	req, _ = http.NewRequest("POST", fakeURL, bytes.NewBuffer(jsonVar))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "auth", Value: token})

	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	_ = json.Unmarshal(resp.Body.Bytes(), &result)

	assert.Equal(t, http.StatusBadRequest, resp.Code, "should return 400 badRequest")

	fakeURL = fmt.Sprintf(urlFormat, orgID1.Hex(), "fakeApp", "env1", compID1)
	req, _ = http.NewRequest("POST", fakeURL, bytes.NewBuffer(jsonVar))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "auth", Value: token})

	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	_ = json.Unmarshal(resp.Body.Bytes(), &result)

	assert.Equal(t, http.StatusBadRequest, resp.Code, "should return 400 BadRequest")

	newVar = models.Variable{
		Name: "newVarX",
	}
	jsonVar, _ = json.Marshal(newVar)
	req, _ = http.NewRequest("POST", url, bytes.NewBuffer(jsonVar))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "auth", Value: token})

	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	_ = json.Unmarshal(resp.Body.Bytes(), &result)

	assert.Equal(t, http.StatusBadRequest, resp.Code, "should return 400 BadRequest")

	newVar = models.Variable{
		Name:        "Incorrect Variable $$$",
		Value:       "value2",
		IsEncrypted: false,
	}
	jsonVar, _ = json.Marshal(newVar)
	req, _ = http.NewRequest("POST", url, bytes.NewBuffer(jsonVar))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "auth", Value: token})

	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	_ = json.Unmarshal(resp.Body.Bytes(), &result)

	assert.Equal(t, http.StatusBadRequest, resp.Code, "should return 400 BadRequest")
}

func TestHandler_DeleteVariable(t *testing.T) {
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

	user := models.User{
		Email:  "test@test.com",
		Client: "test-client",
	}
	_ = user.Create(mongo)

	setupTestHandler(router, mongo, config, keyStorage)
	newVar := models.Variable{
		Name:        "newVar",
		Value:       "value",
		IsEncrypted: true,
	}
	org := &models.Organization{Status: models.OrgActive, AccountID: user.ID}
	orgID, err := org.Create(mongo)
	if err != nil {
		assert.Fail(t, "failed to create organization")
	}
	varID, err := newVar.Create(mongo)
	if err != nil {
		assert.Fail(t, "failed to create variable")
	}

	req, _ := http.NewRequest("DELETE", "/variables/"+orgID+"/"+varID, nil)
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "auth", Value: token})

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusNoContent, resp.Code, "should return 204 No Content")

	var result models.Variable
	err = result.GetByID(mongo, newVar.ID.Hex())
	assert.ErrorIsf(t, err, conureerrors.ErrObjectNotFound, "should return error as variable does not exist")
}
