package applications

import (
	"bytes"
	"encoding/json"
	"github.com/coffeenights/conure/cmd/api-server/models"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCreateEnvironment(t *testing.T) {
	createRequest := &CreateEnvironmentRequest{
		Name: "staging",
	}
	orgID := primitive.NewObjectID().Hex()
	app, err := models.NewApplication(orgID, "test-app", primitive.NewObjectID().Hex()).Create(testConf.app.MongoDB)
	if err != nil {
		t.Fatalf("Failed to create application: %v", err)
	}
	defer app.Delete(testConf.app.MongoDB)
	jsonData, err := json.Marshal(createRequest)
	if err != nil {
		log.Fatalf("Failed to marshal request: %v", err)
	}
	request, err := http.NewRequest("POST", "/organizations/"+orgID+"/a/"+app.ID.Hex()+"/e/", bytes.NewBuffer(jsonData))
	if err != nil {
		log.Fatalf("Failed to create request: %v", err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.AddCookie(testConf.generateCookie())
	resp := httptest.NewRecorder()
	testConf.router.ServeHTTP(resp, request)
	if resp.Code != http.StatusCreated {
		t.Errorf("Expected response code 201, got: %v", resp.Code)
	}
	err = app.GetByID(testConf.app.MongoDB, app.ID.Hex())
	if err != nil {
		t.Errorf("Failed to get application: %v", err)
	}
	if len(app.Environments) != 1 {
		t.Errorf("Expected 1 environment, got: %v", len(app.Environments))
	}
}

func TestCreateEnvironment_NotExist(t *testing.T) {
	createRequest := &CreateEnvironmentRequest{
		Name: "staging",
	}
	orgID := primitive.NewObjectID().Hex()
	appID := primitive.NewObjectID().Hex()
	jsonData, err := json.Marshal(createRequest)
	if err != nil {
		log.Fatalf("Failed to marshal request: %v", err)
	}
	request, err := http.NewRequest("POST", "/organizations/"+orgID+"/a/"+appID+"/e/", bytes.NewBuffer(jsonData))
	if err != nil {
		log.Fatalf("Failed to create request: %v", err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.AddCookie(testConf.generateCookie())
	resp := httptest.NewRecorder()
	testConf.router.ServeHTTP(resp, request)
	if resp.Code != http.StatusNotFound {
		t.Errorf("Expected response code 404, got: %v", resp.Code)
	}
}

func TestDeleteEnvironment(t *testing.T) {
	orgID := primitive.NewObjectID().Hex()
	app, err := models.NewApplication(orgID, "test-app", primitive.NewObjectID().Hex()).Create(testConf.app.MongoDB)
	if err != nil {
		t.Fatalf("Failed to create application: %v", err)
	}
	defer app.Delete(testConf.app.MongoDB)
	env, err := app.CreateEnvironment(testConf.app.MongoDB, "staging")
	if err != nil {
		t.Fatalf("Failed to create environment: %v", err)
	}
	request, err := http.NewRequest("DELETE", "/organizations/"+orgID+"/a/"+app.ID.Hex()+"/e/"+env.Name+"/", nil)
	if err != nil {
		log.Fatalf("Failed to create request: %v", err)
	}
	request.AddCookie(testConf.generateCookie())
	resp := httptest.NewRecorder()
	testConf.router.ServeHTTP(resp, request)
	if resp.Code != http.StatusOK {
		t.Errorf("Expected response code 200, got: %v", resp.Code)
	}
	err = app.GetByID(testConf.app.MongoDB, app.ID.Hex())
	if err != nil {
		t.Errorf("Failed to get application: %v", err)
	}
	if len(app.Environments) != 0 {
		t.Errorf("Expected 0 environments, got: %v", len(app.Environments))
	}
}
