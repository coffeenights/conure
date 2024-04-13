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

	// Create test application
	app1, err := models.NewApplication(oID, "TestListApplications", primitive.NewObjectID().Hex()).Create(testConf.app.MongoDB)
	if err != nil {
		t.Fatal(err)
	}
	app2, err := models.NewApplication(oID, "TestListApplications2", primitive.NewObjectID().Hex()).Create(testConf.app.MongoDB)
	if err != nil {
		t.Fatal(err)
	}
	_, err = app1.CreateEnvironment(testConf.app.MongoDB, "TestListApplications")
	if err != nil {
		t.Fatal(err)
	}
	_, err = app2.CreateEnvironment(testConf.app.MongoDB, "TestListApplications")
	if err != nil {
		t.Fatal(err)
	}
	url := "/organizations/" + oID + "/a/"
	req, _ := http.NewRequest("GET", url, nil)
	req.AddCookie(testConf.generateCookie())
	resp := httptest.NewRecorder()
	testConf.router.ServeHTTP(resp, req)

	// Assert
	if resp.Code != http.StatusOK {
		t.Errorf("Expected response code 200, got: %v", resp.Code)
	}
	_ = app1.Delete(testConf.app.MongoDB)
	_ = app2.Delete(testConf.app.MongoDB)
	_ = org.Delete(testConf.app.MongoDB)
}

func TestListApplications_Empty(t *testing.T) {
	// Create test organization
	org := models.Organization{
		Status:    models.OrgActive,
		AccountID: testConf.authUser.ID,
		Name:      "Test Organization for ListApplications_Empty",
	}
	oID, err := org.Create(testConf.app.MongoDB) // lint:ignore
	if err != nil {
		t.Fatal(err)
	}
	url := "/organizations/" + oID + "/a/"
	req, _ := http.NewRequest("GET", url, nil)
	req.AddCookie(testConf.generateCookie())
	resp := httptest.NewRecorder()
	testConf.router.ServeHTTP(resp, req)

	// Assert
	if resp.Code != http.StatusOK {
		t.Errorf("Expected response code 200, got: %v", resp.Code)
	}
	_ = org.Delete(testConf.app.MongoDB)
}

func TestDetailApplication(t *testing.T) {
	// Create test organization
	org := models.Organization{
		Status:    models.OrgActive,
		AccountID: testConf.authUser.ID,
		Name:      "Test Organization for DetailApplication",
	}
	oID, err := org.Create(testConf.app.MongoDB) // lint:ignore
	if err != nil {
		t.Fatal(err)
	}

	// Create test application
	app1, err := models.NewApplication(oID, "TestDetailApplication", testConf.authUser.ID.Hex()).Create(testConf.app.MongoDB)
	if err != nil {
		t.Fatal(err)
	}
	envName := "test-detail-application"
	_, err = app1.CreateEnvironment(testConf.app.MongoDB, envName)
	if err != nil {
		t.Fatal(err)
	}
	url := "/organizations/" + oID + "/a/" + app1.ID.Hex() + "/e/" + envName + "/"
	req, _ := http.NewRequest("GET", url, nil)
	req.AddCookie(testConf.generateCookie())
	resp := httptest.NewRecorder()
	testConf.router.ServeHTTP(resp, req)

	// Assert
	if resp.Code != http.StatusOK {
		t.Errorf("Expected response code 200, got: %v", resp.Code)
	}
	_ = app1.Delete(testConf.app.MongoDB)
	_ = org.Delete(testConf.app.MongoDB)
}

func TestDetailApplication_NotFound(t *testing.T) {
	// Create test organization
	org := models.Organization{
		Status:    models.OrgActive,
		AccountID: testConf.authUser.ID,
		Name:      "Test Organization for DetailApplication_NotFound",
	}
	oID, err := org.Create(testConf.app.MongoDB) // lint:ignore
	if err != nil {
		t.Fatal(err)
	}
	url := "/organizations/" + oID + "/a/" + primitive.NewObjectID().Hex() + "/e/test-detail-application/"
	req, _ := http.NewRequest("GET", url, nil)
	req.AddCookie(testConf.generateCookie())
	resp := httptest.NewRecorder()
	testConf.router.ServeHTTP(resp, req)

	// Assert
	if resp.Code != http.StatusNotFound {
		t.Errorf("Expected response code 404, got: %v", resp.Code)
	}
	_ = org.Delete(testConf.app.MongoDB)

}

func TestCreateApplication(t *testing.T) {
	// Create test organization
	org := models.Organization{
		Status:    models.OrgActive,
		AccountID: testConf.authUser.ID,
		Name:      "Test Organization for CreateApplication",
	}
	oID, err := org.Create(testConf.app.MongoDB) // lint:ignore
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
	req.AddCookie(testConf.generateCookie())
	resp := httptest.NewRecorder()
	testConf.router.ServeHTTP(resp, req)

	// Assert
	if resp.Code != http.StatusCreated {
		t.Errorf("Expected response code 201, got: %v", resp.Code)
	}
	_ = org.Delete(testConf.app.MongoDB)
}

func TestCreateApplication_Empty(t *testing.T) {
	// Create test organization
	org := models.Organization{
		Status:    models.OrgActive,
		AccountID: testConf.authUser.ID,
		Name:      "Test Organization for CreateApplication",
	}
	oID, err := org.Create(testConf.app.MongoDB) // lint:ignore
	if err != nil {
		t.Fatal(err)
	}
	url := "/organizations/" + oID + "/a/"
	req, _ := http.NewRequest("POST", url, nil)
	req.AddCookie(testConf.generateCookie())
	resp := httptest.NewRecorder()
	testConf.router.ServeHTTP(resp, req)

	// Assert
	if resp.Code != http.StatusBadRequest {
		t.Errorf("Expected response code 400, got: %v", resp.Code)
	}
	_ = org.Delete(testConf.app.MongoDB)
}
