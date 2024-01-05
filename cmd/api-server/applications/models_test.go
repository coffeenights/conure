package applications

import (
	apiConfig "github.com/coffeenights/conure/cmd/api-server/config"
	"github.com/coffeenights/conure/cmd/api-server/database"
	"github.com/coffeenights/conure/internal/config"
	_ "github.com/joho/godotenv/autoload"
	"testing"
)

func setupDB() (*database.MongoDB, error) {
	appConfig := config.LoadConfig(apiConfig.Config{})
	testDBName := appConfig.MongoDBName + "-test"
	client, err := database.ConnectToMongoDB(appConfig.MongoDBURI, testDBName)
	if err != nil {
		return nil, err
	}
	return &database.MongoDB{Client: client.Client, DBName: testDBName}, nil
}

func TestOrganization_Create(t *testing.T) {
	client, err := setupDB()
	if err != nil {
		t.Fatal(err)
	}

	org := &Organization{Status: OrgActive, AccountId: "12345", Name: "Test Organization"}

	err = org.Create(client)
	if err != nil {
		t.Errorf("Failed to create organization: %v", err)
	}
}

func TestOrganization_GetById(t *testing.T) {
	client, err := setupDB()
	if err != nil {
		t.Fatal(err)
	}

	org := &Organization{Status: OrgActive, AccountId: "12345"}
	err = org.Create(client)
	if err != nil {
		t.Fatal(err)
	}

	got, err := org.GetById(client, org.AccountId)
	if got.AccountId != org.AccountId {
		t.Errorf("Got %v, want %v", got.AccountId, org.AccountId)
	}
}

func TestOrganization_Update(t *testing.T) {
	client, err := setupDB()
	if err != nil {
		t.Fatal(err)
	}

	org := &Organization{Status: OrgActive, AccountId: "12345"}
	err = org.Create(client)
	if err != nil {
		t.Fatal(err)
	}

	org.Status = OrgDisabled
	err = org.Update(client)
	if err != nil {
		t.Errorf("Failed to update organization: %v", err)
	}

	got, err := org.GetById(client, org.AccountId)
	if got.Status != OrgDisabled {
		t.Errorf("Got %v, want %v", got.Status, OrgDisabled)
	}
}

func TestOrganization_Delete(t *testing.T) {
	client, err := setupDB()
	if err != nil {
		t.Fatal(err)
	}

	org := &Organization{Status: OrgActive, AccountId: "12345"}
	err = org.Create(client)
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

	org := &Organization{Status: OrgActive, AccountId: "12345"}
	err = org.Create(client)
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
