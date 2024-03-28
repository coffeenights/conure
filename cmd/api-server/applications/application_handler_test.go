package applications

import (
	"github.com/coffeenights/conure/cmd/api-server/models"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"testing"
)

func TestListOrganizationApplications(t *testing.T) {
	client, err := models.SetupDB()
	if err != nil {
		t.Fatal(err)
	}
	orgID := primitive.NewObjectID()
	app1, err := models.NewApplication(orgID.Hex(), "TestListOrganizationApplications1", primitive.NewObjectID().Hex()).Create(client)
	if err != nil {
		t.Fatal(err)
	}
	app2, err := models.NewApplication(orgID.Hex(), "TestListOrganizationApplications2", primitive.NewObjectID().Hex()).Create(client)
	if err != nil {
		t.Fatal(err)
	}
	apps, err := ListOrganizationApplications(orgID.Hex(), client)
	if err != nil {
		t.Errorf("Failed to list applications: %v", err)
	}
	if len(apps) != 2 {
		t.Errorf("Got %d applications, want == 2", len(apps))
	}
	err = app1.Delete(client)
	if err != nil {
		t.Errorf("Failed to delete application: %v", err)
	}
	err = app2.Delete(client)
	if err != nil {
		t.Errorf("Failed to delete application: %v", err)
	}
}

func TestListOrganizationApplications_NotFound(t *testing.T) {
	client, err := models.SetupDB()
	if err != nil {
		t.Fatal(err)
	}
	l, err := ListOrganizationApplications(primitive.NewObjectID().Hex(), client)
	if err != nil {
		t.Errorf("Failed to list applications: %v", err)
	}
	if len(l) != 0 {
		t.Errorf("Expected error, got nil")
	}
}
