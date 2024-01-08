package applications

import (
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

//func TestCreateOrganization(t *testing.T) {
//	// Setup
//	gin.SetMode(gin.TestMode)
//	router := gin.Default()
//	router.POST("/organizations", CreateOrganization)
//
//	// Prepare request body
//	org := Organization{
//		// Fill in fields as needed
//	}
//	body, _ := json.Marshal(org)
//	req, _ := http.NewRequest("POST", "/organizations", bytes.NewBuffer(body))
//	req.Header.Set("Content-Type", "application/json")
//
//	// Execute
//	resp := httptest.NewRecorder()
//	router.ServeHTTP(resp, req)
//
//	// Assert
//	if resp.Code != 200 {
//		t.Errorf("Expected response code 200, got: %v", resp.Code)
//	}
//}
//
//func TestListEnvironments(t *testing.T) {
//	// Setup
//	gin.SetMode(gin.TestMode)
//	router := gin.Default()
//	router.GET("/environments", ListEnvironments)
//
//	// Execute
//	req, _ := http.NewRequest("GET", "/environments", nil)
//	resp := httptest.NewRecorder()
//	router.ServeHTTP(resp, req)
//
//	// Assert
//	if resp.Code != 200 {
//		t.Errorf("Expected response code 200, got: %v", resp.Code)
//	}
//}
