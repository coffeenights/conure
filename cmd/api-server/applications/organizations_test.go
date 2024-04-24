package applications

import (
	"bytes"
	"encoding/json"
	"github.com/coffeenights/conure/cmd/api-server/models"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDetailOrganization(t *testing.T) {
	// Create test organization
	org := models.Organization{
		Status:    models.OrgActive,
		AccountID: testConf.authUser.ID,
		Name:      "Test Organization",
	}
	oID, err := org.Create(testConf.app.MongoDB) // lint:ignore
	if err != nil {
		t.Fatal(err)
	}
	url := "/organizations/" + oID
	req, _ := http.NewRequest("GET", url, nil)
	req.AddCookie(testConf.generateCookie())
	resp := httptest.NewRecorder()
	testConf.router.ServeHTTP(resp, req)

	// Assert
	if resp.Code != http.StatusOK {
		t.Errorf("Expected response code 200, got: %v", resp.Code)
	}
}

func TestCreateOrganization(t *testing.T) {
	createRequest := &CreateOrganizationRequest{
		Name: "POST Test Organization",
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
	request.AddCookie(testConf.generateCookie())
	resp := httptest.NewRecorder()
	testConf.router.ServeHTTP(resp, request)
	// Assert
	if resp.Code != http.StatusCreated {
		t.Errorf("Expected response code 201, got: %v", resp.Code)
	}
	org := models.Organization{}
	response := OrganizationResponse{}
	err = json.Unmarshal(resp.Body.Bytes(), &response)
	if err != nil {
		log.Fatal(err)
	}
	_, err = org.GetById(testConf.app.MongoDB, response.ID.Hex())
	if err != nil {
		log.Fatal(err)
	}
}
