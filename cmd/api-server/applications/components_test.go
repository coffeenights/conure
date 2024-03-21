package applications

import (
	"bytes"
	"encoding/json"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestListComponents(t *testing.T) {
	router, app := setupRouter()
	// Create test organization
	org := Organization{
		Status:    OrgActive,
		AccountID: "testOrgId",
		Name:      "Test Organization for ListApplications",
	}
	oID, err := org.Create(app.MongoDB) // lint:ignore
	if err != nil {
		t.Fatal(err)
	}
	defer org.Delete(app.MongoDB)

	// Create test application
	application, err := NewApplication(oID, "TestListComponents", primitive.NewObjectID().Hex()).Create(app.MongoDB)
	if err != nil {
		t.Fatal(err)
	}
	defer application.Delete(app.MongoDB)

	_, err = application.CreateEnvironment(app.MongoDB, "staging")
	if err != nil {
		t.Fatal(err)
	}

	comp, err := NewComponent(application, "TestListComponents", "service").Create(app.MongoDB)
	if err != nil {
		t.Errorf("Failed to create component: %v", err)
	}
	defer comp.Delete(app.MongoDB)

	url := "/organizations/" + oID + "/a/" + application.ID.Hex() + "/e/" + application.Environments[0].Name + "/c/"
	req, _ := http.NewRequest("GET", url, nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	// Assert
	if resp.Code != http.StatusOK {
		t.Errorf("Expected response code 200, got: %v", resp.Code)
	}
	var response ComponentListResponse
	err = json.Unmarshal(resp.Body.Bytes(), &response)
	if err != nil {
		t.Errorf("Failed to unmarshal response: %v", err)
	}
	if len(response.Components) != 1 {
		t.Errorf("Expected 1 component, got: %v", len(response.Components))
	}

}

func TestListComponents_NotExist(t *testing.T) {
	router, app := setupRouter()

	// Create test organization
	org := Organization{
		Status:    OrgActive,
		AccountID: "testOrgId",
		Name:      "Test Organization for ListApplications",
	}
	oID, err := org.Create(app.MongoDB) // lint:ignore
	if err != nil {
		t.Fatal(err)
	}
	defer org.Delete(app.MongoDB)

	url := "/organizations/" + oID + "/a/" + primitive.NewObjectID().Hex() + "/e/test-env/c/"
	req, _ := http.NewRequest("GET", url, nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	// Assert
	if resp.Code != http.StatusNotFound {
		t.Errorf("Expected response code 404, got: %v", resp.Code)
	}
}

func TestListComponents_Empty(t *testing.T) {
	router, app := setupRouter()

	// Create test organization
	org := Organization{
		Status:    OrgActive,
		AccountID: "testOrgId",
		Name:      "Test Organization for ListApplications",
	}
	oID, err := org.Create(app.MongoDB) // lint:ignore
	if err != nil {
		t.Fatal(err)
	}
	defer org.Delete(app.MongoDB)

	// Create test application
	application, err := NewApplication(oID, "TestListComponents", primitive.NewObjectID().Hex()).Create(app.MongoDB)
	if err != nil {
		t.Fatal(err)
	}
	defer application.Delete(app.MongoDB)

	_, err = application.CreateEnvironment(app.MongoDB, "staging")
	if err != nil {
		t.Fatal(err)
	}

	url := "/organizations/" + oID + "/a/" + application.ID.Hex() + "/e/" + application.Environments[0].Name + "/c/"
	req, _ := http.NewRequest("GET", url, nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

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
	router, app := setupRouter()

	// Create test organization
	org := Organization{
		Status:    OrgActive,
		AccountID: "testOrgId",
		Name:      "Test Organization for ListApplications",
	}
	oID, err := org.Create(app.MongoDB) // lint:ignore
	if err != nil {
		t.Fatal(err)
	}
	defer org.Delete(app.MongoDB)

	// Create test application
	application, err := NewApplication(oID, "TestListComponents", primitive.NewObjectID().Hex()).Create(app.MongoDB)
	if err != nil {
		t.Fatal(err)
	}
	defer application.Delete(app.MongoDB)

	env, err := application.CreateEnvironment(app.MongoDB, "staging")
	if err != nil {
		t.Fatal(err)
	}

	url := "/organizations/" + oID + "/a/" + application.ID.Hex() + "/e/" + env.Name + "/c/"
	body := map[string]interface{}{
		"name":        "TestComponent",
		"type":        "service",
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
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	// Assert
	if resp.Code != http.StatusCreated {
		t.Errorf("Expected response code 201, got: %v", resp.Code)
	}
	var response ComponentResponse
	err = json.Unmarshal(resp.Body.Bytes(), &response)
	if err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}
	if response.ID != "TestComponent" {
		t.Errorf("Expected component name to be TestComponent, got: %v", response.ID)
	}
	// Clean up Component
	comp := Component{
		ID: response.ID,
	}
	_ = comp.Delete(app.MongoDB)
}

func TestDetailComponent(t *testing.T) {
	router, app := setupRouter()

	// Create test organization
	org := Organization{
		Status:    OrgActive,
		AccountID: "testOrgId",
		Name:      "Test Organization for ListApplications",
	}
	oID, err := org.Create(app.MongoDB) // lint:ignore
	if err != nil {
		t.Fatal(err)
	}
	defer org.Delete(app.MongoDB)

	// Create test application
	application, err := NewApplication(oID, "TestListComponents", primitive.NewObjectID().Hex()).Create(app.MongoDB)
	if err != nil {
		t.Fatal(err)
	}
	defer application.Delete(app.MongoDB)

	env, err := application.CreateEnvironment(app.MongoDB, "staging")
	if err != nil {
		t.Fatal(err)
	}

	comp, err := NewComponent(application, "TestDetailComponent", "service").Create(app.MongoDB)
	if err != nil {
		t.Errorf("Failed to create component: %v", err)
	}
	defer comp.Delete(app.MongoDB)

	url := "/organizations/" + oID + "/a/" + application.ID.Hex() + "/e/" + env.Name + "/c/" + comp.ID
	req, _ := http.NewRequest("GET", url, nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	// Assert
	if resp.Code != http.StatusOK {
		t.Errorf("Expected response code 200, got: %v", resp.Code)
	}
	var response ComponentResponse
	err = json.Unmarshal(resp.Body.Bytes(), &response)
	if err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}
	if response.ID != "TestDetailComponent" {
		t.Errorf("Expected component name to be TestDetailComponent, got: %v", response.ID)
	}
}
