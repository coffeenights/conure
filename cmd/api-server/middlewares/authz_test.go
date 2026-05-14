package middlewares

import (
	"testing"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/coffeenights/conure/cmd/api-server/models"
)

func TestCanReadOrg(t *testing.T) {
	owner := primitive.NewObjectID()
	member := primitive.NewObjectID()
	outsider := primitive.NewObjectID()
	orgID := primitive.NewObjectID()
	org := &models.Organization{
		Model:     models.Model{ID: orgID},
		AccountID: owner,
	}

	cases := []struct {
		name string
		user models.User
		want bool
	}{
		{"admin sees any org", models.User{ID: outsider, Role: models.RoleAdmin}, true},
		{"owner sees their org", models.User{ID: owner, Role: models.RoleDeveloper}, true},
		{"member of the org sees it", models.User{ID: member, Role: models.RoleDeveloper, OrganizationID: orgID}, true},
		{"outsider cannot read", models.User{ID: outsider, Role: models.RoleDeveloper}, false},
		{"developer in a different org cannot read", models.User{ID: outsider, Role: models.RoleDeveloper, OrganizationID: primitive.NewObjectID()}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := CanReadOrg(tc.user, org); got != tc.want {
				t.Errorf("CanReadOrg = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestCanWriteOrg(t *testing.T) {
	owner := primitive.NewObjectID()
	member := primitive.NewObjectID()
	orgID := primitive.NewObjectID()
	org := &models.Organization{
		Model:     models.Model{ID: orgID},
		AccountID: owner,
	}
	if !CanWriteOrg(models.User{ID: member, Role: models.RoleAdmin}, org) {
		t.Error("admin must be able to write any org")
	}
	if !CanWriteOrg(models.User{ID: owner, Role: models.RoleDeveloper}, org) {
		t.Error("owner must be able to write their org")
	}
	if CanWriteOrg(models.User{ID: member, Role: models.RoleDeveloper, OrganizationID: orgID}, org) {
		t.Error("non-owner member must NOT be able to write org-level settings")
	}
}

func TestCanWriteOwned(t *testing.T) {
	creator := primitive.NewObjectID()
	other := primitive.NewObjectID()
	if !CanWriteOwned(models.User{ID: other, Role: models.RoleAdmin}, creator) {
		t.Error("admin must be able to write resources they don't own")
	}
	if !CanWriteOwned(models.User{ID: creator, Role: models.RoleDeveloper}, creator) {
		t.Error("creator must be able to write their own resource")
	}
	if CanWriteOwned(models.User{ID: other, Role: models.RoleDeveloper}, creator) {
		t.Error("a non-creator developer must NOT be able to write")
	}
}
