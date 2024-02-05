package applications

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetOrganization(t *testing.T) {
	router, app := setupRouter()
	// Create test organization
	org := Organization{
		Status:    OrgActive,
		AccountID: "testOrgId",
		Name:      "Test Organization",
	}
	oID, err := org.Create(app.MongoDB) // lint:ignore
	if err != nil {
		t.Fatal(err)
	}
	url := "/organizations/" + oID
	req, _ := http.NewRequest("GET", url, nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	// Assert
	if resp.Code != http.StatusOK {
		t.Errorf("Expected response code 200, got: %v", resp.Code)
	}
}

func TestCreateOrganization(t *testing.T) {
	router, app := setupRouter()

	createRequest := &CreateOrganizationRequest{
		Name:      "POST Test Organization",
		AccountID: "333455",
	}
	jsonData, err := json.Marshal(createRequest)
	if err != nil {
		log.Fatal(err)
	}
	request, err := http.NewRequest("POST", "/organizations/", bytes.NewBuffer(jsonData))
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
	org := Organization{}
	response := OrganizationResponse{}
	err = json.Unmarshal(resp.Body.Bytes(), &response)
	if err != nil {
		log.Fatal(err)
	}
	_, err = org.GetById(app.MongoDB, response.ID)
	if err != nil {
		log.Fatal(err)
	}
}
