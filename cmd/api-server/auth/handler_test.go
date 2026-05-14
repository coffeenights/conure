package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"

	apiConfig "github.com/coffeenights/conure/cmd/api-server/config"
	"github.com/coffeenights/conure/cmd/api-server/database"
	"github.com/coffeenights/conure/cmd/api-server/models"
)

func setupTestHandler(router *gin.Engine, mongo *database.MongoDB, conf *apiConfig.Config) {
	authHandler := NewAuthHandler(conf, mongo)
	GenerateRoutes("/auth", router, authHandler)
}

func TestHandler_Login(t *testing.T) {
	gin.SetMode(gin.TestMode)
	config := &apiConfig.Config{
		JWTSecret:   "test-secret",
		MongoDBURI:  "mongodb://localhost:27017",
		MongoDBName: "conure-test-auth",
	}
	testPassword := "password123"
	router := gin.New()

	mongo, _ := database.ConnectToMongoDB(config.MongoDBURI, config.MongoDBName)
	defer cleanUpDB(mongo)
	setupTestHandler(router, mongo, config)

	hashed, _ := GenerateFromPassword(testPassword)
	user := models.User{
		Email:    "test@test.com",
		Client:   "test-client",
		Password: hashed,
	}
	_ = user.Create(mongo)

	// Create a request with correct credentials
	loginRequest := LoginRequest{
		Email:    user.Email,
		Password: testPassword,
	}
	jsonData, _ := json.Marshal(loginRequest)

	req, _ := http.NewRequest("POST", "/auth/login", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	response := make(map[string]interface{})
	_ = json.Unmarshal(resp.Body.Bytes(), &response)
	assert.Equal(t, http.StatusOK, resp.Code, "(login correct) should return 200 OK")
	assert.NotEmpty(t, response["token"], "(login correct) should return a token")
	u := models.User{}
	_ = u.GetByEmail(mongo, user.Email)
	assert.NotNil(t, u.LastLoginAt, "(login correct) should update last login time")

	// Create a request with invalid credentials
	loginRequest.Password = "invalid-password"
	jsonData, _ = json.Marshal(loginRequest)
	req, _ = http.NewRequest("POST", "/auth/login", bytes.NewBuffer(jsonData))

	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusUnauthorized, resp.Code, "(invalid password) should return 401 Unauthorized")

	// Create a request with unknown email
	loginRequest.Email = "fake-email@test.com"
	loginRequest.Password = "password123"
	jsonData, _ = json.Marshal(loginRequest)
	req, _ = http.NewRequest("POST", "/auth/login", bytes.NewBuffer(jsonData))

	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusUnauthorized, resp.Code, "(unknown email) should return 401 Unauthorized")

	// Create a request with no email
	loginRequest.Email = ""
	loginRequest.Password = "password123"
	jsonData, _ = json.Marshal(loginRequest)
	req, _ = http.NewRequest("POST", "/auth/login", bytes.NewBuffer(jsonData))

	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusBadRequest, resp.Code, "(no email) should return 400 Bad Request")

	// Create a request with no password
	loginRequest.Password = ""
	jsonData, _ = json.Marshal(loginRequest)
	req, _ = http.NewRequest("POST", "/auth/login", bytes.NewBuffer(jsonData))

	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusBadRequest, resp.Code, "(no password) should return 400 Bad Request")
}

func TestHandler_Me(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	config := &apiConfig.Config{
		JWTSecret:   "test-secret",
		MongoDBURI:  "mongodb://localhost:27017",
		MongoDBName: "conure-test-auth",
	}

	testPayload := JWTData{
		Email:  "test@test.com",
		Client: "fake-client",
	}
	testPassword := "password123"

	mongo, _ := database.ConnectToMongoDB(config.MongoDBURI, config.MongoDBName)
	defer cleanUpDB(mongo)
	setupTestHandler(router, mongo, config)

	hashed, _ := GenerateFromPassword(testPassword)
	user := models.User{
		Email:    "test@test.com",
		Client:   "test-client",
		Password: hashed,
	}
	_ = user.Create(mongo)
	token, _ := GenerateToken(1*time.Hour, testPayload, config.JWTSecret)

	err := user.Create(mongo)
	assert.Error(t, err, "should return error when creating user with same email")

	// Create a request with correct token
	req, _ := http.NewRequest("GET", "/auth/me", nil)
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "auth", Value: token})

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	response := models.User{}
	_ = json.Unmarshal(resp.Body.Bytes(), &response)
	assert.Equal(t, http.StatusOK, resp.Code, "(correct user) should return 200 OK")
	assert.Equal(t, user.Email, response.Email, "(correct user) should return the correct user")
	assert.Equal(t, user.Client, response.Client, "(correct user) should return the correct client")

	// Create a request without token
	req, _ = http.NewRequest("GET", "/auth/me", nil)
	req.Header.Set("Content-Type", "application/json")

	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusUnauthorized, resp.Code, "(without token) should return 401 Unauthorized")

	// Create a request with inactive user
	user = models.User{
		Email:    "test1@test.com",
		Client:   "test-client",
		Password: hashed,
		IsActive: false,
	}
	_ = user.Create(mongo)
	collection := mongo.Client.Database(mongo.DBName).Collection(models.UserCollection)
	filter := bson.M{"email": "test1@test.com"}
	update := bson.M{"$set": bson.M{"isActive": false}}
	_, _ = collection.UpdateOne(context.Background(), filter, update)
	testPayload = JWTData{
		Email:  "test1@test.com",
		Client: "fake-client",
	}
	token, _ = GenerateToken(1*time.Hour, testPayload, config.JWTSecret)

	req, _ = http.NewRequest("GET", "/auth/me", nil)
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "auth", Value: token})

	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusUnauthorized, resp.Code, "(inactive user) should return 401 Unauthorized")
}

func TestHandler_ChangePassword(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	config := &apiConfig.Config{
		JWTSecret:   "test-secret",
		MongoDBURI:  "mongodb://localhost:27017",
		MongoDBName: "conure-test-auth",
	}

	testPayload := JWTData{
		Email:  "test@test.com",
		Client: "fake-client",
	}
	testPassword := "password123"

	mongo, _ := database.ConnectToMongoDB(config.MongoDBURI, config.MongoDBName)
	defer cleanUpDB(mongo)
	setupTestHandler(router, mongo, config)

	hashed, _ := GenerateFromPassword(testPassword)
	user := models.User{
		Email:    "test@test.com",
		Client:   "test-client",
		Password: hashed,
	}
	_ = user.Create(mongo)
	token, _ := GenerateToken(1*time.Hour, testPayload, config.JWTSecret)

	// Create a request with correct credentials
	changePasswordRequest := ChangePasswordRequest{
		OldPassword: testPassword,
		Password:    "NewPassword123+",
		Password2:   "NewPassword123+",
	}
	jsonData, _ := json.Marshal(changePasswordRequest)

	req, _ := http.NewRequest("PATCH", "/auth/change-password", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "auth", Value: token})

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	response := make(map[string]interface{})
	_ = json.Unmarshal(resp.Body.Bytes(), &response)
	assert.Equal(t, http.StatusOK, resp.Code, "(update correct) should return 200 OK")
	assert.NotEmpty(t, response["message"], "(update correct) should return a message")
	u := models.User{}
	_ = u.GetByEmail(mongo, user.Email)
	assert.NotEqualf(t, user.Password, u.Password, "(update correct) should update the password")

	// Create a request with invalid old password
	user.ID = primitive.NewObjectID()
	user.Email = "test1@test.com"
	testPayload.Email = "test1@test.com"
	_ = user.Create(mongo)
	token, _ = GenerateToken(1*time.Hour, testPayload, config.JWTSecret)

	changePasswordRequest.OldPassword = "invalid-password"
	jsonData, _ = json.Marshal(changePasswordRequest)
	req, _ = http.NewRequest("PATCH", "/auth/change-password", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "auth", Value: token})

	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusBadRequest, resp.Code, "(invalid password) should return 400 Bad Request")
	u = models.User{}
	_ = u.GetById(mongo, user.ID.Hex())
	assert.Equalf(t, user.Password, u.Password, "(invalid password) should not update the password")

	// Create a request with not matching confirm password
	changePasswordRequest.Password = "Password123-"
	changePasswordRequest.Password2 = "Password123+"
	jsonData, _ = json.Marshal(changePasswordRequest)
	req, _ = http.NewRequest("PATCH", "/auth/change-password", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "auth", Value: token})

	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusBadRequest, resp.Code, "(bad confirm password) should return 400 Bad Request")

	// Create a request without old password
	changePasswordRequest.OldPassword = ""
	jsonData, _ = json.Marshal(changePasswordRequest)
	req, _ = http.NewRequest("PATCH", "/auth/change-password", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "auth", Value: token})

	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusBadRequest, resp.Code, "(no old password) should return 400 Bad Request")

	// Create a request without token
	jsonData, _ = json.Marshal(changePasswordRequest)
	req, _ = http.NewRequest("PATCH", "/auth/change-password", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")

	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusUnauthorized, resp.Code, "(no token) should return 401 Unauthorized")
}

// TestHandler_UpdateMe covers PATCH /auth/me — the self-service email
// edit. Mirrors the structure of TestHandler_ChangePassword: walk through
// the happy path, validation rejects, duplicate-email collision, and the
// unauthenticated case.
func TestHandler_UpdateMe(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	config := &apiConfig.Config{
		JWTSecret:   "test-secret",
		MongoDBURI:  "mongodb://localhost:27017",
		MongoDBName: "conure-test-auth",
	}

	mongo, _ := database.ConnectToMongoDB(config.MongoDBURI, config.MongoDBName)
	defer cleanUpDB(mongo)
	setupTestHandler(router, mongo, config)

	// The auth router doesn't run the full bootstrap that installs the
	// unique-email index, so install it directly. Without it, the
	// duplicate-email subtest would silently succeed via two coexisting
	// rows instead of exercising the 11000 → ErrEmailAlreadyExists path.
	if err := models.EnsureUserIndexes(context.Background(), mongo); err != nil {
		t.Fatal(err)
	}

	hashed, _ := GenerateFromPassword("Password123")
	caller := models.User{Email: "caller@test.com", Client: "test-client", Password: hashed}
	if err := caller.Create(mongo); err != nil {
		t.Fatal(err)
	}
	other := models.User{Email: "taken@test.com", Client: "test-client", Password: hashed}
	if err := other.Create(mongo); err != nil {
		t.Fatal(err)
	}

	tokenFor := func(email string) string {
		tok, err := GenerateToken(time.Hour, JWTData{Email: email}, config.JWTSecret)
		if err != nil {
			t.Fatal(err)
		}
		return tok
	}

	doPATCH := func(t *testing.T, body any, cookie *http.Cookie) *httptest.ResponseRecorder {
		t.Helper()
		jsonData, _ := json.Marshal(body)
		req, _ := http.NewRequest("PATCH", "/auth/me", bytes.NewBuffer(jsonData))
		req.Header.Set("Content-Type", "application/json")
		if cookie != nil {
			req.AddCookie(cookie)
		}
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		return w
	}

	t.Run("happy path updates email", func(t *testing.T) {
		newEmail := "renamed@test.com"
		resp := doPATCH(t, UpdateMeRequest{Email: &newEmail}, &http.Cookie{Name: "auth", Value: tokenFor(caller.Email)})
		assert.Equal(t, http.StatusOK, resp.Code, "should return 200 on successful update")

		// Verify the row actually changed.
		u := models.User{}
		_ = u.GetByEmail(mongo, newEmail)
		assert.Equal(t, newEmail, u.Email, "email column should be the new value")

		// Reset for subsequent subtests.
		u.Email = caller.Email
		if err := u.Update(mongo); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("invalid email is rejected", func(t *testing.T) {
		bad := "not-an-email"
		resp := doPATCH(t, UpdateMeRequest{Email: &bad}, &http.Cookie{Name: "auth", Value: tokenFor(caller.Email)})
		assert.Equal(t, http.StatusBadRequest, resp.Code)
		assert.Contains(t, resp.Body.String(), "invalid_email")
	})

	t.Run("duplicate email collides with another user", func(t *testing.T) {
		clash := other.Email
		resp := doPATCH(t, UpdateMeRequest{Email: &clash}, &http.Cookie{Name: "auth", Value: tokenFor(caller.Email)})
		assert.Equal(t, http.StatusBadRequest, resp.Code)
		assert.Contains(t, resp.Body.String(), "email_already_exists")
	})

	t.Run("no token returns 401", func(t *testing.T) {
		newEmail := "anything@test.com"
		resp := doPATCH(t, UpdateMeRequest{Email: &newEmail}, nil)
		assert.Equal(t, http.StatusUnauthorized, resp.Code)
	})

	t.Run("empty body is a no-op success", func(t *testing.T) {
		// Sending {} means "change nothing." The handler should treat that
		// as a successful no-op rather than failing — we don't want clients
		// to have to send the current email back just to keep it.
		resp := doPATCH(t, struct{}{}, &http.Cookie{Name: "auth", Value: tokenFor(caller.Email)})
		assert.Equal(t, http.StatusOK, resp.Code)
	})
}
