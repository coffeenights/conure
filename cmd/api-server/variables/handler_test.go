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
	"github.com/stretchr/testify/require"
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
		MongoDBName:        "conure-test-variables",
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

	// Org owned by the authenticated user — what the happy-path requests use.
	ownedOrg := &models.Organization{Status: models.OrgActive, AccountID: user.ID}
	ownedOrgIDHex, err := ownedOrg.Create(mongo)
	require.NoError(t, err)
	orgID, err := primitive.ObjectIDFromHex(ownedOrgIDHex)
	require.NoError(t, err)

	// Org owned by a different user — variables here must NOT be visible.
	otherOrg := &models.Organization{Status: models.OrgActive, AccountID: primitive.NewObjectID()}
	otherOrgIDHex, err := otherOrg.Create(mongo)
	require.NoError(t, err)

	orgVar := &models.Variable{
		OrganizationID: orgID,
		Name:           "var1",
		Value:          "value1",
		IsEncrypted:    false,
		Type:           models.OrganizationType,
	}
	_, _ = orgVar.Create(mongo)

	encryptedValue, err := EncryptValue(keyStorage, "value2")
	require.NoError(t, err)
	orgVar2 := &models.Variable{
		OrganizationID: orgID,
		Name:           "var2",
		Value:          encryptedValue,
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

	// Valid object id but the org doesn't exist → 404.
	req, _ = http.NewRequest("GET", "/variables/"+primitive.NewObjectID().Hex(), nil)
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "auth", Value: token})

	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusNotFound, resp.Code, "should return 404 Not Found")

	// Org owned by someone else → 403.
	req, _ = http.NewRequest("GET", "/variables/"+otherOrgIDHex, nil)
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "auth", Value: token})

	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusForbidden, resp.Code, "should return 403 Forbidden when org is owned by another user")

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
		MongoDBName:        "conure-test-variables",
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

	ownedOrg := &models.Organization{Status: models.OrgActive, AccountID: user.ID}
	ownedOrgIDHex, err := ownedOrg.Create(mongo)
	require.NoError(t, err)
	orgID1, err := primitive.ObjectIDFromHex(ownedOrgIDHex)
	require.NoError(t, err)

	otherOrg := &models.Organization{Status: models.OrgActive, AccountID: primitive.NewObjectID()}
	otherOrgIDHex, err := otherOrg.Create(mongo)
	require.NoError(t, err)

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

	encryptedValue, err := EncryptValue(keyStorage, "value2")
	require.NoError(t, err)
	orgVar2 := &models.Variable{
		OrganizationID: orgID1,
		EnvironmentID:  &env1,
		ApplicationID:  &app1,
		Name:           "var2",
		Value:          encryptedValue,
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

	// Foreign org → 403.
	fakeURL = fmt.Sprintf(urlFormat, otherOrgIDHex, app1.Hex(), env1)
	req, _ = http.NewRequest("GET", fakeURL, nil)
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "auth", Value: token})

	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusForbidden, resp.Code, "should return 403 for foreign org")
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
		MongoDBName:        "conure-test-variables",
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

	ownedOrg := &models.Organization{Status: models.OrgActive, AccountID: user.ID}
	ownedOrgIDHex, err := ownedOrg.Create(mongo)
	require.NoError(t, err)
	orgID1, err := primitive.ObjectIDFromHex(ownedOrgIDHex)
	require.NoError(t, err)

	otherOrg := &models.Organization{Status: models.OrgActive, AccountID: primitive.NewObjectID()}
	otherOrgIDHex, err := otherOrg.Create(mongo)
	require.NoError(t, err)

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

	encryptedValue, err := EncryptValue(keyStorage, "value2")
	require.NoError(t, err)
	orgVar2 := &models.Variable{
		OrganizationID: orgID1,
		EnvironmentID:  &env1,
		ApplicationID:  &app1,
		ComponentID:    &comp1,
		Name:           "var2",
		Value:          encryptedValue,
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

	// Foreign org → 403.
	fakeURL = fmt.Sprintf(urlFormat, otherOrgIDHex, app1.Hex(), env1, comp1.Hex())
	req, _ = http.NewRequest("GET", fakeURL, nil)
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "auth", Value: token})

	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusForbidden, resp.Code, "should return 403 for foreign org")
}

func TestHandler_ListEnvironmentVariablesAllScopes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	token, _ := auth.GenerateToken(1*time.Hour, auth.JWTData{
		Email: "test@test.com", Client: "test-client",
	}, "test-secret")

	config := &apiConfig.Config{
		JWTSecret:          "test-secret",
		MongoDBURI:         "mongodb://localhost:27017",
		MongoDBName:        "conure-test-variables",
		AuthStrategySystem: "local",
	}
	mongo, _ := database.ConnectToMongoDB(config.MongoDBURI, config.MongoDBName)
	defer cleanUpDB(mongo)
	keyStorage := NewLocalSecretKey("secret.key")

	user := models.User{Email: "test@test.com", Client: "test-client"}
	_ = user.Create(mongo)

	ownedOrg := &models.Organization{Status: models.OrgActive, AccountID: user.ID}
	ownedOrgIDHex, err := ownedOrg.Create(mongo)
	require.NoError(t, err)
	orgID, err := primitive.ObjectIDFromHex(ownedOrgIDHex)
	require.NoError(t, err)
	appID := primitive.NewObjectID()
	envName := "prod"

	// Seed: org-only var, env-only var, and a name that exists in both
	// tiers — the env value should win.
	orgOnly := &models.Variable{OrganizationID: orgID, Name: "ORG_ONLY", Value: "from-org", Type: models.OrganizationType}
	_, _ = orgOnly.Create(mongo)
	shared := &models.Variable{OrganizationID: orgID, Name: "SHARED", Value: "org-value", Type: models.OrganizationType}
	_, _ = shared.Create(mongo)

	// Env-tier: one plain, one encrypted, and the SHARED override.
	envApp := appID
	envEnv := envName
	envOnly := &models.Variable{OrganizationID: orgID, ApplicationID: &envApp, EnvironmentID: &envEnv, Name: "ENV_ONLY", Value: "from-env", Type: models.EnvironmentType}
	_, _ = envOnly.Create(mongo)
	encryptedValue, err := EncryptValue(keyStorage, "secret-plain")
	require.NoError(t, err)
	envSecret := &models.Variable{OrganizationID: orgID, ApplicationID: &envApp, EnvironmentID: &envEnv, Name: "ENV_SECRET", Value: encryptedValue, IsEncrypted: true, Type: models.EnvironmentType}
	_, _ = envSecret.Create(mongo)
	envShared := &models.Variable{OrganizationID: orgID, ApplicationID: &envApp, EnvironmentID: &envEnv, Name: "SHARED", Value: "env-value", Type: models.EnvironmentType}
	_, _ = envShared.Create(mongo)

	setupTestHandler(router, mongo, config, keyStorage)

	url := fmt.Sprintf("/variables/%s/%s/e/%s/allscopes", orgID.Hex(), appID.Hex(), envName)
	req, _ := http.NewRequest("GET", url, nil)
	req.AddCookie(&http.Cookie{Name: "auth", Value: token})

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)

	var got []models.Variable
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &got))

	byName := map[string]models.Variable{}
	for _, v := range got {
		byName[v.Name] = v
	}
	assert.Equal(t, 4, len(got), "should return ORG_ONLY, ENV_ONLY, ENV_SECRET, SHARED")
	assert.Equal(t, "from-org", byName["ORG_ONLY"].Value)
	assert.Equal(t, models.OrganizationType, byName["ORG_ONLY"].Type)
	assert.Equal(t, "from-env", byName["ENV_ONLY"].Value)
	assert.Equal(t, models.EnvironmentType, byName["ENV_ONLY"].Type)
	assert.Equal(t, "env-value", byName["SHARED"].Value, "env should override org")
	assert.Equal(t, models.EnvironmentType, byName["SHARED"].Type)
	assert.Equal(t, "secret-plain", byName["ENV_SECRET"].Value, "secret value should be decrypted in response")
	assert.True(t, byName["ENV_SECRET"].IsEncrypted)
}

func TestHandler_ListComponentVariablesAllScopes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	token, _ := auth.GenerateToken(1*time.Hour, auth.JWTData{
		Email: "test@test.com", Client: "test-client",
	}, "test-secret")

	config := &apiConfig.Config{
		JWTSecret:          "test-secret",
		MongoDBURI:         "mongodb://localhost:27017",
		MongoDBName:        "conure-test-variables",
		AuthStrategySystem: "local",
	}
	mongo, _ := database.ConnectToMongoDB(config.MongoDBURI, config.MongoDBName)
	defer cleanUpDB(mongo)
	keyStorage := NewLocalSecretKey("secret.key")

	user := models.User{Email: "test@test.com", Client: "test-client"}
	_ = user.Create(mongo)

	ownedOrg := &models.Organization{Status: models.OrgActive, AccountID: user.ID}
	ownedOrgIDHex, err := ownedOrg.Create(mongo)
	require.NoError(t, err)
	orgID, err := primitive.ObjectIDFromHex(ownedOrgIDHex)
	require.NoError(t, err)
	appID := primitive.NewObjectID()
	compID := primitive.NewObjectID()
	envName := "prod"

	// Three tiers, one common name: component should win.
	orgVar := &models.Variable{OrganizationID: orgID, Name: "SHARED", Value: "org-value", Type: models.OrganizationType}
	_, _ = orgVar.Create(mongo)

	envApp := appID
	envEnv := envName
	envVar := &models.Variable{OrganizationID: orgID, ApplicationID: &envApp, EnvironmentID: &envEnv, Name: "SHARED", Value: "env-value", Type: models.EnvironmentType}
	_, _ = envVar.Create(mongo)
	envOnly := &models.Variable{OrganizationID: orgID, ApplicationID: &envApp, EnvironmentID: &envEnv, Name: "ENV_ONLY", Value: "env-only", Type: models.EnvironmentType}
	_, _ = envOnly.Create(mongo)

	compApp := appID
	compEnv := envName
	compComp := compID
	compVar := &models.Variable{OrganizationID: orgID, ApplicationID: &compApp, EnvironmentID: &compEnv, ComponentID: &compComp, Name: "SHARED", Value: "comp-value", Type: models.ComponentType}
	_, _ = compVar.Create(mongo)
	compOnly := &models.Variable{OrganizationID: orgID, ApplicationID: &compApp, EnvironmentID: &compEnv, ComponentID: &compComp, Name: "COMP_ONLY", Value: "comp-only", Type: models.ComponentType}
	_, _ = compOnly.Create(mongo)

	setupTestHandler(router, mongo, config, keyStorage)

	url := fmt.Sprintf("/variables/%s/%s/e/%s/c/%s/allscopes", orgID.Hex(), appID.Hex(), envName, compID.Hex())
	req, _ := http.NewRequest("GET", url, nil)
	req.AddCookie(&http.Cookie{Name: "auth", Value: token})

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)

	var got []models.Variable
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &got))

	byName := map[string]models.Variable{}
	for _, v := range got {
		byName[v.Name] = v
	}
	assert.Equal(t, 3, len(got), "should return SHARED, ENV_ONLY, COMP_ONLY")
	assert.Equal(t, "comp-value", byName["SHARED"].Value, "component should win over env and org")
	assert.Equal(t, models.ComponentType, byName["SHARED"].Type)
	assert.Equal(t, "env-only", byName["ENV_ONLY"].Value)
	assert.Equal(t, models.EnvironmentType, byName["ENV_ONLY"].Type)
	assert.Equal(t, "comp-only", byName["COMP_ONLY"].Value)
	assert.Equal(t, models.ComponentType, byName["COMP_ONLY"].Type)
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
		MongoDBName:        "conure-test-variables",
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

	ownedOrg := &models.Organization{Status: models.OrgActive, AccountID: user.ID}
	ownedOrgIDHex, err := ownedOrg.Create(mongo)
	require.NoError(t, err)
	orgID1, err := primitive.ObjectIDFromHex(ownedOrgIDHex)
	require.NoError(t, err)

	otherOrg := &models.Organization{Status: models.OrgActive, AccountID: primitive.NewObjectID()}
	otherOrgIDHex, err := otherOrg.Create(mongo)
	require.NoError(t, err)

	setupTestHandler(router, mongo, config, keyStorage)
	newVar := models.Variable{
		Name:        "newVar",
		Value:       "value2",
		IsEncrypted: true,
	}

	jsonVar, _ := json.Marshal(newVar)
	var result models.Variable

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

	// Posting to a foreign org must be rejected.
	newVar = models.Variable{Name: "stealthVar", Value: "x", IsEncrypted: false}
	jsonVar, _ = json.Marshal(newVar)
	req, _ = http.NewRequest("POST", "/variables/"+otherOrgIDHex, bytes.NewBuffer(jsonVar))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "auth", Value: token})

	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusForbidden, resp.Code, "should return 403 for foreign org")

	// Posting to a non-existent org should 404.
	req, _ = http.NewRequest("POST", "/variables/"+primitive.NewObjectID().Hex(), bytes.NewBuffer(jsonVar))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "auth", Value: token})

	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusNotFound, resp.Code, "should return 404 for non-existent org")
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
		MongoDBName:        "conure-test-variables",
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

	ownedOrg := &models.Organization{Status: models.OrgActive, AccountID: user.ID}
	ownedOrgIDHex, err := ownedOrg.Create(mongo)
	require.NoError(t, err)
	orgID1, err := primitive.ObjectIDFromHex(ownedOrgIDHex)
	require.NoError(t, err)

	otherOrg := &models.Organization{Status: models.OrgActive, AccountID: primitive.NewObjectID()}
	otherOrgIDHex, err := otherOrg.Create(mongo)
	require.NoError(t, err)

	setupTestHandler(router, mongo, config, keyStorage)
	newVar := models.Variable{
		Name:        "newVar",
		Value:       "value2",
		IsEncrypted: true,
	}
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

	// Foreign org → 403.
	newVar = models.Variable{Name: "stealthVar", Value: "x", IsEncrypted: false}
	jsonVar, _ = json.Marshal(newVar)
	foreignURL := fmt.Sprintf(urlFormat, otherOrgIDHex, appID1.Hex(), "env1")
	req, _ = http.NewRequest("POST", foreignURL, bytes.NewBuffer(jsonVar))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "auth", Value: token})

	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusForbidden, resp.Code, "should return 403 for foreign org")
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
		MongoDBName:        "conure-test-variables",
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

	ownedOrg := &models.Organization{Status: models.OrgActive, AccountID: user.ID}
	ownedOrgIDHex, err := ownedOrg.Create(mongo)
	require.NoError(t, err)
	orgID1, err := primitive.ObjectIDFromHex(ownedOrgIDHex)
	require.NoError(t, err)

	otherOrg := &models.Organization{Status: models.OrgActive, AccountID: primitive.NewObjectID()}
	otherOrgIDHex, err := otherOrg.Create(mongo)
	require.NoError(t, err)

	setupTestHandler(router, mongo, config, keyStorage)
	newVar := models.Variable{
		Name:        "newVar",
		Value:       "value2",
		IsEncrypted: true,
	}

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

	// Foreign org → 403.
	newVar = models.Variable{Name: "stealthVar", Value: "x", IsEncrypted: false}
	jsonVar, _ = json.Marshal(newVar)
	foreignURL := fmt.Sprintf(urlFormat, otherOrgIDHex, appID1.Hex(), "env1", compID1.Hex())
	req, _ = http.NewRequest("POST", foreignURL, bytes.NewBuffer(jsonVar))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "auth", Value: token})

	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusForbidden, resp.Code, "should return 403 for foreign org")
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
		MongoDBName:        "conure-test-variables",
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

	org := &models.Organization{Status: models.OrgActive, AccountID: user.ID}
	orgIDHex, err := org.Create(mongo)
	require.NoError(t, err, "failed to create organization")
	orgID, err := primitive.ObjectIDFromHex(orgIDHex)
	require.NoError(t, err)

	// Second org owned by someone else, used to verify cross-org delete fails.
	otherOrg := &models.Organization{Status: models.OrgActive, AccountID: primitive.NewObjectID()}
	otherOrgIDHex, err := otherOrg.Create(mongo)
	require.NoError(t, err)
	otherOrgID, err := primitive.ObjectIDFromHex(otherOrgIDHex)
	require.NoError(t, err)

	newVar := models.Variable{
		OrganizationID: orgID,
		Type:           models.OrganizationType,
		Name:           "newVar",
		Value:          "value",
		IsEncrypted:    true,
	}
	varID, err := newVar.Create(mongo)
	require.NoError(t, err, "failed to create variable")

	// A variable that belongs to the other org. The owned user must not be
	// able to delete it by pairing their own org ID with this variable ID.
	foreignVar := models.Variable{
		OrganizationID: otherOrgID,
		Type:           models.OrganizationType,
		Name:           "foreignVar",
		Value:          "secret",
	}
	foreignVarID, err := foreignVar.Create(mongo)
	require.NoError(t, err)

	// Cross-org pairing must be rejected.
	req, _ := http.NewRequest("DELETE", "/variables/"+orgIDHex+"/"+foreignVarID, nil)
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "auth", Value: token})

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusNotFound, resp.Code, "deleting a variable from another org must not succeed")

	// And the foreign variable must still exist.
	var stillThere models.Variable
	err = stillThere.GetByID(mongo, foreignVar.ID.Hex())
	require.NoError(t, err, "foreign variable should still exist after blocked delete")

	// Happy path: delete own org's variable.
	req, _ = http.NewRequest("DELETE", "/variables/"+orgIDHex+"/"+varID, nil)
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "auth", Value: token})

	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusNoContent, resp.Code, "should return 204 No Content")

	var result models.Variable
	err = result.GetByID(mongo, newVar.ID.Hex())
	assert.ErrorIsf(t, err, conureerrors.ErrObjectNotFound, "should return error as variable does not exist")

	// Foreign org → 403.
	req, _ = http.NewRequest("DELETE", "/variables/"+otherOrgIDHex+"/"+foreignVarID, nil)
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "auth", Value: token})

	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusForbidden, resp.Code, "should return 403 for foreign org")
}
