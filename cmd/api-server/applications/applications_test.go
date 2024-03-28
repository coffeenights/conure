package applications

import (
	"bytes"
	"encoding/json"
	"github.com/coffeenights/conure/cmd/api-server/models"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestListApplications(t *testing.T) {
	router, app := setupRouter()
	// Create test organization
	org := models.Organization{
		Status:    models.OrgActive,
		AccountID: "testOrgId",
		Name:      "Test Organization for ListApplications",
	}
	oID, err := org.Create(app.MongoDB) // lint:ignore
	if err != nil {
		t.Fatal(err)
	}

	// Create test application
	app1, err := models.NewApplication(oID, "TestListApplications", primitive.NewObjectID().Hex()).Create(app.MongoDB)
	if err != nil {
		t.Fatal(err)
	}
	app2, err := models.NewApplication(oID, "TestListApplications2", primitive.NewObjectID().Hex()).Create(app.MongoDB)
	if err != nil {
		t.Fatal(err)
	}
	_, err = app1.CreateEnvironment(app.MongoDB, "TestListApplications")
	if err != nil {
		t.Fatal(err)
	}
	_, err = app2.CreateEnvironment(app.MongoDB, "TestListApplications")
	if err != nil {
		t.Fatal(err)
	}
	url := "/organizations/" + oID + "/a/"
	req, _ := http.NewRequest("GET", url, nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	// Assert
	if resp.Code != http.StatusOK {
		t.Errorf("Expected response code 200, got: %v", resp.Code)
	}
	_ = app1.Delete(app.MongoDB)
	_ = app2.Delete(app.MongoDB)
	_ = org.Delete(app.MongoDB)
}

func TestListApplications_Empty(t *testing.T) {
	router, app := setupRouter()
	// Create test organization
	org := models.Organization{
		Status:    models.OrgActive,
		AccountID: "testOrgId",
		Name:      "Test Organization for ListApplications_Empty",
	}
	oID, err := org.Create(app.MongoDB) // lint:ignore
	if err != nil {
		t.Fatal(err)
	}
	url := "/organizations/" + oID + "/a/"
	req, _ := http.NewRequest("GET", url, nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	// Assert
	if resp.Code != http.StatusOK {
		t.Errorf("Expected response code 200, got: %v", resp.Code)
	}
	_ = org.Delete(app.MongoDB)
}

func TestDetailApplication(t *testing.T) {
	router, app := setupRouter()
	// Create test organization
	org := models.Organization{
		Status:    models.OrgActive,
		AccountID: "testOrgId",
		Name:      "Test Organization for DetailApplication",
	}
	oID, err := org.Create(app.MongoDB) // lint:ignore
	if err != nil {
		t.Fatal(err)
	}

	// Create test application
	app1, err := models.NewApplication(oID, "TestDetailApplication", primitive.NewObjectID().Hex()).Create(app.MongoDB)
	if err != nil {
		t.Fatal(err)
	}
	envName := "test-detail-application"
	_, err = app1.CreateEnvironment(app.MongoDB, envName)
	if err != nil {
		t.Fatal(err)
	}
	url := "/organizations/" + oID + "/a/" + app1.ID.Hex() + "/e/" + envName + "/"
	req, _ := http.NewRequest("GET", url, nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	// Assert
	if resp.Code != http.StatusOK {
		t.Errorf("Expected response code 200, got: %v", resp.Code)
	}
	_ = app1.Delete(app.MongoDB)
	_ = org.Delete(app.MongoDB)
}

func TestCreateApplication(t *testing.T) {
	router, app := setupRouter()
	// Create test organization
	org := models.Organization{
		Status:    models.OrgActive,
		AccountID: "testOrgId",
		Name:      "Test Organization for CreateApplication",
	}
	oID, err := org.Create(app.MongoDB) // lint:ignore
	if err != nil {
		t.Fatal(err)
	}
	url := "/organizations/" + oID + "/a/"
	body := map[string]interface{}{
		"name": "TestCreateApplication",
	}
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}

	bodyReader := bytes.NewBuffer(bodyBytes)
	req, _ := http.NewRequest("POST", url, bodyReader)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	// Assert
	if resp.Code != http.StatusCreated {
		t.Errorf("Expected response code 201, got: %v", resp.Code)
	}
	_ = org.Delete(app.MongoDB)
}

func TestCreateApplication_Empty(t *testing.T) {
	router, app := setupRouter()
	// Create test organization
	org := models.Organization{
		Status:    models.OrgActive,
		AccountID: "testOrgId",
		Name:      "Test Organization for CreateApplication",
	}
	oID, err := org.Create(app.MongoDB) // lint:ignore
	if err != nil {
		t.Fatal(err)
	}
	url := "/organizations/" + oID + "/a/"
	req, _ := http.NewRequest("POST", url, nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	// Assert
	if resp.Code != http.StatusBadRequest {
		t.Errorf("Expected response code 400, got: %v", resp.Code)
	}
	_ = org.Delete(app.MongoDB)
}
