package applications

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestListApplications(t *testing.T) {
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

	// Create test application
	app1, err := NewApplication(oID, "TestListApplications", primitive.NewObjectID().Hex()).Create(app.MongoDB)
	if err != nil {
		t.Fatal(err)
	}
	app2, err := NewApplication(oID, "TestListApplications2", primitive.NewObjectID().Hex()).Create(app.MongoDB)
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
	org := Organization{
		Status:    OrgActive,
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
	org := Organization{
		Status:    OrgActive,
		AccountID: "testOrgId",
		Name:      "Test Organization for DetailApplication",
	}
	oID, err := org.Create(app.MongoDB) // lint:ignore
	if err != nil {
		t.Fatal(err)
	}

	// Create test application
	app1, err := NewApplication(oID, "TestDetailApplication", primitive.NewObjectID().Hex()).Create(app.MongoDB)
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
