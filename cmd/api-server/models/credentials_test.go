package models

import (
	"context"
	"testing"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/coffeenights/conure/cmd/api-server/conureerrors"
	"github.com/coffeenights/conure/cmd/api-server/database"
)

func clearCreds(ctx context.Context, db *database.MongoDB, orgs ...primitive.ObjectID) {
	coll := db.Client.Database(db.DBName).Collection(CredentialCollection)
	for _, o := range orgs {
		_, _ = coll.DeleteMany(ctx, bson.M{"organizationId": o})
	}
}

func TestCredential_CreateGetByOrgAndName(t *testing.T) {
	db, err := SetupDB()
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	org := primitive.NewObjectID()
	t.Cleanup(func() { clearCreds(ctx, db, org) })

	c := &Credential{
		OrganizationID: org,
		Name:           "ghcr",
		Kind:           CredentialKindRegistry,
		RegistryURL:    "ghcr.io",
		Username:       "octocat",
		Secret:         "ciphertext-blob",
	}
	if _, err := c.Create(ctx, db); err != nil {
		t.Fatalf("create: %v", err)
	}

	got := &Credential{}
	if err := got.GetByOrgAndName(ctx, db, org, "ghcr"); err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Kind != CredentialKindRegistry || got.RegistryURL != "ghcr.io" ||
		got.Username != "octocat" || got.Secret != "ciphertext-blob" {
		t.Fatalf("round-trip mismatch: %+v", got)
	}
}

func TestCredential_GetByOrgAndName_NotFoundIsTyped(t *testing.T) {
	db, err := SetupDB()
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	org := primitive.NewObjectID()
	t.Cleanup(func() { clearCreds(ctx, db, org) })

	c := &Credential{}
	err = c.GetByOrgAndName(ctx, db, org, "missing")
	if err != conureerrors.ErrObjectNotFound {
		t.Fatalf("expected ErrObjectNotFound, got %v", err)
	}
}

func TestCredential_OrgIsolation(t *testing.T) {
	db, err := SetupDB()
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	orgA := primitive.NewObjectID()
	orgB := primitive.NewObjectID()
	t.Cleanup(func() { clearCreds(ctx, db, orgA, orgB) })

	a := &Credential{OrganizationID: orgA, Name: "shared", Kind: CredentialKindGit, Secret: "a"}
	if _, err := a.Create(ctx, db); err != nil {
		t.Fatalf("create A: %v", err)
	}

	// Same logical name in org B must not collide with org A's, and org B
	// must not be able to read org A's row by name.
	other := &Credential{}
	if err := other.GetByOrgAndName(ctx, db, orgB, "shared"); err != conureerrors.ErrObjectNotFound {
		t.Fatalf("org B leaked org A's credential: err=%v cred=%+v", err, other)
	}
	bList, err := other.ListByOrg(ctx, db, orgB)
	if err != nil {
		t.Fatalf("list B: %v", err)
	}
	if len(bList) != 0 {
		t.Fatalf("expected org B to see 0 credentials, got %d", len(bList))
	}
}

func TestCredential_ListByOrgSortedAndScoped(t *testing.T) {
	db, err := SetupDB()
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	org := primitive.NewObjectID()
	t.Cleanup(func() { clearCreds(ctx, db, org) })

	for _, n := range []string{"zeta", "alpha", "mid"} {
		cr := &Credential{OrganizationID: org, Name: n, Kind: CredentialKindRegistry, Secret: "x"}
		if _, err := cr.Create(ctx, db); err != nil {
			t.Fatalf("create %s: %v", n, err)
		}
	}
	c := &Credential{}
	got, err := c.ListByOrg(ctx, db, org)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	want := []string{"alpha", "mid", "zeta"}
	if len(got) != len(want) {
		t.Fatalf("expected %d, got %d", len(want), len(got))
	}
	for i, n := range want {
		if got[i].Name != n {
			t.Fatalf("sort order: got[%d]=%q want %q (full=%+v)", i, got[i].Name, n, got)
		}
	}
}

func TestCredential_SoftDeleteHidesFromGetAndList(t *testing.T) {
	db, err := SetupDB()
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	org := primitive.NewObjectID()
	t.Cleanup(func() { clearCreds(ctx, db, org) })

	c := &Credential{OrganizationID: org, Name: "temp", Kind: CredentialKindGit, Secret: "x"}
	if _, err := c.Create(ctx, db); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := c.Delete(ctx, db); err != nil {
		t.Fatalf("delete: %v", err)
	}

	after := &Credential{}
	if err := after.GetByOrgAndName(ctx, db, org, "temp"); err != conureerrors.ErrObjectNotFound {
		t.Fatalf("soft-deleted credential still resolvable by name: %v", err)
	}
	list, err := after.ListByOrg(ctx, db, org)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("soft-deleted credential still listed: %+v", list)
	}
}
