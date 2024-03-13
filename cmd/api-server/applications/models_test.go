package applications

import (
	_ "github.com/joho/godotenv/autoload"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"testing"
)

func TestOrganization_Create(t *testing.T) {
	client, err := setupDB()
	if err != nil {
		t.Fatal(err)
	}

	org := &Organization{Status: OrgActive, AccountID: "12345", Name: "Test Organization"}

	_, err = org.Create(client)
	if err != nil {
		t.Errorf("Failed to create organization: %v", err)
	}
}

func TestOrganization_GetById(t *testing.T) {
	client, err := setupDB()
	if err != nil {
		t.Fatal(err)
	}

	org := &Organization{Status: OrgActive, AccountID: "12345"}
	id, err := org.Create(client)
	if err != nil {
		t.Fatal(err)
	}

	got, _ := org.GetById(client, id)
	if got.AccountID != org.AccountID {
		t.Errorf("Got %v, want %v", got.AccountID, org.AccountID)
	}
}

func TestOrganization_Update(t *testing.T) {
	client, err := setupDB()
	if err != nil {
		t.Fatal(err)
	}

	org := &Organization{Status: OrgActive, AccountID: "12345"}
	id, err := org.Create(client)
	if err != nil {
		t.Fatal(err)
	}

	org.Status = OrgDisabled
	err = org.Update(client)
	if err != nil {
		t.Errorf("Failed to update organization: %v", err)
	}

	got, err := org.GetById(client, id)
	if got.Status != OrgDisabled {
		t.Errorf("Got %v, want %v", got.Status, OrgDisabled)
	}
}

func TestOrganization_Delete(t *testing.T) {
	client, err := setupDB()
	if err != nil {
		t.Fatal(err)
	}

	org := &Organization{Status: OrgActive, AccountID: "12345"}
	_, err = org.Create(client)
	if err != nil {
		t.Fatal(err)
	}

	err = org.Delete(client)
	if err != nil {
		t.Errorf("Failed to delete organization: %v", err)
	}

	_, err = org.GetById(client, org.ID.Hex())
	if err == nil {
		t.Errorf("Got 1 document, want 0")
	}
}

func TestOrganization_SoftDelete(t *testing.T) {
	client, err := setupDB()
	if err != nil {
		t.Fatal(err)
	}

	org := &Organization{Status: OrgActive, AccountID: "12345"}
	_, err = org.Create(client)
	if err != nil {
		t.Fatal(err)
	}

	err = org.SoftDelete(client)
	if err != nil {
		t.Errorf("Failed to soft delete organization: %v", err)
	}

	_, err = org.GetById(client, org.ID.Hex())
	if err == nil {
		t.Errorf("Got 1 document, want 0")
	}
	err = org.Delete(client)
	if err != nil {
		t.Errorf("Failed to delete organization: %v", err)
	}
}

func TestApplication_Create(t *testing.T) {
	client, err := setupDB()
	if err != nil {
		t.Fatal(err)
	}
	app, err := NewApplication(primitive.NewObjectID().Hex(), "TestApplicationCreate", primitive.NewObjectID().Hex()).Create(client)
	if err != nil {
		t.Errorf("Failed to create application: %v", err)
	}
	got, _ := app.GetByID(client, app.ID.Hex())
	if got.Name != app.Name {
		t.Errorf("Got %v, want %v", got.Name, app.Name)
	}
	err = app.Delete(client)
	if err != nil {
		t.Errorf("Failed to delete application: %v", err)
	}
}

func TestApplication_GetById(t *testing.T) {
	client, err := setupDB()
	if err != nil {
		t.Fatal(err)
	}
	app, err := NewApplication(primitive.NewObjectID().Hex(), "TestApplicationGetById", primitive.NewObjectID().Hex()).Create(client)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := app.GetByID(client, app.ID.Hex())
	if got.Name != app.Name {
		t.Errorf("Got %v, want %v", got.Name, app.Name)
	}
	err = app.Delete(client)
	if err != nil {
		t.Errorf("Failed to delete application: %v", err)
	}
}

func TestApplication_Update(t *testing.T) {
	client, err := setupDB()
	if err != nil {
		t.Fatal(err)
	}
	app, err := NewApplication(primitive.NewObjectID().Hex(), "TestApplicationGetById", primitive.NewObjectID().Hex()).Create(client)
	if err != nil {
		t.Fatal(err)
	}
	app.Name = "Updated Application"
	err = app.Update(client)
	if err != nil {
		t.Errorf("Failed to update application: %v", err)
	}
	got, _ := app.GetByID(client, app.ID.Hex())
	if got.Name != app.Name {
		t.Errorf("Got %v, want %v", got.Name, app.Name)
	}
	err = app.Delete(client)
	if err != nil {
		t.Errorf("Failed to delete application: %v", err)
	}
}

func TestApplication_SoftDelete(t *testing.T) {
	client, err := setupDB()
	if err != nil {
		t.Fatal(err)
	}
	app, err := NewApplication(primitive.NewObjectID().Hex(), "TestApplicationSoftDelete", primitive.NewObjectID().Hex()).Create(client)
	if err != nil {
		t.Fatal(err)
	}
	err = app.SoftDelete(client)
	if err != nil {
		t.Errorf("Failed to soft delete application: %v", err)
	}
	_, err = app.GetByID(client, app.ID.Hex())
	if err == nil {
		t.Errorf("Got 1 document, want 0")
	}
	err = app.Delete(client)
	if err != nil {
		t.Errorf("Failed to delete application: %v", err)
	}
}

func Test_ApplicationList(t *testing.T) {
	client, err := setupDB()
	if err != nil {
		t.Fatal(err)
	}
	orgID := primitive.NewObjectID()
	app1, err := NewApplication(orgID.Hex(), "TestApplicationList1", primitive.NewObjectID().Hex()).Create(client)
	if err != nil {
		t.Fatal(err)
	}
	app2, err := NewApplication(orgID.Hex(), "TestApplicationList2", primitive.NewObjectID().Hex()).Create(client)
	if err != nil {
		t.Fatal(err)
	}
	apps, err := ApplicationList(client, orgID.Hex())
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

func TestApplication_ListNotDeleted(t *testing.T) {
	client, err := setupDB()
	if err != nil {
		t.Fatal(err)
	}

	orgID := primitive.NewObjectID()
	app1, err := NewApplication(orgID.Hex(), "TestApplicationListNotDeleted1", primitive.NewObjectID().Hex()).Create(client)
	if err != nil {
		t.Fatal(err)
	}
	app2, err := NewApplication(orgID.Hex(), "TestApplicationListNotDeleted2", primitive.NewObjectID().Hex()).Create(client)
	if err != nil {
		t.Fatal(err)
	}

	err = app1.SoftDelete(client)
	if err != nil {
		t.Errorf("Failed to soft delete application: %v", err)
	}
	apps, err := ApplicationList(client, orgID.Hex())
	if err != nil {
		t.Errorf("Failed to list applications: %v", err)
	}
	if len(apps) != 1 {
		t.Errorf("Got %d applications, want = 1", len(apps))
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

func TestComponent_CreateList(t *testing.T) {
	client, err := setupDB()
	if err != nil {
		t.Fatal(err)
	}
	app, err := NewApplication(primitive.NewObjectID().Hex(), "TestApplicationSoftDelete", primitive.NewObjectID().Hex()).Create(client)
	if err != nil {
		t.Fatal(err)
	}
	comp := NewComponent(app, "testComponent", "service")
	comp.Properties = map[string]interface{}{
		"cpu":      "1",
		"memory":   "1Gi",
		"replicas": int32(1),
	}
	err = comp.Create(client)
	if err != nil {
		t.Errorf("Failed to create component: %v", err)
	}
	comps, err := app.ListComponents(client)
	if err != nil {
		t.Errorf("Failed to list components: %v", err)
	}
	if len(comps) == 0 {
		t.Errorf("Got 0 components, want > 0")
	}
	err = app.Delete(client)
	if err != nil {
		t.Errorf("Failed to delete application: %v", err)
	}
}
