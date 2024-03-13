package applications

import (
	_ "github.com/joho/godotenv/autoload"
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
}

func TestApplication_Create(t *testing.T) {
	client, err := setupDB()
	if err != nil {
		t.Fatal(err)
	}
	app := NewApplication("Test Application", "development")
	id, err := app.Create(client)
	if err != nil {
		t.Errorf("Failed to create application: %v", err)
	}
	got, _ := app.GetById(client, id)
	if got.Name != app.Name {
		t.Errorf("Got %v, want %v", got.Name, app.Name)
	}
}

func TestApplication_GetById(t *testing.T) {
	client, err := setupDB()
	if err != nil {
		t.Fatal(err)
	}
	app := NewApplication("Test Application", "development")
	id, err := app.Create(client)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := app.GetById(client, id)
	if got.Name != app.Name {
		t.Errorf("Got %v, want %v", got.Name, app.Name)
	}
}

func TestApplication_Update(t *testing.T) {
	client, err := setupDB()
	if err != nil {
		t.Fatal(err)
	}
	app := NewApplication("Test Application", "development")
	id, err := app.Create(client)
	if err != nil {
		t.Fatal(err)
	}
	app.Name = "Updated Application"
	err = app.Update(client)
	if err != nil {
		t.Errorf("Failed to update application: %v", err)
	}
	got, _ := app.GetById(client, id)
	if got.Name != app.Name {
		t.Errorf("Got %v, want %v", got.Name, app.Name)
	}
}

func TestApplication_SoftDelete(t *testing.T) {
	client, err := setupDB()
	if err != nil {
		t.Fatal(err)
	}
	app := NewApplication("Test Application", "development")
	id, err := app.Create(client)
	if err != nil {
		t.Fatal(err)
	}
	err = app.SoftDelete(client)
	if err != nil {
		t.Errorf("Failed to soft delete application: %v", err)
	}
	_, err = app.GetById(client, id)
	if err == nil {
		t.Errorf("Got 1 document, want 0")
	}
}

func Test_ApplicationList(t *testing.T) {
	client, err := setupDB()
	if err != nil {
		t.Fatal(err)
	}
	app := NewApplication("testList", "development")
	_, err = app.Create(client)
	if err != nil {
		t.Fatal(err)
	}
	apps, err := ApplicationList(client, app.OrganizationID)
	if err != nil {
		t.Errorf("Failed to list applications: %v", err)
	}
	if len(apps) == 0 {
		t.Errorf("Got 0 applications, want > 0")
	}
}

func TestApplication_ListNotDeleted(t *testing.T) {
	client, err := setupDB()
	if err != nil {
		t.Fatal(err)
	}
	app := NewApplication("testList2", "development")
	_, err = app.Create(client)
	if err != nil {
		t.Fatal(err)
	}
	err = app.SoftDelete(client)
	if err != nil {
		t.Errorf("Failed to soft delete application: %v", err)
	}
	apps, err := ApplicationList(client, app.OrganizationID)
	if err != nil {
		t.Errorf("Failed to list applications: %v", err)
	}
	if len(apps) != 0 {
		t.Errorf("Got > 0 applications, want = 0")
	}
}

func TestComponent_CreateList(t *testing.T) {
	client, err := setupDB()
	if err != nil {
		t.Fatal(err)
	}
	app := NewApplication("testAppComponent", "development")
	_, err = app.Create(client)
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
}
