package applications

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/coffeenights/conure/cmd/api-server/models"
)

func TestListComponents(t *testing.T) {
	// Create test organization
	org := models.Organization{
		Status:    models.OrgActive,
		AccountID: testConf.authUser.ID,
		Name:      "Test Organization for ListComponents",
	}
	oID, err := org.Create(testConf.app.MongoDB) // lint:ignore
	if err != nil {
		t.Fatal(err)
	}
	defer org.Delete(testConf.app.MongoDB)

	// Create test application
	application, err := models.NewApplication(oID, "TestListComponents", testConf.authUser.ID.Hex()).Create(testConf.app.MongoDB)
	if err != nil {
		t.Fatal(err)
	}
	defer application.Delete(testConf.app.MongoDB)

	_, err = application.CreateEnvironment(testConf.app.MongoDB, "staging")
	if err != nil {
		t.Fatal(err)
	}

	comp1 := models.Component{
		ApplicationID: application.ID,
		Name:          "test-component-list",
		Type:          "service",
	}
	err = comp1.Create(testConf.app.MongoDB)
	if err != nil {
		t.Errorf("Failed to create component: %v", err)
	}
	defer comp1.Delete(testConf.app.MongoDB)

	comp2 := models.Component{
		ApplicationID: application.ID,
		Name:          "test-component2-list",
		Type:          "service",
	}
	err = comp2.Create(testConf.app.MongoDB)
	if err != nil {
		t.Errorf("Failed to create component: %v", err)
	}
	defer comp2.Delete(testConf.app.MongoDB)

	url := "/organizations/" + oID + "/a/" + application.ID.Hex() + "/e/" + application.Environments[0].Name + "/c"
	req, _ := http.NewRequest("GET", url, nil)
	req.AddCookie(testConf.generateCookie())
	resp := httptest.NewRecorder()
	testConf.router.ServeHTTP(resp, req)

	// Assert
	if resp.Code != http.StatusOK {
		t.Errorf("Expected response code 200, got: %v", resp.Code)
	}
	var response ComponentListResponse
	err = json.Unmarshal(resp.Body.Bytes(), &response)
	if err != nil {
		t.Errorf("Failed to unmarshal response: %v", err)
	}
	if len(response.Components) != 2 {
		t.Errorf("Expected 2 component, got: %v", len(response.Components))
	}
	if response.Components[0].Name != "test-component-list" {
		t.Errorf("Expected component name to be test-component-list, got: %v", response.Components[0].Name)
	}
	if response.Components[1].Name != "test-component2-list" {
		t.Errorf("Expected component name to be test-component2-list, got: %v", response.Components[1].Name)
	}
}

func TestListComponents_NotExist(t *testing.T) {

	// Create test organization
	org := models.Organization{
		Status:    models.OrgActive,
		AccountID: testConf.authUser.ID,
		Name:      "Test Organization for ListApplications",
	}
	oID, err := org.Create(testConf.app.MongoDB) // lint:ignore
	if err != nil {
		t.Fatal(err)
	}
	defer org.Delete(testConf.app.MongoDB)

	url := "/organizations/" + oID + "/a/" + primitive.NewObjectID().Hex() + "/e/test-env/c"
	req, _ := http.NewRequest("GET", url, nil)
	req.AddCookie(testConf.generateCookie())
	resp := httptest.NewRecorder()
	testConf.router.ServeHTTP(resp, req)

	// Assert
	if resp.Code != http.StatusNotFound {
		t.Errorf("Expected response code 404, got: %v", resp.Code)
	}
}

func TestListComponents_Empty(t *testing.T) {

	// Create test organization
	org := models.Organization{
		Status:    models.OrgActive,
		AccountID: testConf.authUser.ID,
		Name:      "Test Organization for ListApplications",
	}
	oID, err := org.Create(testConf.app.MongoDB) // lint:ignore
	if err != nil {
		t.Fatal(err)
	}
	defer org.Delete(testConf.app.MongoDB)

	// Create test application
	application, err := models.NewApplication(oID, "TestListComponents", testConf.authUser.ID.Hex()).Create(testConf.app.MongoDB)
	if err != nil {
		t.Fatal(err)
	}
	defer application.Delete(testConf.app.MongoDB)

	_, err = application.CreateEnvironment(testConf.app.MongoDB, "staging")
	if err != nil {
		t.Fatal(err)
	}

	url := "/organizations/" + oID + "/a/" + application.ID.Hex() + "/e/" + application.Environments[0].Name + "/c"
	req, _ := http.NewRequest("GET", url, nil)
	req.AddCookie(testConf.generateCookie())
	resp := httptest.NewRecorder()
	testConf.router.ServeHTTP(resp, req)

	// Assert
	if resp.Code != http.StatusOK {
		t.Errorf("Expected response code 200, got: %v", resp.Code)
	}
	var response ComponentListResponse
	err = json.Unmarshal(resp.Body.Bytes(), &response)
	if err != nil {
		t.Errorf("Failed to unmarshal response: %v", err)
	}
	if len(response.Components) != 0 {
		t.Errorf("Expected 0 component, got: %v", len(response.Components))
	}
}

func TestCreateComponent(t *testing.T) {
	// Create test organization
	org := models.Organization{
		Status:    models.OrgActive,
		AccountID: testConf.authUser.ID,
		Name:      "Test Organization for ListApplications",
	}
	oID, err := org.Create(testConf.app.MongoDB) // lint:ignore
	if err != nil {
		t.Fatal(err)
	}
	defer org.Delete(testConf.app.MongoDB)

	// Create test application
	application, err := models.NewApplication(oID, "TestListComponents", testConf.authUser.ID.Hex()).Create(testConf.app.MongoDB)
	if err != nil {
		t.Fatal(err)
	}
	defer application.Delete(testConf.app.MongoDB)

	env, err := application.CreateEnvironment(testConf.app.MongoDB, "staging")
	if err != nil {
		t.Fatal(err)
	}

	url := "/organizations/" + oID + "/a/" + application.ID.Hex() + "/e/" + env.Name + "/c"
	body := map[string]interface{}{
		"type":        "service",
		"name":        "test-component",
		"description": "Test component description",
		"properties": map[string]interface{}{
			"image": "nginx:latest",
			"port":  "80",
			"cpu":   "100m",
		},
	}
	payload, err := json.Marshal(body)
	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(payload))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(testConf.generateCookie())
	resp := httptest.NewRecorder()
	testConf.router.ServeHTTP(resp, req)

	// Assert
	if resp.Code != http.StatusCreated {
		t.Errorf("Expected response code 201, got: %v", resp.Code)
		t.FailNow()
	}
	var response ComponentResponse
	err = json.Unmarshal(resp.Body.Bytes(), &response)
	if err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}
	if response.Component.Name != "test-component" {
		t.Errorf("Expected component name to be test-component, got: %v", response.Component.Name)
	}
	// Clean up Component
	comp := models.Component{
		Model: models.Model{
			ID: response.ID,
		},
	}
	_ = comp.Delete(testConf.app.MongoDB)
}

func TestDetailComponent(t *testing.T) {
	// Create test organization
	org := models.Organization{
		Status:    models.OrgActive,
		AccountID: testConf.authUser.ID,
		Name:      "Test Organization for ListApplications",
	}
	oID, err := org.Create(testConf.app.MongoDB) // lint:ignore
	if err != nil {
		t.Fatal(err)
	}
	defer org.Delete(testConf.app.MongoDB)

	// Create test application
	application, err := models.NewApplication(oID, "TestListComponents", testConf.authUser.ID.Hex()).Create(testConf.app.MongoDB)
	if err != nil {
		t.Fatal(err)
	}
	defer application.Delete(testConf.app.MongoDB)

	env, err := application.CreateEnvironment(testConf.app.MongoDB, "staging")
	if err != nil {
		t.Fatal(err)
	}

	comp := models.Component{
		ApplicationID: application.ID,
		Name:          "test-detail-component",
		Type:          "service",
	}
	err = comp.Create(testConf.app.MongoDB)
	if err != nil {
		t.Errorf("Failed to create component: %v", err)
	}
	defer comp.Delete(testConf.app.MongoDB)

	url := "/organizations/" + oID + "/a/" + application.ID.Hex() + "/e/" + env.Name + "/c/" + comp.ID.Hex()
	req, _ := http.NewRequest("GET", url, nil)
	req.AddCookie(testConf.generateCookie())
	resp := httptest.NewRecorder()
	testConf.router.ServeHTTP(resp, req)

	// Assert
	if resp.Code != http.StatusOK {
		t.Errorf("Expected response code 200, got: %v", resp.Code)
	}
	var response ComponentResponse
	err = json.Unmarshal(resp.Body.Bytes(), &response)
	if err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}
	if response.Name != "test-detail-component" {
		t.Errorf("Expected component name to be test-detail-component, got: %v", response.Name)
	}
}

func TestDetailComponent_NotFound(t *testing.T) {
	// Create test organization
	org := models.Organization{
		Status:    models.OrgActive,
		AccountID: testConf.authUser.ID,
		Name:      "Test Organization for ListApplications",
	}
	oID, err := org.Create(testConf.app.MongoDB) // lint:ignore
	if err != nil {
		t.Fatal(err)
	}
	defer org.Delete(testConf.app.MongoDB)

	// Create test application
	application, err := models.NewApplication(oID, "TestDetailComponents_NotFound", testConf.authUser.ID.Hex()).Create(testConf.app.MongoDB)
	if err != nil {
		t.Fatal(err)
	}
	defer application.Delete(testConf.app.MongoDB)

	env, err := application.CreateEnvironment(testConf.app.MongoDB, "staging")
	if err != nil {
		t.Fatal(err)
	}

	url := "/organizations/" + oID + "/a/" + application.ID.Hex() + "/e/" + env.Name + "/c/asdasd"
	req, _ := http.NewRequest("GET", url, nil)
	req.AddCookie(testConf.generateCookie())
	resp := httptest.NewRecorder()
	testConf.router.ServeHTTP(resp, req)

	// Assert
	if resp.Code != http.StatusNotFound {
		t.Errorf("Expected response code 404, got: %v", resp.Code)
	}
}
