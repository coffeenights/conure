package applications

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"

	"github.com/coffeenights/conure/cmd/api-server/auth"
	"github.com/coffeenights/conure/cmd/api-server/models"
)

// createUserFresh ensures no stale row from a previous (possibly aborted)
// run blocks the new insert. Mongo's email uniqueness is what bites us,
// and the applications suite reuses one DB across runs.
func createUserFresh(t *testing.T, u *models.User) {
	t.Helper()
	coll := testConf.app.MongoDB.Client.Database(testConf.app.MongoDB.DBName).Collection(models.UserCollection)
	_, _ = coll.DeleteMany(context.Background(), bson.M{"email": u.Email})
	if err := u.Create(testConf.app.MongoDB); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = u.Delete(testConf.app.MongoDB) })
}

// TestDeveloperOrgMembership_ReadAllowed_WriteForbidden verifies the core
// developer rule: a developer in an organization can read peers' apps but
// cannot mutate them.
func TestDeveloperOrgMembership_ReadAllowed_WriteForbidden(t *testing.T) {
	// Org is owned by the test superuser; developer Bob is provisioned as
	// a member of it.
	org := models.Organization{
		Status:    models.OrgActive,
		AccountID: testConf.authUser.ID,
		Name:      "ReadAllowedWriteForbidden Org",
	}
	if _, err := org.Create(testConf.app.MongoDB); err != nil {
		t.Fatal(err)
	}

	// Owner creates an application — Bob did not create it.
	app, err := models.NewApplication(org.ID.Hex(), "owner-app", testConf.authUser.ID.Hex()).Create(testConf.app.MongoDB)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := app.CreateEnvironment(testConf.app.MongoDB, "staging"); err != nil {
		t.Fatal(err)
	}

	// Bob is a developer in the same org. The applications test suite shares
	// one Mongo DB across tests and TestMain doesn't drop it between runs,
	// so we register an explicit cleanup to keep this test idempotent.
	hashed, _ := auth.GenerateFromPassword("Password123")
	bob := &models.User{
		Email:          "bob@test.io",
		Password:       hashed,
		Role:           models.RoleDeveloper,
		OrganizationID: org.ID,
	}
	createUserFresh(t, bob)
	bobToken, _ := auth.GenerateToken(time.Hour, auth.JWTData{Email: bob.Email}, testConf.app.Config.JWTSecret)
	bobCookie := &http.Cookie{Name: "auth", Value: bobToken, Path: "/"}

	// READ: Bob can list org applications.
	req, _ := http.NewRequest(http.MethodGet, "/organizations/"+org.ID.Hex()+"/a", nil)
	req.AddCookie(bobCookie)
	w := httptest.NewRecorder()
	testConf.router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("developer should read org apps, got %d (%s)", w.Code, w.Body.String())
	}

	// READ: Bob can view the owner-created app's detail.
	req, _ = http.NewRequest(http.MethodGet, "/organizations/"+org.ID.Hex()+"/a/"+app.ID.Hex(), nil)
	req.AddCookie(bobCookie)
	w = httptest.NewRecorder()
	testConf.router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("developer should read app detail in their org, got %d (%s)", w.Code, w.Body.String())
	}

	// WRITE: Bob cannot create an environment on someone else's app.
	body := bytes.NewBufferString(`{"name":"prod"}`)
	req, _ = http.NewRequest(http.MethodPost, "/organizations/"+org.ID.Hex()+"/a/"+app.ID.Hex()+"/e", body)
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(bobCookie)
	w = httptest.NewRecorder()
	testConf.router.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Errorf("developer must not mutate someone else's app, got %d (%s)", w.Code, w.Body.String())
	}
}

// TestDeveloperOutsideOrg_CannotRead asserts that a developer whose home
// org doesn't match can't peek into apps in another organization.
func TestDeveloperOutsideOrg_CannotRead(t *testing.T) {
	org := models.Organization{
		Status:    models.OrgActive,
		AccountID: testConf.authUser.ID,
		Name:      "OutsiderTest Org",
	}
	if _, err := org.Create(testConf.app.MongoDB); err != nil {
		t.Fatal(err)
	}

	hashed, _ := auth.GenerateFromPassword("Password123")
	stranger := &models.User{
		Email:    "stranger@test.io",
		Password: hashed,
		Role:     models.RoleDeveloper,
		// No OrganizationID — not a member of any org.
	}
	createUserFresh(t, stranger)
	tok, _ := auth.GenerateToken(time.Hour, auth.JWTData{Email: stranger.Email}, testConf.app.Config.JWTSecret)
	cookie := &http.Cookie{Name: "auth", Value: tok, Path: "/"}

	req, _ := http.NewRequest(http.MethodGet, "/organizations/"+org.ID.Hex(), nil)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	testConf.router.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Errorf("non-member developer must not read the org, got %d (%s)", w.Code, w.Body.String())
	}
}
