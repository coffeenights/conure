package models

import (
	"context"
	"testing"
	"time"

	_ "github.com/joho/godotenv/autoload"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func newTestBuild() *Build {
	return &Build{
		ComponentID:   primitive.NewObjectID(),
		ApplicationID: primitive.NewObjectID(),
		EnvironmentID: "env-1",
		Status:        BuildStatusPending,
		BuildTool:     BuildToolDockerfile,
		BuildLocation: BuildLocationRemote,
		GitRepository: "https://github.com/example/repo",
		GitBranch:     "main",
		ImageRef:      "ghcr.io/example/app:tag",
	}
}

func TestBuild_CreateAndGetByID(t *testing.T) {
	db, err := SetupDB()
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	b := newTestBuild()
	if err := Create(ctx, db, b); err != nil {
		t.Fatalf("create: %v", err)
	}

	got, err := GetBuildByID(ctx, db, b.ID.Hex())
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.ImageRef != b.ImageRef || got.Status != BuildStatusPending {
		t.Errorf("unexpected build: %+v", got)
	}
}

func TestBuild_MarkTerminal_ClearsLease(t *testing.T) {
	db, err := SetupDB()
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	b := newTestBuild()
	if err := Create(ctx, db, b); err != nil {
		t.Fatalf("create: %v", err)
	}
	// Acquire a lease, then mark terminal — terminal should clear it.
	ok, err := b.TryAcquireLease(ctx, db, "watcher-A", 60*time.Second)
	if err != nil || !ok {
		t.Fatalf("acquire: ok=%v err=%v", ok, err)
	}
	if err := b.MarkTerminal(ctx, db, BuildStatusSucceeded, "ghcr.io/example/app:tag", ""); err != nil {
		t.Fatalf("mark terminal: %v", err)
	}
	got, err := GetBuildByID(ctx, db, b.ID.Hex())
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != BuildStatusSucceeded {
		t.Errorf("status: got %v want succeeded", got.Status)
	}
	if got.WatcherID != "" {
		t.Errorf("watcher id should be cleared: %q", got.WatcherID)
	}
	if got.FinishedAt == nil {
		t.Errorf("finishedAt should be set")
	}
}

func TestBuild_MarkTerminal_PreservesOriginalFinishedAt(t *testing.T) {
	db, err := SetupDB()
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	b := newTestBuild()
	if err := Create(ctx, db, b); err != nil {
		t.Fatalf("create: %v", err)
	}

	if err := b.MarkTerminal(ctx, db, BuildStatusSucceeded, "ghcr.io/example/app:tag", ""); err != nil {
		t.Fatalf("first mark terminal: %v", err)
	}
	first, err := GetBuildByID(ctx, db, b.ID.Hex())
	if err != nil || first.FinishedAt == nil {
		t.Fatalf("first read: %v finishedAt=%v", err, first.FinishedAt)
	}
	original := *first.FinishedAt

	// Ensure wall-clock advances past Mongo's millisecond resolution
	// before the second call so an accidental overwrite would be visible.
	time.Sleep(10 * time.Millisecond)

	if err := b.MarkTerminal(ctx, db, BuildStatusSucceeded, "ghcr.io/example/app:tag", ""); err != nil {
		t.Fatalf("second mark terminal: %v", err)
	}
	second, err := GetBuildByID(ctx, db, b.ID.Hex())
	if err != nil || second.FinishedAt == nil {
		t.Fatalf("second read: %v finishedAt=%v", err, second.FinishedAt)
	}
	if !second.FinishedAt.Equal(original) {
		t.Errorf("finishedAt was rewritten on idempotent re-mark: first=%v second=%v", original, *second.FinishedAt)
	}
	// In-memory copy should reflect the original timestamp too.
	if b.FinishedAt == nil || !b.FinishedAt.Equal(original) {
		t.Errorf("in-memory FinishedAt drifted from canonical: have %v, want %v", b.FinishedAt, original)
	}
}

func TestBuild_TryAcquireLease_RaceLoserFails(t *testing.T) {
	db, err := SetupDB()
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	b := newTestBuild()
	if err := Create(ctx, db, b); err != nil {
		t.Fatal(err)
	}
	// First replica wins.
	bA := *b
	ok, err := bA.TryAcquireLease(ctx, db, "replica-A", 60*time.Second)
	if err != nil || !ok {
		t.Fatalf("A acquire: ok=%v err=%v", ok, err)
	}
	// Second replica reads independently and tries. Lease is still
	// fresh — must lose.
	bB, err := GetBuildByID(ctx, db, b.ID.Hex())
	if err != nil {
		t.Fatal(err)
	}
	ok2, err := bB.TryAcquireLease(ctx, db, "replica-B", 60*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if ok2 {
		t.Errorf("replica-B should have lost the race")
	}
}

func TestBuild_TryAcquireLease_ExpiredAllowsTakeover(t *testing.T) {
	db, err := SetupDB()
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	b := newTestBuild()
	if err := Create(ctx, db, b); err != nil {
		t.Fatal(err)
	}
	// Acquire with a tiny TTL so we don't have to wait.
	ok, err := b.TryAcquireLease(ctx, db, "replica-A", 50*time.Millisecond)
	if err != nil || !ok {
		t.Fatalf("A acquire: ok=%v err=%v", ok, err)
	}
	time.Sleep(80 * time.Millisecond) // wait past TTL

	bB, err := GetBuildByID(ctx, db, b.ID.Hex())
	if err != nil {
		t.Fatal(err)
	}
	ok2, err := bB.TryAcquireLease(ctx, db, "replica-B", 60*time.Second)
	if err != nil {
		t.Fatalf("B acquire: %v", err)
	}
	if !ok2 {
		t.Errorf("replica-B should have adopted the expired lease")
	}
	// Read-back: the document should now name replica-B.
	got, err := GetBuildByID(ctx, db, b.ID.Hex())
	if err != nil {
		t.Fatal(err)
	}
	if got.WatcherID != "replica-B" {
		t.Errorf("watcherID: got %q want replica-B", got.WatcherID)
	}
}

func TestBuild_Heartbeat_LostLeaseReturnsFalse(t *testing.T) {
	db, err := SetupDB()
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	b := newTestBuild()
	if err := Create(ctx, db, b); err != nil {
		t.Fatal(err)
	}
	ok, err := b.TryAcquireLease(ctx, db, "replica-A", 50*time.Millisecond)
	if err != nil || !ok {
		t.Fatalf("A acquire: ok=%v err=%v", ok, err)
	}
	time.Sleep(80 * time.Millisecond)
	// Replica-B takes over.
	bB, err := GetBuildByID(ctx, db, b.ID.Hex())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := bB.TryAcquireLease(ctx, db, "replica-B", 60*time.Second); err != nil {
		t.Fatal(err)
	}
	// Original A tries to heartbeat — must fail.
	ok2, err := b.Heartbeat(ctx, db, "replica-A", 60*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if ok2 {
		t.Errorf("replica-A heartbeat should fail after B took over")
	}
}

func TestBuild_AdoptableBuilds_FiltersTerminalAndFreshLeases(t *testing.T) {
	db, err := SetupDB()
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	// Three builds:
	//  - succeeded (terminal, skipped)
	//  - building with fresh lease (skipped)
	//  - building with expired lease (adoptable)
	terminal := newTestBuild()
	terminal.Status = BuildStatusSucceeded
	terminal.JobName = "j-terminal"
	if err := Create(ctx, db, terminal); err != nil {
		t.Fatal(err)
	}

	fresh := newTestBuild()
	fresh.Status = BuildStatusBuilding
	fresh.JobName = "j-fresh"
	if err := Create(ctx, db, fresh); err != nil {
		t.Fatal(err)
	}
	if _, err := fresh.TryAcquireLease(ctx, db, "replica-A", time.Hour); err != nil {
		t.Fatal(err)
	}

	orphan := newTestBuild()
	orphan.Status = BuildStatusBuilding
	orphan.JobName = "j-orphan"
	if err := Create(ctx, db, orphan); err != nil {
		t.Fatal(err)
	}
	if _, err := orphan.TryAcquireLease(ctx, db, "replica-A", 50*time.Millisecond); err != nil {
		t.Fatal(err)
	}
	time.Sleep(80 * time.Millisecond)

	adopt, err := AdoptableBuilds(ctx, db, time.Now(), 50)
	if err != nil {
		t.Fatal(err)
	}
	foundOrphan := false
	for _, b := range adopt {
		if b.ID == terminal.ID {
			t.Errorf("terminal build should not be adoptable")
		}
		if b.ID == fresh.ID {
			t.Errorf("fresh-lease build should not be adoptable")
		}
		if b.ID == orphan.ID {
			foundOrphan = true
		}
	}
	if !foundOrphan {
		t.Errorf("expired-lease build should be adoptable")
	}
}

func TestBuild_AdoptableBuilds_SkipsLocalBuildsAndJobless(t *testing.T) {
	db, err := SetupDB()
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	// Local builds shouldn't be adopted — they have no Job to watch.
	local := newTestBuild()
	local.BuildLocation = BuildLocationLocal
	local.Status = BuildStatusBuilding
	if err := Create(ctx, db, local); err != nil {
		t.Fatal(err)
	}
	// Remote build that hasn't had its Job recorded yet.
	jobless := newTestBuild()
	jobless.JobName = ""
	if err := Create(ctx, db, jobless); err != nil {
		t.Fatal(err)
	}

	adopt, err := AdoptableBuilds(ctx, db, time.Now(), 50)
	if err != nil {
		t.Fatal(err)
	}
	for _, b := range adopt {
		if b.ID == local.ID {
			t.Errorf("local build should not be adoptable")
		}
		if b.ID == jobless.ID {
			t.Errorf("build without jobName should not be adoptable")
		}
	}
}

func TestBuild_ListBuildsForComponent_NewestFirst(t *testing.T) {
	db, err := SetupDB()
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	componentID := primitive.NewObjectID()

	b1 := newTestBuild()
	b1.ComponentID = componentID
	b1.EnvironmentID = "env-listing-test"
	if err := Create(ctx, db, b1); err != nil {
		t.Fatal(err)
	}
	// Stretch the CreatedAt apart so the sort is unambiguous.
	time.Sleep(10 * time.Millisecond)
	b2 := newTestBuild()
	b2.ComponentID = componentID
	b2.EnvironmentID = "env-listing-test"
	if err := Create(ctx, db, b2); err != nil {
		t.Fatal(err)
	}

	got, err := ListBuildsForComponent(ctx, db, componentID, "env-listing-test", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("len=%d want 2", len(got))
	}
	if !got[0].CreatedAt.After(got[1].CreatedAt) {
		t.Errorf("results should be newest-first")
	}
}
