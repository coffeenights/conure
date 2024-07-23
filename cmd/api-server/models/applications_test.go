package models

import (
	"errors"
	"testing"

	_ "github.com/joho/godotenv/autoload"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/coffeenights/conure/cmd/api-server/conureerrors"
)

func TestOrganization_Create(t *testing.T) {
	client, err := SetupDB()
	if err != nil {
		t.Fatal(err)
	}

	org := &Organization{Status: OrgActive, AccountID: primitive.NewObjectID(), Name: "Test Organization"}

	_, err = org.Create(client)
	if err != nil {
		t.Errorf("Failed to create organization: %v", err)
	}
}

func TestOrganization_GetById(t *testing.T) {
	client, err := SetupDB()
	if err != nil {
		t.Fatal(err)
	}

	org := &Organization{Status: OrgActive, AccountID: primitive.NewObjectID()}
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
	client, err := SetupDB()
	if err != nil {
		t.Fatal(err)
	}

	org := &Organization{Status: OrgActive, AccountID: primitive.NewObjectID()}
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
	client, err := SetupDB()
	if err != nil {
		t.Fatal(err)
	}

	org := &Organization{Status: OrgActive, AccountID: primitive.NewObjectID()}
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
	client, err := SetupDB()
	if err != nil {
		t.Fatal(err)
	}

	org := &Organization{Status: OrgActive, AccountID: primitive.NewObjectID()}
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
	client, err := SetupDB()
	if err != nil {
		t.Fatal(err)
	}
	app, err := NewApplication(primitive.NewObjectID().Hex(), "TestApplicationCreate", primitive.NewObjectID().Hex()).Create(client)
	if err != nil {
		t.Errorf("Failed to create application: %v", err)
	}
	var got Application
	err = got.GetByID(client, app.ID.Hex())
	if got.Name != app.Name {
		t.Errorf("Got %v, want %v", got.Name, app.Name)

	}
	err = app.Delete(client)
	if err != nil {
		t.Errorf("Failed to delete application: %v", err)
	}
}

func TestApplication_GetById(t *testing.T) {
	client, err := SetupDB()
	if err != nil {
		t.Fatal(err)
	}
	app, err := NewApplication(primitive.NewObjectID().Hex(), "TestApplicationGetById", primitive.NewObjectID().Hex()).Create(client)
	if err != nil {
		t.Fatal(err)
	}
	var got Application
	_ = got.GetByID(client, app.ID.Hex())
	if got.Name != app.Name {
		t.Errorf("Got %v, want %v", got.Name, app.Name)
	}
	err = app.Delete(client)
	if err != nil {
		t.Errorf("Failed to delete application: %v", err)
	}
}

func TestApplication_GetById_NotExist(t *testing.T) {
	client, err := SetupDB()
	if err != nil {
		t.Fatal(err)
	}
	if err != nil {
		t.Fatal(err)
	}
	got := Application{}
	err = got.GetByID(client, primitive.NewObjectID().Hex())
	if err == nil {
		t.Errorf("Got nil, want error")
	}
	if !errors.Is(err, conureerrors.ErrObjectNotFound) {
		t.Errorf("Got %v, want %v", err, conureerrors.ErrObjectNotFound)
	}
}

func TestApplication_Update(t *testing.T) {
	client, err := SetupDB()
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
	var got Application
	_ = got.GetByID(client, app.ID.Hex())
	if got.Name != app.Name {
		t.Errorf("Got %v, want %v", got.Name, app.Name)
	}
	err = app.Delete(client)
	if err != nil {
		t.Errorf("Failed to delete application: %v", err)
	}
}

func TestApplication_SoftDelete(t *testing.T) {
	client, err := SetupDB()
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
	err = app.GetByID(client, app.ID.Hex())
	if err == nil {
		t.Errorf("Got 1 document, want 0")
	}
	err = app.Delete(client)
	if err != nil {
		t.Errorf("Failed to delete application: %v", err)
	}
}

func Test_ApplicationList(t *testing.T) {
	client, err := SetupDB()
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
	client, err := SetupDB()
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

func TestApplication_CountComponents(t *testing.T) {
	client, err := SetupDB()
	if err != nil {
		t.Fatal(err)
	}
	app, err := NewApplication(primitive.NewObjectID().Hex(), "TestApplicationCountComponents", primitive.NewObjectID().Hex()).Create(client)
	if err != nil {
		t.Fatal(err)
	}
	comp := Component{
		ApplicationID: app.ID,
		Name:          "test-component",
		Type:          "service",
	}
	err = comp.Create(client)
	if err != nil {
		t.Errorf("Failed to create component: %v", err)
	}
	count, err := app.CountComponents(client)
	if err != nil {
		t.Errorf("Failed to count components: %v", err)
	}
	if count != 1 {
		t.Errorf("Got %d components, want 1", count)
	}
	err = app.Delete(client)
	if err != nil {
		t.Errorf("Failed to delete application: %v", err)
	}
	_ = comp.Delete(client)
}

func TestComponent_CreateList(t *testing.T) {
	client, err := SetupDB()
	if err != nil {
		t.Fatal(err)
	}
	app, err := NewApplication(primitive.NewObjectID().Hex(), "TestApplicationSoftDelete", primitive.NewObjectID().Hex()).Create(client)
	if err != nil {
		t.Fatal(err)
	}
	comp := Component{
		ApplicationID: app.ID,
		Name:          "test-component",
		Type:          "webservice",
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
	_ = comp.Delete(client)
}

func TestApplication_CreateEnvironment(t *testing.T) {
	client, err := SetupDB()
	if err != nil {
		t.Fatal(err)
	}
	app, err := NewApplication(primitive.NewObjectID().Hex(), "TestApplicationCreateEnvironment", primitive.NewObjectID().Hex()).Create(client)
	if err != nil {
		t.Fatal(err)
	}
	_, err = app.CreateEnvironment(client, "testEnvironment")
	if err != nil {
		t.Errorf("Failed to create environment: %v", err)
	}

	_ = app.Delete(client)
}

func TestApplication_DeleteEnvironmentByID(t *testing.T) {
	client, err := SetupDB()
	if err != nil {
		t.Fatal(err)
	}
	app, err := NewApplication(primitive.NewObjectID().Hex(), "TestApplicationDeleteEnvironment", primitive.NewObjectID().Hex()).Create(client)
	if err != nil {
		t.Fatal(err)
	}
	env1, err := app.CreateEnvironment(client, "staging")
	if err != nil {
		t.Fatal(err)
	}
	_, err = app.CreateEnvironment(client, "development")
	if err != nil {
		t.Fatal(err)
	}
	err = app.DeleteEnvironmentByID(client, env1.ID)
	if err != nil {
		t.Errorf("Failed to delete environment: %v", err)
	}
	if err = app.GetByID(client, app.ID.Hex()); err != nil {
		t.Errorf("Failed to get application: %v", err)
	}
	if len(app.Environments) != 1 {
		t.Errorf("Got %d environments, want 1", len(app.Environments))
	}
	_ = app.Delete(client)
}

func TestApplication_DeleteEnvironmentByName(t *testing.T) {
	client, err := SetupDB()
	if err != nil {
		t.Fatal(err)
	}
	app, err := NewApplication(primitive.NewObjectID().Hex(), "TestApplicationDeleteEnvironmentByName", primitive.NewObjectID().Hex()).Create(client)
	if err != nil {
		t.Fatal(err)
	}
	env1, err := app.CreateEnvironment(client, "staging")
	if err != nil {
		t.Fatal(err)
	}
	_, err = app.CreateEnvironment(client, "development")
	if err != nil {
		t.Fatal(err)
	}
	err = app.DeleteEnvironmentByName(client, env1.Name)
	if err != nil {
		t.Errorf("Failed to delete environment: %v", err)
	}
	if err = app.GetByID(client, app.ID.Hex()); err != nil {
		t.Errorf("Failed to get application: %v", err)
	}
	if len(app.Environments) != 1 {
		t.Errorf("Got %d environments, want 1", len(app.Environments))
	}
	_ = app.Delete(client)
}

func TestApplication_GetEnvironmentByName(t *testing.T) {
	client, err := SetupDB()
	if err != nil {
		t.Fatal(err)
	}
	app, err := NewApplication(primitive.NewObjectID().Hex(), "TestApplicationGetEnvironmentByName", primitive.NewObjectID().Hex()).Create(client)
	if err != nil {
		t.Fatal(err)
	}
	env1, err := app.CreateEnvironment(client, "staging")
	if err != nil {
		t.Fatal(err)
	}
	_, err = app.CreateEnvironment(client, "development")
	if err != nil {
		t.Fatal(err)
	}
	env, err := app.GetEnvironmentByName(client, env1.Name)
	if err != nil {
		t.Errorf("Failed to get environment: %v", err)
	}
	if env.Name != env1.Name {
		t.Errorf("Got %v, want %v", env.Name, env1.Name)
	}
	_ = app.Delete(client)
}

func TestApplication_GetEnvironmentByName_NotFound(t *testing.T) {
	client, err := SetupDB()
	if err != nil {
		t.Fatal(err)
	}
	app, err := NewApplication(primitive.NewObjectID().Hex(), "TestApplicationGetEnvironmentByNameNotFound", primitive.NewObjectID().Hex()).Create(client)
	if err != nil {
		t.Fatal(err)
	}
	_, err = app.CreateEnvironment(client, "staging")
	if err != nil {
		t.Fatal(err)
	}
	_, err = app.GetEnvironmentByName(client, "asd")
	if err == nil {
		t.Errorf("Got nil, want error")
	}
}

func TestApplication_DeleteEnvironment_NotExist(t *testing.T) {
	client, err := SetupDB()
	if err != nil {
		t.Fatal(err)
	}
	app, err := NewApplication(primitive.NewObjectID().Hex(), "TestApplicationDeleteEnvironmentNotExist", primitive.NewObjectID().Hex()).Create(client)
	if err != nil {
		t.Fatal(err)
	}
	err = app.DeleteEnvironmentByID(client, primitive.NewObjectID().Hex())
	if err == nil {
		t.Errorf("Got nil, want error")
	}
	_ = app.Delete(client)
}

func TestComponent_GetByID(t *testing.T) {
	client, err := SetupDB()
	if err != nil {
		t.Fatal(err)
	}
	app, err := NewApplication(primitive.NewObjectID().Hex(), "TestComponentGetByID", primitive.NewObjectID().Hex()).Create(client)
	if err != nil {
		t.Fatal(err)
	}
	comp := ComponentTemplate(app.ID, "test-component-get-by-id")

	err = comp.Create(client)
	defer comp.Delete(client)
	if err != nil {
		t.Errorf("Failed to create component: %v", err)
	}

	findComp := Component{}
	err = findComp.GetByID(client, comp.ID.Hex())
	if err != nil {
		t.Errorf("Failed to get component: %v", err)
	}

	if findComp.ID != comp.ID {
		t.Errorf("Got %v, want %v", findComp.ID, comp.ID)
	}
}

func TestComponent_Create(t *testing.T) {
	client, err := SetupDB()
	if err != nil {
		t.Fatal(err)
	}
	app, err := NewApplication(primitive.NewObjectID().Hex(), "TestComponentCreate", primitive.NewObjectID().Hex()).Create(client)
	if err != nil {
		t.Fatal(err)
	}
	defer app.Delete(client)

	comp := ComponentTemplate(app.ID, "test-component")
	err = comp.Create(client)
	if err != nil {
		t.Errorf("Failed to create component: %v", err)
	}
	_ = comp.Delete(client)
}

func TestComponent_Create_Duplicate(t *testing.T) {
	client, err := SetupDB()
	if err != nil {
		t.Fatal(err)
	}
	app, err := NewApplication(primitive.NewObjectID().Hex(), "TestComponentCreateDuplicate", primitive.NewObjectID().Hex()).Create(client)
	if err != nil {
		t.Fatal(err)
	}
	defer app.Delete(client)

	comp := ComponentTemplate(app.ID, "test-component")
	err = comp.Create(client)
	if err != nil {
		t.Errorf("Failed to create component: %v", err)
		t.FailNow()
	}
	err = comp.Create(client)
	if err == nil {
		t.Errorf("Got nil, want error")
	} else if !errors.Is(err, conureerrors.ErrObjectAlreadyExists) {
		t.Errorf("Got %v, want %v", err, conureerrors.ErrObjectAlreadyExists)
	}
	_ = comp.Delete(client)
}
