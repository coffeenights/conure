package applications

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/coffeenights/conure/cmd/api-server/models"
)

// helper: create org/app/component/env and return identifiers usable in
// build URLs. Doesn't deploy a CRD — the read-only build endpoints don't
// need k8s.
func seedBuildContext(t *testing.T) (orgID, appID, env, componentID string, cleanup func()) {
	t.Helper()
	org := models.Organization{
		Status:    models.OrgActive,
		AccountID: testConf.authUser.ID,
		Name:      "Org for build tests " + primitive.NewObjectID().Hex(),
	}
	oID, err := org.Create(testConf.app.MongoDB)
	if err != nil {
		t.Fatal(err)
	}
	app, err := models.NewApplication(oID, "BuildTestApp", primitive.NewObjectID().Hex()).Create(testConf.app.MongoDB)
	if err != nil {
		t.Fatal(err)
	}
	app.AccountID = testConf.authUser.ID
	_ = app.Update(testConf.app.MongoDB)
	envObj, err := app.CreateEnvironment(testConf.app.MongoDB, "dev")
	if err != nil {
		t.Fatal(err)
	}
	component := &models.Component{
		Name:          "build-test-component",
		Type:          "service",
		ApplicationID: app.ID,
	}
	if err := component.Create(testConf.app.MongoDB); err != nil {
		t.Fatal(err)
	}
	return oID, app.ID.Hex(), envObj.Name, component.ID.Hex(), func() {
		_ = app.Delete(testConf.app.MongoDB)
		_ = org.Delete(testConf.app.MongoDB)
	}
}

func TestListBuilds_EmptyReturnsEmptySlice(t *testing.T) {
	orgID, appID, env, componentID, cleanup := seedBuildContext(t)
	defer cleanup()

	url := "/organizations/" + orgID + "/a/" + appID + "/e/" + env + "/c/" + componentID + "/builds"
	req, _ := http.NewRequest(http.MethodGet, url, nil)
	req.AddCookie(testConf.generateCookie())
	resp := httptest.NewRecorder()
	testConf.router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status: %d body=%s", resp.Code, resp.Body.String())
	}
	var got BuildListResponse
	if err := json.Unmarshal(resp.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Builds == nil {
		t.Errorf("response should return [] not null")
	}
	if len(got.Builds) != 0 {
		t.Errorf("expected empty list, got %d", len(got.Builds))
	}
}

func TestListBuilds_NewestFirst(t *testing.T) {
	orgID, appID, env, componentID, cleanup := seedBuildContext(t)
	defer cleanup()
	ctx := context.Background()

	componentOID, _ := primitive.ObjectIDFromHex(componentID)
	appOID, _ := primitive.ObjectIDFromHex(appID)

	// Build an env-ID lookup by reading the app back.
	app := &models.Application{}
	if err := app.GetByID(testConf.app.MongoDB, appID); err != nil {
		t.Fatal(err)
	}
	envID := ""
	for _, e := range app.Environments {
		if e.Name == env {
			envID = e.ID
		}
	}
	if envID == "" {
		t.Fatal("env not found on app")
	}

	for i := 0; i < 2; i++ {
		b := &models.Build{
			ComponentID:   componentOID,
			ApplicationID: appOID,
			EnvironmentID: envID,
			Status:        models.BuildStatusSucceeded,
			BuildTool:     models.BuildToolDockerfile,
			BuildLocation: models.BuildLocationLocal,
			ImageRef:      "ghcr.io/test/img:t" + primitive.NewObjectID().Hex(),
		}
		if err := models.Create(ctx, testConf.app.MongoDB, b); err != nil {
			t.Fatal(err)
		}
	}

	url := "/organizations/" + orgID + "/a/" + appID + "/e/" + env + "/c/" + componentID + "/builds"
	req, _ := http.NewRequest(http.MethodGet, url, nil)
	req.AddCookie(testConf.generateCookie())
	resp := httptest.NewRecorder()
	testConf.router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status: %d body=%s", resp.Code, resp.Body.String())
	}
	var got BuildListResponse
	_ = json.Unmarshal(resp.Body.Bytes(), &got)
	if len(got.Builds) != 2 {
		t.Fatalf("expected 2 builds got %d", len(got.Builds))
	}
	if !got.Builds[0].CreatedAt.After(got.Builds[1].CreatedAt) && !got.Builds[0].CreatedAt.Equal(got.Builds[1].CreatedAt) {
		t.Errorf("expected newest-first ordering")
	}
}

func TestGetBuild_WrongComponentReturns404(t *testing.T) {
	orgID, appID, env, componentID, cleanup := seedBuildContext(t)
	defer cleanup()
	ctx := context.Background()

	// Create a build for a different component.
	other := primitive.NewObjectID()
	appOID, _ := primitive.ObjectIDFromHex(appID)
	b := &models.Build{
		ComponentID:   other,
		ApplicationID: appOID,
		EnvironmentID: "any",
		Status:        models.BuildStatusSucceeded,
		BuildTool:     models.BuildToolDockerfile,
		BuildLocation: models.BuildLocationLocal,
		ImageRef:      "x:1",
	}
	if err := models.Create(ctx, testConf.app.MongoDB, b); err != nil {
		t.Fatal(err)
	}
	url := "/organizations/" + orgID + "/a/" + appID + "/e/" + env + "/c/" + componentID + "/builds/" + b.ID.Hex()
	req, _ := http.NewRequest(http.MethodGet, url, nil)
	req.AddCookie(testConf.generateCookie())
	resp := httptest.NewRecorder()
	testConf.router.ServeHTTP(resp, req)
	if resp.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.Code)
	}
}

func TestTriggerBuild_RemoteRailpackRejected(t *testing.T) {
	orgID, appID, env, componentID, cleanup := seedBuildContext(t)
	defer cleanup()

	body, _ := json.Marshal(TriggerBuildRequest{
		BuildTool:     "railpack",
		BuildLocation: "remote",
		GitRepository: "https://example.com/repo",
		GitBranch:     "main",
		ImageRef:      "ghcr.io/x/y:tag",
	})
	url := "/organizations/" + orgID + "/a/" + appID + "/e/" + env + "/c/" + componentID + "/builds"
	req, _ := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(testConf.generateCookie())
	resp := httptest.NewRecorder()
	testConf.router.ServeHTTP(resp, req)
	if resp.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for remote railpack, got %d body=%s", resp.Code, resp.Body.String())
	}
}

func TestTriggerBuild_RemoteWithoutGitRejected(t *testing.T) {
	orgID, appID, env, componentID, cleanup := seedBuildContext(t)
	defer cleanup()

	body, _ := json.Marshal(TriggerBuildRequest{
		BuildTool:     "dockerfile",
		BuildLocation: "remote",
		ImageRef:      "ghcr.io/x/y:tag",
	})
	url := "/organizations/" + orgID + "/a/" + appID + "/e/" + env + "/c/" + componentID + "/builds"
	req, _ := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(testConf.generateCookie())
	resp := httptest.NewRecorder()
	testConf.router.ServeHTTP(resp, req)
	if resp.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for remote without git, got %d body=%s", resp.Code, resp.Body.String())
	}
}
