package link

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// writeFile is a small helper so tests don't reimplement the dir+file dance
// each time they need a fixture link file in place.
func writeFile(t *testing.T, dir string, f File) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, ".conure"), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".conure", "link.json"), data, 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func TestPath_FoundInCwd(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	writeFile(t, dir, File{"prod": {OrgID: "o1"}})

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
	writeFile(t, root, File{"prod": {OrgID: "o1"}})

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

func TestFileExists(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	if FileExists() {
		t.Fatal("FileExists should be false on an empty dir")
	}
	writeFile(t, dir, File{"prod": {OrgID: "o1"}})
	if !FileExists() {
		t.Fatal("FileExists should be true after writing a link")
	}
}

func TestExists_PerProfile(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	writeFile(t, dir, File{"prod": {OrgID: "o1"}})

	if !Exists("prod") {
		t.Error("Exists(prod) should be true")
	}
	if Exists("staging") {
		t.Error("Exists(staging) should be false — no entry for it")
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	orig := File{
		"prod": {
			OrgID:         "org-1",
			AppID:         "app-1",
			ComponentID:   "comp-1",
			ComponentName: "api",
			Environment:   "production",
		},
		"staging": {
			OrgID:         "org-2",
			AppID:         "app-2",
			ComponentID:   "comp-2",
			ComponentName: "api",
			Environment:   "staging",
		},
	}
	if err := Save(orig); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(got) != len(orig) {
		t.Fatalf("len = %d, want %d", len(got), len(orig))
	}
	for name, want := range orig {
		gotLink, ok := got[name]
		if !ok {
			t.Errorf("missing profile %q after round-trip", name)
			continue
		}
		if *gotLink != *want {
			t.Errorf("round-trip mismatch for %q:\n got = %+v\nwant = %+v", name, *gotLink, *want)
		}
	}
}

func TestGet_MissingProfile(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	writeFile(t, dir, File{"prod": {OrgID: "o1"}})

	if _, err := Get("staging"); err == nil {
		t.Fatal("expected error when profile is not in the file")
	}
}

func TestLoad_MissingFile(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	if _, err := Load(); err == nil {
		t.Fatal("expected error when no link file exists")
	}
}

func TestLoadOrEmpty_MissingFile(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	f, err := LoadOrEmpty()
	if err != nil {
		t.Fatalf("LoadOrEmpty: %v", err)
	}
	if len(f) != 0 {
		t.Errorf("expected empty map, got %v", f)
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

	if _, err := Load(); err == nil {
		t.Fatal("expected error on malformed JSON")
	}
}
