package models

import (
	"context"
	"errors"
	"testing"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/coffeenights/conure/cmd/api-server/conureerrors"
)

func TestComponentRevision_AutoIncrementsVersion(t *testing.T) {
	db, err := SetupDB()
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	componentID := primitive.NewObjectID()
	envID := "env-aaaa"

	for i := 1; i <= 3; i++ {
		rev := &ComponentRevision{
			ComponentID:   componentID,
			EnvironmentID: envID,
		}
		if err := rev.CreateDraft(ctx, db); err != nil {
			t.Fatalf("create v%d: %v", i, err)
		}
		if rev.Version != i {
			t.Errorf("expected version %d, got %d", i, rev.Version)
		}
	}

	t.Cleanup(func() {
		_, _ = DeleteAllRevisionsForComponent(ctx, db, componentID)
	})
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
	if err := draft.CreateDraft(ctx, db); err != nil {
		t.Fatal(err)
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

func TestComponentRevision_MarkDeployedIsImmutable(t *testing.T) {
	db, err := SetupDB()
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	componentID := primitive.NewObjectID()
	envID := "env-cccc"
	t.Cleanup(func() { _, _ = DeleteAllRevisionsForComponent(ctx, db, componentID) })

	rev := &ComponentRevision{ComponentID: componentID, EnvironmentID: envID}
	if err := rev.CreateDraft(ctx, db); err != nil {
		t.Fatal(err)
	}
	if err := rev.MarkDeployed(ctx, db); err != nil {
		t.Fatal(err)
	}
	if rev.Status != RevisionStatusDeployed {
		t.Fatalf("expected deployed, got %s", rev.Status)
	}
	// MarkDeployed twice must not succeed (it filters by status=draft).
	err = rev.MarkDeployed(ctx, db)
	if !errors.Is(err, conureerrors.ErrObjectNotFound) {
		t.Errorf("expected ErrObjectNotFound on second MarkDeployed, got %v", err)
	}
	// UpdateDraft on a deployed rev must be rejected.
	err = rev.UpdateDraft(ctx, db, map[string]interface{}{"x": 1})
	if !errors.Is(err, conureerrors.ErrNotAllowed) {
		t.Errorf("expected ErrNotAllowed on UpdateDraft of deployed, got %v", err)
	}
}

func TestComponentRevision_DeleteDraftsForPair(t *testing.T) {
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
	for i := 0; i < 2; i++ {
		draft := &ComponentRevision{ComponentID: componentID, EnvironmentID: envID}
		if err := draft.CreateDraft(ctx, db); err != nil {
			t.Fatal(err)
		}
	}

	deleted, err := DeleteDraftsForPair(ctx, db, componentID, envID)
	if err != nil {
		t.Fatal(err)
	}
	if deleted != 2 {
		t.Errorf("expected to delete 2 drafts, got %d", deleted)
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
		if err := rev.CreateDraft(ctx, db); err != nil {
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
