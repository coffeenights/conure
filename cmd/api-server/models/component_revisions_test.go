package models

import (
	"context"
	"errors"
	"testing"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/coffeenights/conure/cmd/api-server/conureerrors"
)

// Deployed revisions form a dense 1,2,3… trail. Drafts carry version 0 and
// do NOT consume a number, so interleaving an upserted draft must not
// perturb the deployed sequence.
func TestComponentRevision_DeployedVersionsAreDenseAndDraftFree(t *testing.T) {
	db, err := SetupDB()
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	componentID := primitive.NewObjectID()
	envID := "env-aaaa"
	t.Cleanup(func() { _, _ = DeleteAllRevisionsForComponent(ctx, db, componentID) })

	for i := 1; i <= 3; i++ {
		dep := &ComponentRevision{ComponentID: componentID, EnvironmentID: envID}
		if err := dep.CreateDeployed(ctx, db); err != nil {
			t.Fatalf("create deployed #%d: %v", i, err)
		}
		if dep.Version != i {
			t.Errorf("expected deployed version %d, got %d", i, dep.Version)
		}
		// An interleaved draft must not advance the deployed counter.
		dr := &ComponentRevision{ComponentID: componentID, EnvironmentID: envID}
		if err := dr.UpsertDraft(ctx, db); err != nil {
			t.Fatalf("upsert draft after deployed #%d: %v", i, err)
		}
		if dr.Version != 0 {
			t.Errorf("draft must have version 0, got %d", dr.Version)
		}
	}
}

// UpsertDraft enforces at most one draft per (component, env): a second
// upsert updates the same row in place rather than creating another.
func TestComponentRevision_UpsertDraftIsSingle(t *testing.T) {
	db, err := SetupDB()
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	componentID := primitive.NewObjectID()
	envID := "env-aaab"
	t.Cleanup(func() { _, _ = DeleteAllRevisionsForComponent(ctx, db, componentID) })

	first := &ComponentRevision{ComponentID: componentID, EnvironmentID: envID, Values: map[string]interface{}{"n": float64(1)}}
	if err := first.UpsertDraft(ctx, db); err != nil {
		t.Fatal(err)
	}
	second := &ComponentRevision{ComponentID: componentID, EnvironmentID: envID, Values: map[string]interface{}{"n": float64(2)}, Comment: "edited"}
	if err := second.UpsertDraft(ctx, db); err != nil {
		t.Fatal(err)
	}
	if second.ID != first.ID {
		t.Errorf("expected upsert to reuse the single draft row %v, got %v", first.ID, second.ID)
	}

	revs, err := ListRevisions(ctx, db, componentID, envID)
	if err != nil {
		t.Fatal(err)
	}
	if len(revs) != 1 {
		t.Fatalf("expected exactly 1 draft row, got %d: %+v", len(revs), revs)
	}
	if got := revs[0].Values["n"]; got != float64(2) || revs[0].Comment != "edited" {
		t.Errorf("draft not updated in place: %+v", revs[0])
	}
}

func TestComponentRevision_LatestDraftAndDeployed(t *testing.T) {
	db, err := SetupDB()
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	componentID := primitive.NewObjectID()
	envID := "env-bbbb"
	t.Cleanup(func() { _, _ = DeleteAllRevisionsForComponent(ctx, db, componentID) })

	deployed := &ComponentRevision{ComponentID: componentID, EnvironmentID: envID}
	if err := deployed.CreateDeployed(ctx, db); err != nil {
		t.Fatal(err)
	}
	draft := &ComponentRevision{ComponentID: componentID, EnvironmentID: envID}
	if err := draft.UpsertDraft(ctx, db); err != nil {
		t.Fatal(err)
	}
	if draft.Version != 0 {
		t.Errorf("draft must have version 0, got %d", draft.Version)
	}

	gotDraft, err := LatestDraft(ctx, db, componentID, envID)
	if err != nil {
		t.Fatal(err)
	}
	if gotDraft.ID != draft.ID {
		t.Errorf("LatestDraft returned wrong row: %v vs %v", gotDraft.ID, draft.ID)
	}
	gotDep, err := LatestDeployed(ctx, db, componentID, envID)
	if err != nil {
		t.Fatal(err)
	}
	if gotDep.ID != deployed.ID {
		t.Errorf("LatestDeployed returned wrong row: %v vs %v", gotDep.ID, deployed.ID)
	}
}

func TestComponentRevision_DeployedHistoryIsImmutable(t *testing.T) {
	db, err := SetupDB()
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	componentID := primitive.NewObjectID()
	envID := "env-cccc"
	t.Cleanup(func() { _, _ = DeleteAllRevisionsForComponent(ctx, db, componentID) })

	// Deployed revisions are born deployed and never mutated in place — the
	// deploy paths CreateDeployed a fresh row and SupersedeDrafts the rest.
	rev := &ComponentRevision{ComponentID: componentID, EnvironmentID: envID}
	if err := rev.CreateDeployed(ctx, db); err != nil {
		t.Fatal(err)
	}
	if rev.Status != RevisionStatusDeployed {
		t.Fatalf("expected deployed, got %s", rev.Status)
	}
	// UpdateDraft on a deployed rev must be rejected.
	err = rev.UpdateDraft(ctx, db, map[string]interface{}{"x": 1}, "")
	if !errors.Is(err, conureerrors.ErrNotAllowed) {
		t.Errorf("expected ErrNotAllowed on UpdateDraft of deployed, got %v", err)
	}
}

func TestComponentRevision_SupersedeDrafts(t *testing.T) {
	db, err := SetupDB()
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	componentID := primitive.NewObjectID()
	envID := "env-dddd"
	t.Cleanup(func() { _, _ = DeleteAllRevisionsForComponent(ctx, db, componentID) })

	deployed := &ComponentRevision{ComponentID: componentID, EnvironmentID: envID}
	if err := deployed.CreateDeployed(ctx, db); err != nil {
		t.Fatal(err)
	}
	// At most one draft can exist; SupersedeDrafts must clear it and leave
	// the deployed history intact.
	draft := &ComponentRevision{ComponentID: componentID, EnvironmentID: envID}
	if err := draft.UpsertDraft(ctx, db); err != nil {
		t.Fatal(err)
	}

	deleted, err := SupersedeDrafts(ctx, db, componentID, envID)
	if err != nil {
		t.Fatal(err)
	}
	if deleted != 1 {
		t.Errorf("expected to delete 1 draft, got %d", deleted)
	}
	revisions, err := ListRevisions(ctx, db, componentID, envID)
	if err != nil {
		t.Fatal(err)
	}
	if len(revisions) != 1 || revisions[0].Status != RevisionStatusDeployed {
		t.Errorf("expected 1 deployed revision after purge, got %+v", revisions)
	}
}

func TestComponentRevision_EnvironmentsWithRevisions(t *testing.T) {
	db, err := SetupDB()
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	componentID := primitive.NewObjectID()
	t.Cleanup(func() { _, _ = DeleteAllRevisionsForComponent(ctx, db, componentID) })

	envs := []string{"env-1111", "env-2222"}
	for _, envID := range envs {
		rev := &ComponentRevision{ComponentID: componentID, EnvironmentID: envID}
		if err := rev.UpsertDraft(ctx, db); err != nil {
			t.Fatal(err)
		}
	}
	got, err := EnvironmentsWithRevisions(ctx, db, componentID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Errorf("expected 2 envs, got %d (%v)", len(got), got)
	}
}
