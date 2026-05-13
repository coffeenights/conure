package link

import (
	"os"
	"path/filepath"
	"testing"
)

// writeLink is a small helper so tests don't reimplement the dir+file dance
// each time they need a fixture link in place.
func writeLink(t *testing.T, dir string, l *Link) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, ".conure"), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".conure", "link.json"), []byte(toJSON(t, l)), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func toJSON(t *testing.T, l *Link) string {
	t.Helper()
	// We marshal via Save's format implicitly by round-tripping through the
	// public API in another test; here we just want a minimal valid file.
	return `{
  "org_id":         "` + l.OrgID + `",
  "app_id":         "` + l.AppID + `",
  "component_id":   "` + l.ComponentID + `",
  "component_name": "` + l.ComponentName + `",
  "environment":    "` + l.Environment + `"
}`
}

func TestPath_FoundInCwd(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	writeLink(t, dir, &Link{OrgID: "o1"})

	got, found, err := Path()
	if err != nil {
		t.Fatalf("Path: %v", err)
	}
	if !found {
		t.Fatal("expected found=true when link is in cwd")
	}
	if want := filepath.Join(dir, relPath); got != want {
		t.Errorf("Path = %q, want %q", got, want)
	}
}

func TestPath_FoundInAncestor(t *testing.T) {
	root := t.TempDir()
	writeLink(t, root, &Link{OrgID: "o1"})

	// Descend two levels and look from there.
	deep := filepath.Join(root, "a", "b")
	if err := os.MkdirAll(deep, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	t.Chdir(deep)

	got, found, err := Path()
	if err != nil {
		t.Fatalf("Path: %v", err)
	}
	if !found {
		t.Fatal("expected found=true when link is in an ancestor")
	}
	if want := filepath.Join(root, relPath); got != want {
		t.Errorf("Path = %q, want %q", got, want)
	}
}

func TestPath_NotFoundReturnsCwdDefault(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	got, found, err := Path()
	if err != nil {
		t.Fatalf("Path: %v", err)
	}
	if found {
		t.Fatal("expected found=false when no link exists")
	}
	if want := filepath.Join(dir, relPath); got != want {
		t.Errorf("Path = %q, want %q", got, want)
	}
}

func TestExists(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	if Exists() {
		t.Fatal("Exists should be false on an empty dir")
	}
	writeLink(t, dir, &Link{OrgID: "o1"})
	if !Exists() {
		t.Fatal("Exists should be true after writing a link")
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	orig := &Link{
		OrgID:         "org-1",
		AppID:         "app-1",
		ComponentID:   "comp-1",
		ComponentName: "api",
		Environment:   "production",
	}
	if err := Save(orig); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if *got != *orig {
		t.Errorf("round-trip mismatch:\n got = %+v\nwant = %+v", *got, *orig)
	}
}

func TestLoad_MissingFile(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	_, err := Load()
	if err == nil {
		t.Fatal("expected error when no link file exists")
	}
}

func TestLoad_MalformedJSON(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	if err := os.MkdirAll(filepath.Join(dir, ".conure"), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".conure", "link.json"), []byte("not json"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	_, err := Load()
	if err == nil {
		t.Fatal("expected error on malformed JSON")
	}
}
