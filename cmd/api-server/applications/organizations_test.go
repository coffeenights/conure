package applications

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/coffeenights/conure/cmd/api-server/models"
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
		t.FailNow()
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

func TestListOrganization(t *testing.T) {
	// Create test organization
	org := models.Organization{
		Status:    models.OrgActive,
		AccountID: testConf.authUser.ID,
		Name:      "Test Organization",
	}
	_, err := org.Create(testConf.app.MongoDB)
	if err != nil {
		t.Fatal(err)
	}
	url := "/organizations/"
	req, _ := http.NewRequest("GET", url, nil)
	req.AddCookie(testConf.generateCookie())
	resp := httptest.NewRecorder()
	testConf.router.ServeHTTP(resp, req)

	// Assert
	if resp.Code != http.StatusOK {
		t.Errorf("Expected response code 200, got: %v", resp.Code)
	}

	orgs := OrganizationListResponse{}
	err = json.Unmarshal(resp.Body.Bytes(), &orgs)
	if err != nil {
		t.Fatal(err)
	}
	if len(orgs.Organizations) == 0 {
		t.Error("Expected at least one organization")
	}
	if orgs.Organizations[0].Name != "Test Organization" {
		t.Errorf("Expected organization name to be 'Test Organization', got: %v", orgs.Organizations[0].Name)
	}

	_ = org.Delete(testConf.app.MongoDB)
}
