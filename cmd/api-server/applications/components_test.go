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

// orgWithApp seeds an org + app + (optionally) one environment, returning
// everything tests need to drive HTTP requests.
func orgWithApp(t *testing.T, name, envName string) (*models.Organization, *models.Application, *models.Environment) {
	t.Helper()
	org := &models.Organization{
		Status:    models.OrgActive,
		AccountID: testConf.authUser.ID,
		Name:      "Org for " + name,
	}
	if _, err := org.Create(testConf.app.MongoDB); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = org.Delete(testConf.app.MongoDB) })

	app, err := models.NewApplication(org.ID.Hex(), name, testConf.authUser.ID.Hex()).Create(testConf.app.MongoDB)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = app.Delete(testConf.app.MongoDB) })

	if envName == "" {
		return org, app, nil
	}
	env, err := app.CreateEnvironment(testConf.app.MongoDB, envName)
	if err != nil {
		t.Fatal(err)
	}
	return org, app, env
}

func cleanupComponent(t *testing.T, comp *models.Component) {
	t.Helper()
	t.Cleanup(func() {
		_, _ = models.DeleteAllRevisionsForComponent(context.Background(), testConf.app.MongoDB, comp.ID)
		_ = comp.Delete(testConf.app.MongoDB)
	})
}

func doJSON(t *testing.T, method, url string, body interface{}) *httptest.ResponseRecorder {
	t.Helper()
	var reader *bytes.Buffer
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		reader = bytes.NewBuffer(payload)
	} else {
		reader = bytes.NewBuffer(nil)
	}
	req, _ := http.NewRequest(method, url, reader)
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(testConf.generateCookie())
	resp := httptest.NewRecorder()
	testConf.router.ServeHTTP(resp, req)
	return resp
}

// seededDef is a test-local description of a component definition to write
// into the org-scoped Mongo source of truth.
type seededDef struct {
	compType string
	engine   string
}

func def(compType, engine string) seededDef { return seededDef{compType, engine} }

// withComponentDefs seeds the org-scoped MongoDB component definitions for one
// org for the duration of a test. Definitions are no longer cluster-scoped
// CRDs read live: they are org-scoped Mongo rows (the source of truth) that
// the API resolves and materializes into the cluster at deploy time. Engine
// and field-role resolution on component create read the same rows, so tests
// seed them here per org. Rows are cleaned up after the test.
func withComponentDefs(t *testing.T, org *models.Organization, defs ...seededDef) {
	t.Helper()
	for _, d := range defs {
		row := &models.ComponentDefinition{
			OrganizationID: org.ID,
			Type:           d.compType,
			Engine:         d.engine,
			OCIRepository:  "example.test/" + d.compType + "-" + d.engine,
			OCITag:         "0.1.0",
		}
		if _, err := row.Create(context.Background(), testConf.app.MongoDB); err != nil {
			t.Fatalf("seed component definition %s/%s: %v", d.compType, d.engine, err)
		}
		rowID := row.ID
		t.Cleanup(func() {
			d := &models.ComponentDefinition{}
			d.SetID(rowID)
			_ = d.Delete(context.Background(), testConf.app.MongoDB)
		})
	}
}

func TestCreateComponent_AppWide(t *testing.T) {
	org, app, env := orgWithApp(t, "TestCreateComponent_AppWide", "staging")

	url := "/organizations/" + org.ID.Hex() + "/a/" + app.ID.Hex() + "/c"
	body := map[string]interface{}{
		"name":        "test-component",
		"type":        "service",
		"description": "Test component description",
		"environment": env.Name,
		"values":      models.ComponentRevisionValuesTemplate(),
	}
	resp := doJSON(t, "POST", url, body)
	if resp.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d (%s)", resp.Code, resp.Body.String())
	}

	var raw struct {
		Component *models.Component         `json:"component"`
		Revision  *models.ComponentRevision `json:"revision"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &raw); err != nil {
		t.Fatalf("decoding response: %v", err)
	}
	if raw.Component == nil || raw.Component.Name != "test-component" {
		t.Fatalf("unexpected component: %+v", raw.Component)
	}
	if raw.Revision == nil || raw.Revision.Status != models.RevisionStatusDraft || raw.Revision.Version != 1 {
		t.Fatalf("expected v1 draft, got %+v", raw.Revision)
	}
	cleanupComponent(t, raw.Component)
}

// TestCreateComponent_ResolvesEngineFromSingleDef confirms that when exactly
// one ComponentDefinition matches the requested type, the API persists that
// definition's engine on the Component identity even though the request
// didn't pin one. This is the common "user just picks a type" path.
func TestCreateComponent_ResolvesEngineFromSingleDef(t *testing.T) {
	org, app, env := orgWithApp(t, "TestCreateComponent_ResolvesEngineFromSingleDef", "staging")
	withComponentDefs(t, org, def("webservice", "timoni"))

	url := "/organizations/" + org.ID.Hex() + "/a/" + app.ID.Hex() + "/c"
	resp := doJSON(t, "POST", url, map[string]interface{}{
		"name":        "infer-engine",
		"type":        "webservice",
		"environment": env.Name,
		"values":      map[string]interface{}{},
	})
	if resp.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d (%s)", resp.Code, resp.Body.String())
	}
	var raw struct {
		Component *models.Component `json:"component"`
	}
	_ = json.Unmarshal(resp.Body.Bytes(), &raw)
	if raw.Component == nil || raw.Component.Engine != "timoni" {
		t.Fatalf("expected Engine=timoni, got %+v", raw.Component)
	}
	cleanupComponent(t, raw.Component)
}

// TestCreateComponent_AmbiguousEngineIsRejected covers the multi-engine
// failure path: two definitions for the same type with no engine on the
// request must return ErrAmbiguousComponentEngine, not silently pick one.
func TestCreateComponent_AmbiguousEngineIsRejected(t *testing.T) {
	org, app, env := orgWithApp(t, "TestCreateComponent_AmbiguousEngineIsRejected", "staging")
	withComponentDefs(t, org,
		def("webservice", "timoni"),
		def("webservice", "helm"),
	)

	url := "/organizations/" + org.ID.Hex() + "/a/" + app.ID.Hex() + "/c"
	resp := doJSON(t, "POST", url, map[string]interface{}{
		"name":        "ambiguous",
		"type":        "webservice",
		"environment": env.Name,
		"values":      map[string]interface{}{},
	})
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d (%s)", resp.Code, resp.Body.String())
	}
	// The API surfaces the conureerrors code, not the message — the CLI
	// translates to a user-friendly string. Just check the code.
	var body struct {
		Code string `json:"code"`
	}
	_ = json.Unmarshal(resp.Body.Bytes(), &body)
	if body.Code != "4006" {
		t.Fatalf("expected error code 4006, got %q (full=%s)", body.Code, resp.Body.String())
	}
}

// TestCreateComponent_PinnedEnginePicksMatch verifies that when the request
// pins an engine and two definitions exist, the persisted Component is
// tagged with the requested engine — proving the disambiguation knob works.
func TestCreateComponent_PinnedEnginePicksMatch(t *testing.T) {
	org, app, env := orgWithApp(t, "TestCreateComponent_PinnedEnginePicksMatch", "staging")
	withComponentDefs(t, org,
		def("webservice", "timoni"),
		def("webservice", "helm"),
	)

	url := "/organizations/" + org.ID.Hex() + "/a/" + app.ID.Hex() + "/c"
	resp := doJSON(t, "POST", url, map[string]interface{}{
		"name":        "pinned-helm",
		"type":        "webservice",
		"engine":      "helm",
		"environment": env.Name,
		"values":      map[string]interface{}{},
	})
	if resp.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d (%s)", resp.Code, resp.Body.String())
	}
	var raw struct {
		Component *models.Component `json:"component"`
	}
	_ = json.Unmarshal(resp.Body.Bytes(), &raw)
	if raw.Component == nil || raw.Component.Engine != "helm" {
		t.Fatalf("expected Engine=helm, got %+v", raw.Component)
	}
	cleanupComponent(t, raw.Component)
}

// TestCreateComponent_PinnedEngineWithoutMatchingDef rejects requests that
// pin an engine no registered definition implements for the type.
func TestCreateComponent_PinnedEngineWithoutMatchingDef(t *testing.T) {
	org, app, env := orgWithApp(t, "TestCreateComponent_PinnedEngineWithoutMatchingDef", "staging")
	withComponentDefs(t, org, def("webservice", "timoni"))

	url := "/organizations/" + org.ID.Hex() + "/a/" + app.ID.Hex() + "/c"
	resp := doJSON(t, "POST", url, map[string]interface{}{
		"name":        "no-such-engine",
		"type":        "webservice",
		"engine":      "helm",
		"environment": env.Name,
		"values":      map[string]interface{}{},
	})
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d (%s)", resp.Code, resp.Body.String())
	}
	var body struct {
		Code string `json:"code"`
	}
	_ = json.Unmarshal(resp.Body.Bytes(), &body)
	if body.Code != "4007" {
		t.Fatalf("expected error code 4007, got %q (full=%s)", body.Code, resp.Body.String())
	}
}

func TestListComponents_AppWide(t *testing.T) {
	org, app, env := orgWithApp(t, "TestListComponents_AppWide", "staging")

	for _, name := range []string{"comp-a", "comp-b"} {
		url := "/organizations/" + org.ID.Hex() + "/a/" + app.ID.Hex() + "/c"
		resp := doJSON(t, "POST", url, map[string]interface{}{
			"name":        name,
			"type":        "service",
			"environment": env.Name,
			"values":      map[string]interface{}{},
		})
		if resp.Code != http.StatusCreated {
			t.Fatalf("seeding %s: expected 201, got %d", name, resp.Code)
		}
		var raw struct {
			Component *models.Component `json:"component"`
		}
		_ = json.Unmarshal(resp.Body.Bytes(), &raw)
		cleanupComponent(t, raw.Component)
	}

	url := "/organizations/" + org.ID.Hex() + "/a/" + app.ID.Hex() + "/c"
	resp := doJSON(t, "GET", url, nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.Code)
	}
	var listResp ComponentListResponse
	if err := json.Unmarshal(resp.Body.Bytes(), &listResp); err != nil {
		t.Fatal(err)
	}
	if len(listResp.Components) != 2 {
		t.Fatalf("expected 2 components, got %d", len(listResp.Components))
	}
	for _, c := range listResp.Components {
		if len(c.Environments) == 0 {
			t.Errorf("component %s missing environment presence rollup", c.Name)
		}
	}
}

func TestGetComponent_AppWide(t *testing.T) {
	org, app, env := orgWithApp(t, "TestGetComponent_AppWide", "staging")

	createURL := "/organizations/" + org.ID.Hex() + "/a/" + app.ID.Hex() + "/c"
	resp := doJSON(t, "POST", createURL, map[string]interface{}{
		"name":        "the-component",
		"type":        "service",
		"environment": env.Name,
		"values":      map[string]interface{}{"foo": "bar"},
	})
	if resp.Code != http.StatusCreated {
		t.Fatalf("create: expected 201, got %d", resp.Code)
	}
	var created struct {
		Component *models.Component `json:"component"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	cleanupComponent(t, created.Component)

	url := "/organizations/" + org.ID.Hex() + "/a/" + app.ID.Hex() + "/c/" + created.Component.ID.Hex()
	resp = doJSON(t, "GET", url, nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.Code)
	}
	var got ComponentResponse
	if err := json.Unmarshal(resp.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Component == nil || got.Component.Name != "the-component" {
		t.Fatalf("unexpected component %+v", got.Component)
	}
	if len(got.Environments) != 1 {
		t.Fatalf("expected 1 env presence, got %d", len(got.Environments))
	}
	if !got.Environments[0].HasDraft || got.Environments[0].LatestDraftVersion != 1 {
		t.Errorf("expected v1 draft presence, got %+v", got.Environments[0])
	}
}

func TestGetComponent_NotInApp(t *testing.T) {
	org, app, _ := orgWithApp(t, "TestGetComponent_NotInApp", "staging")

	other := &models.Component{
		Name:          "other-comp",
		Type:          "service",
		ApplicationID: primitive.NewObjectID(),
	}
	if err := other.Create(testConf.app.MongoDB); err != nil {
		t.Fatal(err)
	}
	cleanupComponent(t, other)

	url := "/organizations/" + org.ID.Hex() + "/a/" + app.ID.Hex() + "/c/" + other.ID.Hex()
	resp := doJSON(t, "GET", url, nil)
	if resp.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.Code)
	}
}

func TestPromoteComponent(t *testing.T) {
	org, app, staging := orgWithApp(t, "TestPromoteComponent", "staging")
	prod, err := app.CreateEnvironment(testConf.app.MongoDB, "production")
	if err != nil {
		t.Fatal(err)
	}

	// Seed identity + a deployed v1 in staging.
	component := &models.Component{
		Name:          "promoted",
		Type:          "service",
		ApplicationID: app.ID,
	}
	if err := component.Create(testConf.app.MongoDB); err != nil {
		t.Fatal(err)
	}
	cleanupComponent(t, component)
	deployed := &models.ComponentRevision{
		ComponentID:   component.ID,
		EnvironmentID: staging.ID,
		Values:        map[string]interface{}{"replicas": float64(2)},
	}
	if err := deployed.CreateDeployed(context.Background(), testConf.app.MongoDB); err != nil {
		t.Fatal(err)
	}

	url := "/organizations/" + org.ID.Hex() + "/a/" + app.ID.Hex() + "/c/" + component.ID.Hex() + "/promote"
	resp := doJSON(t, "POST", url, map[string]string{"from": staging.Name, "to": prod.Name})
	if resp.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d (%s)", resp.Code, resp.Body.String())
	}
	var rev models.ComponentRevision
	if err := json.Unmarshal(resp.Body.Bytes(), &rev); err != nil {
		t.Fatal(err)
	}
	if rev.EnvironmentID != prod.ID || rev.Status != models.RevisionStatusDraft || rev.Version != 1 {
		t.Fatalf("unexpected promoted rev: %+v", rev)
	}
	if got, ok := rev.Values["replicas"].(float64); !ok || got != 2 {
		t.Errorf("promoted values not copied: %+v", rev.Values)
	}
}

func TestPromoteComponent_NoDeployedSource(t *testing.T) {
	org, app, staging := orgWithApp(t, "TestPromote_NoSource", "staging")
	prod, err := app.CreateEnvironment(testConf.app.MongoDB, "production")
	if err != nil {
		t.Fatal(err)
	}

	component := &models.Component{
		Name:          "no-source",
		Type:          "service",
		ApplicationID: app.ID,
	}
	if err := component.Create(testConf.app.MongoDB); err != nil {
		t.Fatal(err)
	}
	cleanupComponent(t, component)
	// Only a draft exists in staging; promote should refuse it (deployed-only).
	draft := &models.ComponentRevision{
		ComponentID:   component.ID,
		EnvironmentID: staging.ID,
	}
	if err := draft.CreateDraft(context.Background(), testConf.app.MongoDB); err != nil {
		t.Fatal(err)
	}

	url := "/organizations/" + org.ID.Hex() + "/a/" + app.ID.Hex() + "/c/" + component.ID.Hex() + "/promote"
	resp := doJSON(t, "POST", url, map[string]string{"from": staging.Name, "to": prod.Name})
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.Code)
	}
}

func TestCreateAndUpdateDraftRevision(t *testing.T) {
	org, app, env := orgWithApp(t, "TestCreateUpdateDraftRev", "staging")

	component := &models.Component{
		Name:          "rev-target",
		Type:          "service",
		ApplicationID: app.ID,
	}
	if err := component.Create(testConf.app.MongoDB); err != nil {
		t.Fatal(err)
	}
	cleanupComponent(t, component)

	createURL := "/organizations/" + org.ID.Hex() + "/a/" + app.ID.Hex() + "/e/" + env.Name + "/c/" + component.ID.Hex() + "/revisions"
	resp := doJSON(t, "POST", createURL, CreateRevisionRequest{Values: map[string]interface{}{"image": "v1"}})
	if resp.Code != http.StatusCreated {
		t.Fatalf("create rev: expected 201, got %d (%s)", resp.Code, resp.Body.String())
	}
	var rev models.ComponentRevision
	if err := json.Unmarshal(resp.Body.Bytes(), &rev); err != nil {
		t.Fatal(err)
	}
	if rev.Version != 1 || rev.Status != models.RevisionStatusDraft {
		t.Fatalf("unexpected rev: %+v", rev)
	}

	updateURL := createURL + "/" + rev.ID.Hex()
	resp = doJSON(t, "PUT", updateURL, UpdateRevisionRequest{Values: map[string]interface{}{"image": "v2"}})
	if resp.Code != http.StatusOK {
		t.Fatalf("update rev: expected 200, got %d (%s)", resp.Code, resp.Body.String())
	}
	var updated models.ComponentRevision
	if err := json.Unmarshal(resp.Body.Bytes(), &updated); err != nil {
		t.Fatal(err)
	}
	if got := updated.Values["image"]; got != "v2" {
		t.Errorf("expected image=v2 after update, got %v", got)
	}
}

func TestUpdateDraftRevision_RejectsDeployed(t *testing.T) {
	org, app, env := orgWithApp(t, "TestUpdateDraft_RejectsDeployed", "staging")

	component := &models.Component{
		Name:          "immutable",
		Type:          "service",
		ApplicationID: app.ID,
	}
	if err := component.Create(testConf.app.MongoDB); err != nil {
		t.Fatal(err)
	}
	cleanupComponent(t, component)

	deployed := &models.ComponentRevision{
		ComponentID:   component.ID,
		EnvironmentID: env.ID,
		Values:        map[string]interface{}{"image": "v1"},
	}
	if err := deployed.CreateDeployed(context.Background(), testConf.app.MongoDB); err != nil {
		t.Fatal(err)
	}

	url := "/organizations/" + org.ID.Hex() + "/a/" + app.ID.Hex() + "/e/" + env.Name + "/c/" + component.ID.Hex() + "/revisions/" + deployed.ID.Hex()
	resp := doJSON(t, "PUT", url, UpdateRevisionRequest{Values: map[string]interface{}{"image": "v2"}})
	if resp.Code != http.StatusForbidden && resp.Code != http.StatusNotFound {
		t.Fatalf("expected 403 or 404, got %d", resp.Code)
	}
}

func TestListRevisions(t *testing.T) {
	org, app, env := orgWithApp(t, "TestListRevisions", "staging")

	component := &models.Component{
		Name:          "lister",
		Type:          "service",
		ApplicationID: app.ID,
	}
	if err := component.Create(testConf.app.MongoDB); err != nil {
		t.Fatal(err)
	}
	cleanupComponent(t, component)

	for i := 0; i < 3; i++ {
		rev := &models.ComponentRevision{
			ComponentID:   component.ID,
			EnvironmentID: env.ID,
			Values:        map[string]interface{}{"v": i},
		}
		if err := rev.CreateDeployed(context.Background(), testConf.app.MongoDB); err != nil {
			t.Fatal(err)
		}
	}

	url := "/organizations/" + org.ID.Hex() + "/a/" + app.ID.Hex() + "/e/" + env.Name + "/c/" + component.ID.Hex() + "/revisions"
	resp := doJSON(t, "GET", url, nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.Code)
	}
	var listResp ComponentRevisionListResponse
	if err := json.Unmarshal(resp.Body.Bytes(), &listResp); err != nil {
		t.Fatal(err)
	}
	if len(listResp.Revisions) != 3 {
		t.Fatalf("expected 3 revisions, got %d", len(listResp.Revisions))
	}
	if listResp.Revisions[0].Version != 3 {
		t.Errorf("expected newest first (v3), got %d", listResp.Revisions[0].Version)
	}
}
