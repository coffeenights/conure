package models

import (
	"context"
	"testing"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/coffeenights/conure/cmd/api-server/conureerrors"
	"github.com/coffeenights/conure/cmd/api-server/database"
)

// clearDefs removes every component-definition row for the given owners so the
// shared per-package test DB stays isolated between tests. The default-owner
// rows are global, so tests that seed defaults must list it here too.
func clearDefs(ctx context.Context, db *database.MongoDB, owners ...primitive.ObjectID) {
	coll := db.Client.Database(db.DBName).Collection(ComponentDefinitionCollection)
	for _, o := range owners {
		_, _ = coll.DeleteMany(ctx, bson.M{"organizationId": o})
	}
}

func seedDef(t *testing.T, ctx context.Context, db *database.MongoDB, compType, engine, repo, tag string) {
	t.Helper()
	d := &ComponentDefinition{
		Type:          compType,
		Engine:        engine,
		OCIRepository: repo,
		OCITag:        tag,
	}
	if err := d.UpsertSeed(ctx, db); err != nil {
		t.Fatalf("seed default %s/%s: %v", compType, engine, err)
	}
}

func TestComponentDefinitions_DefaultInheritedByOrg(t *testing.T) {
	db, err := SetupDB()
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	org := primitive.NewObjectID()
	t.Cleanup(func() { clearDefs(ctx, db, DefaultComponentDefinitionOwner, org) })

	seedDef(t, ctx, db, "webservice", "timoni", "ghcr.io/acme/web", "1.0.0")

	got, err := ResolveForOrg(ctx, db, org)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 inherited definition, got %d", len(got))
	}
	if got[0].OCIRepository != "ghcr.io/acme/web" || got[0].OrganizationID != DefaultComponentDefinitionOwner {
		t.Fatalf("org did not inherit the default unchanged: %+v", got[0])
	}

	one, err := ResolveOneForOrg(ctx, db, org, "webservice", "timoni")
	if err != nil {
		t.Fatalf("ResolveOneForOrg: %v", err)
	}
	if one.OCIRepository != "ghcr.io/acme/web" {
		t.Fatalf("ResolveOneForOrg returned wrong row: %+v", one)
	}
}

func TestComponentDefinitions_OrgOverrideShadowsDefault(t *testing.T) {
	db, err := SetupDB()
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	org := primitive.NewObjectID()
	t.Cleanup(func() { clearDefs(ctx, db, DefaultComponentDefinitionOwner, org) })

	seedDef(t, ctx, db, "webservice", "timoni", "ghcr.io/default/web", "1.0.0")

	override := &ComponentDefinition{
		OrganizationID: org,
		Type:           "webservice",
		Engine:         "timoni",
		OCIRepository:  "ghcr.io/org/custom-web",
		OCITag:         "9.9.9",
	}
	if _, err := override.Create(ctx, db); err != nil {
		t.Fatal(err)
	}

	got, err := ResolveForOrg(ctx, db, org)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("override should replace, not add: got %d rows", len(got))
	}
	if got[0].OCIRepository != "ghcr.io/org/custom-web" || got[0].OrganizationID != org {
		t.Fatalf("expected org override to win, got %+v", got[0])
	}

	// A different org with no override still sees the default.
	other := primitive.NewObjectID()
	t.Cleanup(func() { clearDefs(ctx, db, other) })
	otherGot, err := ResolveForOrg(ctx, db, other)
	if err != nil {
		t.Fatal(err)
	}
	if len(otherGot) != 1 || otherGot[0].OCIRepository != "ghcr.io/default/web" {
		t.Fatalf("override leaked across orgs: %+v", otherGot)
	}
}

func TestComponentDefinitions_TombstoneHidesDefaultReversibly(t *testing.T) {
	db, err := SetupDB()
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	org := primitive.NewObjectID()
	t.Cleanup(func() { clearDefs(ctx, db, DefaultComponentDefinitionOwner, org) })

	seedDef(t, ctx, db, "webservice", "timoni", "ghcr.io/default/web", "1.0.0")

	tomb := &ComponentDefinition{
		OrganizationID: org,
		Type:           "webservice",
		Engine:         "timoni",
		Hidden:         true,
	}
	if _, err := tomb.Create(ctx, db); err != nil {
		t.Fatal(err)
	}

	got, err := ResolveForOrg(ctx, db, org)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("tombstone should suppress the inherited default, got %d rows", len(got))
	}
	if _, err := ResolveOneForOrg(ctx, db, org, "webservice", "timoni"); err != conureerrors.ErrComponentNotFound {
		t.Fatalf("expected ErrComponentNotFound for hidden type, got %v", err)
	}

	// Deleting the tombstone restores the inherited default (reversible).
	if err := tomb.Delete(ctx, db); err != nil {
		t.Fatal(err)
	}
	restored, err := ResolveForOrg(ctx, db, org)
	if err != nil {
		t.Fatal(err)
	}
	if len(restored) != 1 || restored[0].OCIRepository != "ghcr.io/default/web" {
		t.Fatalf("default not restored after un-hide: %+v", restored)
	}
}

// TestComponentDefinitions_PerOrgIsolation is the cross-org collision
// regression at the resolution layer: two orgs each define "webservice" with
// different repos; resolving for one must never surface the other's row, and
// must not be reported as ambiguous.
func TestComponentDefinitions_PerOrgIsolation(t *testing.T) {
	db, err := SetupDB()
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	orgA := primitive.NewObjectID()
	orgB := primitive.NewObjectID()
	t.Cleanup(func() { clearDefs(ctx, db, DefaultComponentDefinitionOwner, orgA, orgB) })

	a := &ComponentDefinition{OrganizationID: orgA, Type: "webservice", Engine: "timoni", OCIRepository: "ghcr.io/a/web"}
	b := &ComponentDefinition{OrganizationID: orgB, Type: "webservice", Engine: "timoni", OCIRepository: "ghcr.io/b/web"}
	if _, err := a.Create(ctx, db); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Create(ctx, db); err != nil {
		t.Fatal(err)
	}

	gotA, err := ResolveOneForOrg(ctx, db, orgA, "webservice", "")
	if err != nil {
		t.Fatalf("orgA resolve (engine unset) should be unambiguous, got %v", err)
	}
	if gotA.OCIRepository != "ghcr.io/a/web" {
		t.Fatalf("orgA resolved to the wrong org's definition: %+v", gotA)
	}
	gotB, err := ResolveOneForOrg(ctx, db, orgB, "webservice", "")
	if err != nil {
		t.Fatalf("orgB resolve: %v", err)
	}
	if gotB.OCIRepository != "ghcr.io/b/web" {
		t.Fatalf("orgB resolved to the wrong org's definition: %+v", gotB)
	}
}

func TestComponentDefinitions_AmbiguousEngineWithinOrg(t *testing.T) {
	db, err := SetupDB()
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	org := primitive.NewObjectID()
	t.Cleanup(func() { clearDefs(ctx, db, DefaultComponentDefinitionOwner, org) })

	seedDef(t, ctx, db, "webservice", "timoni", "ghcr.io/d/web-timoni", "1.0.0")
	seedDef(t, ctx, db, "webservice", "helm", "ghcr.io/d/web-helm", "1.0.0")

	if _, err := ResolveOneForOrg(ctx, db, org, "webservice", ""); err != conureerrors.ErrAmbiguousComponentEngine {
		t.Fatalf("expected ErrAmbiguousComponentEngine when engine unset and two engines match, got %v", err)
	}
	got, err := ResolveOneForOrg(ctx, db, org, "webservice", "helm")
	if err != nil {
		t.Fatalf("engine-pinned resolve: %v", err)
	}
	if got.OCIRepository != "ghcr.io/d/web-helm" {
		t.Fatalf("engine pin resolved wrong row: %+v", got)
	}
}

func TestComponentDefinitions_UpsertSeedIdempotent(t *testing.T) {
	db, err := SetupDB()
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	t.Cleanup(func() { clearDefs(ctx, db, DefaultComponentDefinitionOwner) })

	seedDef(t, ctx, db, "webservice", "timoni", "ghcr.io/x/web", "1.0.0")
	seedDef(t, ctx, db, "webservice", "timoni", "ghcr.io/x/web", "2.0.0") // re-seed, new tag

	got, err := ResolveForOrg(ctx, db, DefaultComponentDefinitionOwner)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("re-seeding (type,engine) must reconcile in place, got %d rows", len(got))
	}
	if got[0].OCITag != "2.0.0" {
		t.Fatalf("re-seed did not update the row: %+v", got[0])
	}
}

// TestComponentDefinitions_EmptyEngineIsFindable is the regression for the
// engine-storage bug: a row created with Engine:"" was persisted with the
// field omitted, so GetOwnRow (and the upsert filter) — which match
// engine=="timoni" — could never find it again, producing duplicate rows on
// every subsequent set/hide/seed. Storage is now normalized on write, so the
// empty form and the explicit "timoni" form are the same row.
func TestComponentDefinitions_EmptyEngineIsFindable(t *testing.T) {
	db, err := SetupDB()
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	org := primitive.NewObjectID()
	t.Cleanup(func() { clearDefs(ctx, db, DefaultComponentDefinitionOwner, org) })

	// Org override created with the engine left unset.
	created := &ComponentDefinition{OrganizationID: org, Type: "webservice", OCIRepository: "ghcr.io/e/web"}
	if _, err := created.Create(ctx, db); err != nil {
		t.Fatal(err)
	}
	if created.Engine != "timoni" {
		t.Fatalf("Create did not normalize empty engine, stored %q", created.Engine)
	}

	// Found by empty engine ("" → timoni)…
	var byEmpty ComponentDefinition
	if err := byEmpty.GetOwnRow(ctx, db, org, "webservice", ""); err != nil {
		t.Fatalf("GetOwnRow(engine=\"\") could not find the row: %v", err)
	}
	// …and by the explicit "timoni" form — same row.
	var byTimoni ComponentDefinition
	if err := byTimoni.GetOwnRow(ctx, db, org, "webservice", "timoni"); err != nil {
		t.Fatalf("GetOwnRow(engine=\"timoni\") could not find the row: %v", err)
	}
	if byEmpty.ID != created.ID || byTimoni.ID != created.ID {
		t.Fatalf("empty and timoni forms resolved to different rows: created=%s empty=%s timoni=%s",
			created.ID.Hex(), byEmpty.ID.Hex(), byTimoni.ID.Hex())
	}

	// A default seeded with empty engine reconciles a "timoni" default in
	// place rather than creating a second row (the original duplication).
	seedDef(t, ctx, db, "worker", "timoni", "ghcr.io/e/worker", "1.0.0")
	emptyEngineSeed := &ComponentDefinition{Type: "worker", OCIRepository: "ghcr.io/e/worker", OCITag: "2.0.0"}
	if err := emptyEngineSeed.UpsertSeed(ctx, db); err != nil {
		t.Fatal(err)
	}
	defs, err := ResolveForOrg(ctx, db, DefaultComponentDefinitionOwner)
	if err != nil {
		t.Fatal(err)
	}
	workers := 0
	for _, d := range defs {
		if d.Type == "worker" {
			workers++
			if d.OCITag != "2.0.0" {
				t.Fatalf("empty-engine re-seed did not update in place: %+v", d)
			}
		}
	}
	if workers != 1 {
		t.Fatalf("empty-engine seed duplicated the timoni default: got %d worker rows", workers)
	}
}
