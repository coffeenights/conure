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

	org := &Organization{Status: OrgActive, AccountId: "12345", Name: "Test Organization"}

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

	org := &Organization{Status: OrgActive, AccountId: "12345"}
	_, err = org.Create(client)
	if err != nil {
		t.Fatal(err)
	}

	got, _ := org.GetById(client, org.AccountId)
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
	_, err = org.Create(client)
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

	org := &Organization{Status: OrgActive, AccountId: "12345"}
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
