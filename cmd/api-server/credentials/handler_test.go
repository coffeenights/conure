package credentials

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/coffeenights/conure/cmd/api-server/auth"
	apiConfig "github.com/coffeenights/conure/cmd/api-server/config"
	"github.com/coffeenights/conure/cmd/api-server/database"
	"github.com/coffeenights/conure/cmd/api-server/models"
	"github.com/coffeenights/conure/cmd/api-server/variables"
)

// testEnv bootstraps a router with the credentials routes, an authenticated
// user, an org that user owns, and a second org owned by someone else (for
// the isolation/RBAC assertions). Mirrors the variables handler test setup;
// uses a per-package DB name so it is parallel-safe with the other suites.
type testEnv struct {
	router   *gin.Engine
	mongo    *database.MongoDB
	token    string
	ownedOrg primitive.ObjectID
	otherOrg primitive.ObjectID
	keyStore variables.SecretKeyStorage
}

func setup(t *testing.T) *testEnv {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()

	token, _ := auth.GenerateToken(time.Hour,
		auth.JWTData{Email: "test@test.com", Client: "test-client"}, "test-secret")

	conf := &apiConfig.Config{
		JWTSecret:          "test-secret",
		MongoDBURI:         "mongodb://localhost:27017",
		MongoDBName:        "conure-test-credentials",
		AuthStrategySystem: "local",
		AESStorageStrategy: "local",
	}
	mongo, err := database.ConnectToMongoDB(conf.MongoDBURI, conf.MongoDBName)
	require.NoError(t, err)
	t.Cleanup(func() { _ = mongo.Client.Database(mongo.DBName).Drop(context.Background()) })

	// Generate the AES key into a temp file so the suite doesn't depend on a
	// secret.key existing in this package's directory (it doesn't, unlike
	// cmd/api-server/variables).
	keyStore := variables.NewLocalSecretKey(filepath.Join(t.TempDir(), "secret.key"))
	require.NoError(t, keyStore.Generate())

	user := models.User{Email: "test@test.com", Client: "test-client"}
	require.NoError(t, user.Create(mongo))

	ownedOrg := &models.Organization{Status: models.OrgActive, AccountID: user.ID}
	ownedHex, err := ownedOrg.Create(mongo)
	require.NoError(t, err)
	ownedID, err := primitive.ObjectIDFromHex(ownedHex)
	require.NoError(t, err)

	otherOrg := &models.Organization{Status: models.OrgActive, AccountID: primitive.NewObjectID()}
	otherHex, err := otherOrg.Create(mongo)
	require.NoError(t, err)
	otherID, err := primitive.ObjectIDFromHex(otherHex)
	require.NoError(t, err)

	GenerateRoutes("/credentials", router, NewApiHandler(conf, mongo, keyStore))
	return &testEnv{router, mongo, token, ownedID, otherID, keyStore}
}

func (e *testEnv) do(t *testing.T, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var r *bytes.Reader
	if body != nil {
		b, err := json.Marshal(body)
		require.NoError(t, err)
		r = bytes.NewReader(b)
	} else {
		r = bytes.NewReader(nil)
	}
	req, _ := http.NewRequest(method, path, r)
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "auth", Value: e.token})
	resp := httptest.NewRecorder()
	e.router.ServeHTTP(resp, req)
	return resp
}

func TestCreateCredential_ThenListIsMetadataOnly(t *testing.T) {
	e := setup(t)
	org := e.ownedOrg.Hex()

	resp := e.do(t, http.MethodPost, "/credentials/"+org+"/c", CreateCredentialRequest{
		Name: "ghcr", Kind: "registry", RegistryURL: "ghcr.io",
		Username: "octocat", Secret: "s3cr3t-token",
	})
	require.Equal(t, http.StatusCreated, resp.Code, "body=%s", resp.Body.String())

	// The create response must not echo the secret anywhere.
	assert.NotContains(t, resp.Body.String(), "s3cr3t-token")

	// Stored value must be ciphertext, not the plaintext.
	stored := &models.Credential{}
	require.NoError(t, stored.GetByOrgAndName(context.Background(), e.mongo, e.ownedOrg, "ghcr"))
	assert.NotEqual(t, "s3cr3t-token", stored.Secret, "secret stored in plaintext")
	dec, err := variables.DecryptValue(e.keyStore, stored.Secret)
	require.NoError(t, err)
	assert.Equal(t, "s3cr3t-token", dec, "ciphertext must decrypt to original")

	// List returns metadata only — no secret material in the payload.
	listResp := e.do(t, http.MethodGet, "/credentials/"+org+"/c", nil)
	require.Equal(t, http.StatusOK, listResp.Code)
	assert.NotContains(t, listResp.Body.String(), "s3cr3t-token")
	assert.NotContains(t, strings.ToLower(listResp.Body.String()), "\"secret\"")

	var list []CredentialResponse
	require.NoError(t, json.Unmarshal(listResp.Body.Bytes(), &list))
	require.Len(t, list, 1)
	assert.Equal(t, "ghcr", list[0].Name)
	assert.Equal(t, "registry", list[0].Kind)
	assert.Equal(t, "octocat", list[0].Username)
}

func TestCreateCredential_RotatesInPlace(t *testing.T) {
	e := setup(t)
	org := e.ownedOrg.Hex()

	first := e.do(t, http.MethodPost, "/credentials/"+org+"/c", CreateCredentialRequest{
		Name: "ghcr", Kind: "registry", RegistryURL: "ghcr.io", Username: "u1", Secret: "old",
	})
	require.Equal(t, http.StatusCreated, first.Code)

	// Re-posting the same name rotates: 200 (not 201), single row, new material.
	second := e.do(t, http.MethodPost, "/credentials/"+org+"/c", CreateCredentialRequest{
		Name: "ghcr", Kind: "registry", RegistryURL: "ghcr.io", Username: "u2", Secret: "new",
	})
	require.Equal(t, http.StatusOK, second.Code, "rotation should be 200, body=%s", second.Body.String())

	listResp := e.do(t, http.MethodGet, "/credentials/"+org+"/c", nil)
	var list []CredentialResponse
	require.NoError(t, json.Unmarshal(listResp.Body.Bytes(), &list))
	require.Len(t, list, 1, "rotation must not create a second row")
	assert.Equal(t, "u2", list[0].Username)

	stored := &models.Credential{}
	require.NoError(t, stored.GetByOrgAndName(context.Background(), e.mongo, e.ownedOrg, "ghcr"))
	dec, _ := variables.DecryptValue(e.keyStore, stored.Secret)
	assert.Equal(t, "new", dec)
}

// Regression for the omitempty+$set bug: creating a credential as `registry`
// (with a RegistryURL) then re-setting the SAME name as `git` (no registry)
// must fully replace the row — the stale registryUrl must NOT survive the
// rotation. Before the fix, `bson:"registryUrl,omitempty"` dropped the empty
// value from the $set doc and Mongo kept the old "ghcr.io".
func TestCreateCredential_CrossKindRotationClearsStaleRegistry(t *testing.T) {
	e := setup(t)
	org := e.ownedOrg.Hex()

	// First: a registry credential carrying a registry URL + username.
	require.Equal(t, http.StatusCreated, e.do(t, http.MethodPost, "/credentials/"+org+"/c",
		CreateCredentialRequest{
			Name: "dup", Kind: "registry", RegistryURL: "ghcr.io", Username: "octocat", Secret: "old",
		}).Code)

	// Rotate the same name to a git credential with no registry URL.
	require.Equal(t, http.StatusOK, e.do(t, http.MethodPost, "/credentials/"+org+"/c",
		CreateCredentialRequest{
			Name: "dup", Kind: "git", Secret: "tok",
		}).Code)

	stored := &models.Credential{}
	require.NoError(t, stored.GetByOrgAndName(context.Background(), e.mongo, e.ownedOrg, "dup"))
	assert.Equal(t, models.CredentialKindGit, stored.Kind)
	assert.Empty(t, stored.RegistryURL, "stale registryUrl must be cleared on cross-kind rotation")
	// git applies its username default at projection time, not on store, so
	// an unset username must persist as empty (also proves the field cleared).
	assert.Empty(t, stored.Username, "stale username must be cleared on cross-kind rotation")
	dec, _ := variables.DecryptValue(e.keyStore, stored.Secret)
	assert.Equal(t, "tok", dec)

	// And it must not leak through the metadata list either.
	listResp := e.do(t, http.MethodGet, "/credentials/"+org+"/c", nil)
	var list []CredentialResponse
	require.NoError(t, json.Unmarshal(listResp.Body.Bytes(), &list))
	require.Len(t, list, 1)
	assert.Equal(t, "git", list[0].Kind)
	assert.Empty(t, list[0].RegistryURL)
}

func TestCreateCredential_Validation(t *testing.T) {
	e := setup(t)
	org := e.ownedOrg.Hex()

	// Unknown kind.
	r1 := e.do(t, http.MethodPost, "/credentials/"+org+"/c", CreateCredentialRequest{
		Name: "x", Kind: "ssh", Secret: "s",
	})
	assert.Equal(t, http.StatusBadRequest, r1.Code)

	// registry without username.
	r2 := e.do(t, http.MethodPost, "/credentials/"+org+"/c", CreateCredentialRequest{
		Name: "x", Kind: "registry", RegistryURL: "ghcr.io", Secret: "s",
	})
	assert.Equal(t, http.StatusBadRequest, r2.Code)

	// git with no username is allowed (default applied at projection time).
	r3 := e.do(t, http.MethodPost, "/credentials/"+org+"/c", CreateCredentialRequest{
		Name: "gh", Kind: "git", Secret: "tok",
	})
	assert.Equal(t, http.StatusCreated, r3.Code, "body=%s", r3.Body.String())
}

func TestCredentials_OrgIsolationAndRBAC(t *testing.T) {
	e := setup(t)

	// Create a credential in the user's owned org.
	require.Equal(t, http.StatusCreated, e.do(t, http.MethodPost,
		"/credentials/"+e.ownedOrg.Hex()+"/c", CreateCredentialRequest{
			Name: "ghcr", Kind: "registry", RegistryURL: "ghcr.io", Username: "u", Secret: "s",
		}).Code)

	// Listing the OTHER org (owned by a different account) must be forbidden,
	// not just empty — the user has no read access to it at all.
	other := e.do(t, http.MethodGet, "/credentials/"+e.otherOrg.Hex()+"/c", nil)
	assert.Equal(t, http.StatusForbidden, other.Code)

	// Writing to the other org is forbidden too.
	w := e.do(t, http.MethodPost, "/credentials/"+e.otherOrg.Hex()+"/c", CreateCredentialRequest{
		Name: "y", Kind: "git", Secret: "s",
	})
	assert.Equal(t, http.StatusForbidden, w.Code)

	// Bad org id → 400; valid-but-missing org → 404.
	assert.Equal(t, http.StatusBadRequest,
		e.do(t, http.MethodGet, "/credentials/not-hex/c", nil).Code)
	assert.Equal(t, http.StatusNotFound,
		e.do(t, http.MethodGet, "/credentials/"+primitive.NewObjectID().Hex()+"/c", nil).Code)
}

func TestDeleteCredential(t *testing.T) {
	e := setup(t)
	org := e.ownedOrg.Hex()

	require.Equal(t, http.StatusCreated, e.do(t, http.MethodPost, "/credentials/"+org+"/c",
		CreateCredentialRequest{Name: "tmp", Kind: "git", Secret: "s"}).Code)

	del := e.do(t, http.MethodDelete, "/credentials/"+org+"/c/tmp", nil)
	assert.Equal(t, http.StatusNoContent, del.Code)

	// Gone from listing.
	listResp := e.do(t, http.MethodGet, "/credentials/"+org+"/c", nil)
	var list []CredentialResponse
	require.NoError(t, json.Unmarshal(listResp.Body.Bytes(), &list))
	assert.Len(t, list, 0)

	// Deleting a missing one → 404.
	assert.Equal(t, http.StatusNotFound,
		e.do(t, http.MethodDelete, "/credentials/"+org+"/c/tmp", nil).Code)
}
