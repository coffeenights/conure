package models

import (
	"context"
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

func TestComponentTypeSpec_Create(t *testing.T) {
	client, err := SetupDB()
	if err != nil {
		t.Fatal(err)
	}

	orgID := primitive.NewObjectID()
	iconURL := "https://example.com/icon.png"
	spec := &ComponentTypeSpec{
		OrganizationID: orgID,
		Name:           "Test WebService",
		Description:    "A test web service component type",
		Type:           "webservice",
		OCIRepository:  "nginx",
		OCITag:         "latest",
		IconURL:        &iconURL,
	}

	err = spec.Create(context.Background(), client)
	if err != nil {
		t.Errorf("Failed to create component type spec: %v", err)
	}

	// Verify the spec was created
	var got ComponentTypeSpec
	err = got.GetByID(context.Background(), client, spec.ID.Hex())
	if err != nil {
		t.Errorf("Failed to get component type spec: %v", err)
	}

	if got.Name != spec.Name {
		t.Errorf("Got %v, want %v", got.Name, spec.Name)
	}

	// Cleanup
	err = spec.Delete(context.Background(), client)
	if err != nil {
		t.Errorf("Failed to delete component type spec: %v", err)
	}
}

func TestComponentTypeSpec_GetByID(t *testing.T) {
	client, err := SetupDB()
	if err != nil {
		t.Fatal(err)
	}

	orgID := primitive.NewObjectID()
	iconURL := "https://example.com/postgres.png"
	spec := &ComponentTypeSpec{
		OrganizationID: orgID,
		Name:           "Test Database",
		Description:    "A test database component type",
		Type:           "database",
		OCIRepository:  "postgres",
		OCITag:         "13",
		IconURL:        &iconURL,
	}

	err = spec.Create(context.Background(), client)
	if err != nil {
		t.Fatal(err)
	}
	defer spec.Delete(context.Background(), client)

	var got ComponentTypeSpec
	err = got.GetByID(context.Background(), client, spec.ID.Hex())
	if err != nil {
		t.Errorf("Failed to get component type spec: %v", err)
	}

	if got.OrganizationID != spec.OrganizationID {
		t.Errorf("Got %v, want %v", got.OrganizationID, spec.OrganizationID)
	}
	if got.Name != spec.Name {
		t.Errorf("Got %v, want %v", got.Name, spec.Name)
	}
	if got.Type != spec.Type {
		t.Errorf("Got %v, want %v", got.Type, spec.Type)
	}
	if got.OCIRepository != spec.OCIRepository {
		t.Errorf("Got %v, want %v", got.OCIRepository, spec.OCIRepository)
	}
}

func TestComponentTypeSpec_GetByID_NotFound(t *testing.T) {
	client, err := SetupDB()
	if err != nil {
		t.Fatal(err)
	}

	var spec ComponentTypeSpec
	err = spec.GetByID(context.Background(), client, primitive.NewObjectID().Hex())
	if err == nil {
		t.Errorf("Got nil, want error")
	}
	if !errors.Is(err, conureerrors.ErrObjectNotFound) {
		t.Errorf("Got %v, want %v", err, conureerrors.ErrObjectNotFound)
	}
}

func TestComponentTypeSpec_Update(t *testing.T) {
	client, err := SetupDB()
	if err != nil {
		t.Fatal(err)
	}

	orgID := primitive.NewObjectID()
	iconURL := "https://example.com/alpine.png"
	spec := &ComponentTypeSpec{
		OrganizationID: orgID,
		Name:           "Test Service",
		Description:    "A test service component type",
		Type:           "service",
		OCIRepository:  "alpine",
		OCITag:         "3.14",
		IconURL:        &iconURL,
	}

	err = spec.Create(context.Background(), client)
	if err != nil {
		t.Fatal(err)
	}
	defer spec.Delete(context.Background(), client)

	// Update the spec
	spec.Name = "Updated Test Service"
	spec.Description = "An updated test service component type"
	spec.OCITag = "3.15"

	err = spec.Update(context.Background(), client)
	if err != nil {
		t.Errorf("Failed to update component type spec: %v", err)
	}

	// Verify the update
	var got ComponentTypeSpec
	err = got.GetByID(context.Background(), client, spec.ID.Hex())
	if err != nil {
		t.Errorf("Failed to get updated component type spec: %v", err)
	}

	if got.Name != "Updated Test Service" {
		t.Errorf("Got %v, want %v", got.Name, "Updated Test Service")
	}
	if got.Description != "An updated test service component type" {
		t.Errorf("Got %v, want %v", got.Description, "An updated test service component type")
	}
	if got.OCITag != "3.15" {
		t.Errorf("Got %v, want %v", got.OCITag, "3.15")
	}
}

func TestComponentTypeSpec_Delete(t *testing.T) {
	client, err := SetupDB()
	if err != nil {
		t.Fatal(err)
	}

	orgID := primitive.NewObjectID()
	iconURL := "https://example.com/busybox.png"
	spec := &ComponentTypeSpec{
		OrganizationID: orgID,
		Name:           "Test Delete",
		Description:    "A test component type for deletion",
		Type:           "worker",
		OCIRepository:  "busybox",
		OCITag:         "latest",
		IconURL:        &iconURL,
	}

	err = spec.Create(context.Background(), client)
	if err != nil {
		t.Fatal(err)
	}

	// Delete the spec
	err = spec.Delete(context.Background(), client)
	if err != nil {
		t.Errorf("Failed to delete component type spec: %v", err)
	}

	// Verify it's deleted
	var got ComponentTypeSpec
	err = got.GetByID(context.Background(), client, spec.ID.Hex())
	if err == nil {
		t.Errorf("Got component type spec, want error")
	}
	if !errors.Is(err, conureerrors.ErrObjectNotFound) {
		t.Errorf("Got %v, want %v", err, conureerrors.ErrObjectNotFound)
	}
}

func TestComponentTypeSpecList(t *testing.T) {
	client, err := SetupDB()
	if err != nil {
		t.Fatal(err)
	}

	orgID := primitive.NewObjectID()

	// Create multiple component type specs
	iconURL1 := "https://example.com/nginx.png"
	spec1 := &ComponentTypeSpec{
		OrganizationID: orgID,
		Name:           "Web Service",
		Description:    "A web service component",
		Type:           "webservice",
		OCIRepository:  "nginx",
		OCITag:         "latest",
		IconURL:        &iconURL1,
	}

	iconURL2 := "https://example.com/mysql.png"
	spec2 := &ComponentTypeSpec{
		OrganizationID: orgID,
		Name:           "Database",
		Description:    "A database component",
		Type:           "database",
		OCIRepository:  "mysql",
		OCITag:         "8.0",
		IconURL:        &iconURL2,
	}

	// Create specs in different organization to test filtering
	otherOrgID := primitive.NewObjectID()
	iconURL3 := "https://example.com/redis.png"
	spec3 := &ComponentTypeSpec{
		OrganizationID: otherOrgID,
		Name:           "Other Org Service",
		Description:    "A service in different org",
		Type:           "service",
		OCIRepository:  "redis",
		OCITag:         "6.0",
		IconURL:        &iconURL3,
	}

	err = spec1.Create(context.Background(), client)
	if err != nil {
		t.Fatal(err)
	}
	defer spec1.Delete(context.Background(), client)

	err = spec2.Create(context.Background(), client)
	if err != nil {
		t.Fatal(err)
	}
	defer spec2.Delete(context.Background(), client)

	err = spec3.Create(context.Background(), client)
	if err != nil {
		t.Fatal(err)
	}
	defer spec3.Delete(context.Background(), client)

	// Test listing specs for the first organization
	specs, err := ComponentTypeSpecList(context.Background(), client, orgID.Hex())
	if err != nil {
		t.Errorf("Failed to list component type specs: %v", err)
	}

	if len(specs) != 2 {
		t.Errorf("Got %d specs, want 2", len(specs))
	}

	// Verify the specs belong to the correct organization
	for _, spec := range specs {
		if spec.OrganizationID != orgID {
			t.Errorf("Got organizationID %v, want %v", spec.OrganizationID, orgID)
		}
	}
}

func TestComponentTypeSpecList_EmptyOrganization(t *testing.T) {
	client, err := SetupDB()
	if err != nil {
		t.Fatal(err)
	}

	// Test listing specs for an organization with no specs
	emptyOrgID := primitive.NewObjectID()
	specs, err := ComponentTypeSpecList(context.Background(), client, emptyOrgID.Hex())
	if err != nil {
		t.Errorf("Failed to list component type specs: %v", err)
	}

	if len(specs) != 0 {
		t.Errorf("Got %d specs, want 0", len(specs))
	}
}

func TestComponentTypeSpecList_InvalidOrganizationID(t *testing.T) {
	client, err := SetupDB()
	if err != nil {
		t.Fatal(err)
	}

	// Test with invalid organization ID
	_, err = ComponentTypeSpecList(context.Background(), client, "invalid-id")
	if err == nil {
		t.Errorf("Got nil, want error for invalid organization ID")
	}
}

func TestComponentTypeSpecGetByType(t *testing.T) {
	client, err := SetupDB()
	if err != nil {
		t.Fatal(err)
	}

	orgID := primitive.NewObjectID()

	// Create component type specs of different types
	iconURL1 := "https://example.com/webservice.png"
	spec1 := &ComponentTypeSpec{
		OrganizationID: orgID,
		Name:           "Nginx Web Service",
		Description:    "A nginx web service",
		Type:           "webservice",
		OCIRepository:  "nginx",
		OCITag:         "latest",
		IconURL:        &iconURL1,
	}

	iconURL2 := "https://example.com/database.png"
	spec2 := &ComponentTypeSpec{
		OrganizationID: orgID,
		Name:           "MySQL Database",
		Description:    "A MySQL database",
		Type:           "database",
		OCIRepository:  "mysql",
		OCITag:         "8.0",
		IconURL:        &iconURL2,
	}

	// Create specs in different organization to test filtering
	otherOrgID := primitive.NewObjectID()
	iconURL3 := "https://example.com/other-webservice.png"
	spec3 := &ComponentTypeSpec{
		OrganizationID: otherOrgID,
		Name:           "Other Org Web Service",
		Description:    "A web service in different org",
		Type:           "webservice",
		OCIRepository:  "nginx",
		OCITag:         "alpine",
		IconURL:        &iconURL3,
	}

	// Create all specs
	err = spec1.Create(context.Background(), client)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = spec1.Delete(context.Background(), client)
	}()

	err = spec2.Create(context.Background(), client)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = spec2.Delete(context.Background(), client)
	}()

	err = spec3.Create(context.Background(), client)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = spec3.Delete(context.Background(), client)
	}()

	// Test getting webservice spec for the first organization
	var webserviceSpec ComponentTypeSpec
	err = webserviceSpec.GetByType(context.Background(), client, orgID.Hex(), "webservice")
	if err != nil {
		t.Errorf("Failed to get webservice component type spec: %v", err)
	}

	// Verify returned spec is webservice type and belongs to correct organization
	if webserviceSpec.Type != "webservice" {
		t.Errorf("Got type %v, want webservice", webserviceSpec.Type)
	}
	if webserviceSpec.OrganizationID != orgID {
		t.Errorf("Got organizationID %v, want %v", webserviceSpec.OrganizationID, orgID)
	}
	if webserviceSpec.Name != "Nginx Web Service" {
		t.Errorf("Got name %v, want 'Nginx Web Service'", webserviceSpec.Name)
	}

	// Test getting database spec for the first organization
	var databaseSpec ComponentTypeSpec
	err = databaseSpec.GetByType(context.Background(), client, orgID.Hex(), "database")
	if err != nil {
		t.Errorf("Failed to get database component type spec: %v", err)
	}

	if databaseSpec.Type != "database" {
		t.Errorf("Got type %v, want database", databaseSpec.Type)
	}
	if databaseSpec.Name != "MySQL Database" {
		t.Errorf("Got name %v, want 'MySQL Database'", databaseSpec.Name)
	}
}

func TestComponentTypeSpecGetByType_NotFound(t *testing.T) {
	client, err := SetupDB()
	if err != nil {
		t.Fatal(err)
	}

	orgID := primitive.NewObjectID()

	// Test getting spec for a type that doesn't exist
	var spec ComponentTypeSpec
	err = spec.GetByType(context.Background(), client, orgID.Hex(), "nonexistent")
	if err == nil {
		t.Errorf("Got nil, want error for nonexistent type")
	}
	if !errors.Is(err, conureerrors.ErrObjectNotFound) {
		t.Errorf("Got %v, want %v", err, conureerrors.ErrObjectNotFound)
	}
}

func TestComponentTypeSpecGetByType_InvalidOrganizationID(t *testing.T) {
	client, err := SetupDB()
	if err != nil {
		t.Fatal(err)
	}

	// Test with invalid organization ID
	var spec ComponentTypeSpec
	err = spec.GetByType(context.Background(), client, "invalid-id", "webservice")
	if err == nil {
		t.Errorf("Got nil, want error for invalid organization ID")
	}
}
