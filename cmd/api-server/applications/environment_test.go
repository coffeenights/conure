package applications

import (
	"bytes"
	"encoding/json"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCreateEnvironment(t *testing.T) {
	router, _ := setupRouter()
	createRequest := &CreateEnvironmentRequest{
		Name:           "staging",
		ApplicationID:  primitive.NewObjectID().Hex(),
		OrganizationID: "6599082303bedbfeb7243ada",
	}
	jsonData, err := json.Marshal(createRequest)
	if err != nil {
		log.Fatal(err)
	}
	request, err := http.NewRequest("POST", "/organizations/"+createRequest.OrganizationID+"/"+createRequest.ApplicationID+"/e/", bytes.NewBuffer(jsonData))
	if err != nil {
		log.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, request)
	// Assert
	if resp.Code != http.StatusCreated {
		t.Errorf("Expected response code 201, got: %v", resp.Code)
	}
}

func TestListEnvironments(t *testing.T) {
	router, _ := setupRouter()
	// Create a test environment
	createRequest := &CreateEnvironmentRequest{
		Name:           "staging-test",
		ApplicationID:  primitive.NewObjectID().Hex(),
		OrganizationID: "6599082303bedbfeb7243ada",
	}
	jsonData, err := json.Marshal(createRequest)
	if err != nil {
		log.Fatal(err)
	}
	request, err := http.NewRequest("POST", "/organizations/"+createRequest.OrganizationID+"/"+createRequest.ApplicationID+"/e/", bytes.NewBuffer(jsonData))
	if err != nil {
		log.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, request)
	// Assert
	if resp.Code != http.StatusCreated {
		t.Errorf("Expected response code 201, got: %v", resp.Code)
	}

	// List environments
	request, err = http.NewRequest("GET", "/organizations/"+createRequest.OrganizationID+"/"+createRequest.ApplicationID+"/e/", nil)
	if err != nil {
		log.Fatal(err)
	}
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, request)
	// Assert
	if resp.Code != http.StatusOK {
		t.Errorf("Expected response code 200, got: %v", resp.Code)
	}

	var response EnvironmentListResponse
	err = json.Unmarshal(resp.Body.Bytes(), &response)
	if err != nil {
		log.Fatal(err)
	}
	if len(response.Environments) != 1 {
		t.Errorf("Expected 1 environment, got: %v", len(response.Environments))
	}
	if response.Environments[0].Name != "staging-test" {
		t.Errorf("Expected environment to be staging-test, got: %v", response.Environments[0].Name)
	}
}
