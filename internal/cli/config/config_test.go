package config

import (
	"os"
	"path/filepath"
	"testing"
)

// withFakeHome points HOME at a fresh tempdir for the duration of the test
// so Load/Save read and write into an isolated location. Tests run in
// parallel safely because t.Setenv handles the per-test reset.
func withFakeHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	return home
}

func TestSaveLoadRoundTrip(t *testing.T) {
	withFakeHome(t)

	orig := &Config{
		Server:    "https://api.example.com",
		Token:     "secret-token",
		ActiveOrg: "org-1",
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

func TestSave_FilePermissions(t *testing.T) {
	home := withFakeHome(t)

	if err := Save(&Config{Server: "x", Token: "y"}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	info, err := os.Stat(filepath.Join(home, ".conure", "config.json"))
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	// 0600: tokens are sensitive; readable group/other would be a regression.
	if perm := info.Mode().Perm(); perm != 0600 {
		t.Errorf("config.json perm = %o, want 0600", perm)
	}
}

func TestLoad_MissingFile(t *testing.T) {
	withFakeHome(t)
	if _, err := Load(); err == nil {
		t.Fatal("expected error when config file is missing")
	}
}

func TestRequireAuth_NoConfig(t *testing.T) {
	withFakeHome(t)
	if _, err := RequireAuth(""); err == nil {
		t.Fatal("expected error when not logged in")
	}
}

func TestRequireAuth_EmptyToken(t *testing.T) {
	withFakeHome(t)
	if err := Save(&Config{Server: "https://x", Token: ""}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if _, err := RequireAuth(""); err == nil {
		t.Fatal("expected error when token is empty")
	}
}

func TestRequireAuth_EmptyServer(t *testing.T) {
	withFakeHome(t)
	if err := Save(&Config{Server: "", Token: "tok"}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if _, err := RequireAuth(""); err == nil {
		t.Fatal("expected error when server is empty")
	}
}

func TestRequireAuth_HappyPath(t *testing.T) {
	withFakeHome(t)
	if err := Save(&Config{Server: "https://x", Token: "tok"}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	cfg, err := RequireAuth("")
	if err != nil {
		t.Fatalf("RequireAuth: %v", err)
	}
	if cfg.Server != "https://x" || cfg.Token != "tok" {
		t.Errorf("unexpected config: %+v", cfg)
	}
}

func TestRequireAuth_ServerOverride(t *testing.T) {
	withFakeHome(t)
	if err := Save(&Config{Server: "https://stored", Token: "tok"}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	cfg, err := RequireAuth("https://override")
	if err != nil {
		t.Fatalf("RequireAuth: %v", err)
	}
	if cfg.Server != "https://override" {
		t.Errorf("Server = %q, want override to win", cfg.Server)
	}
}

func TestRequireActiveOrg_Missing(t *testing.T) {
	withFakeHome(t)
	if err := Save(&Config{Server: "https://x", Token: "tok"}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if _, err := RequireActiveOrg(""); err == nil {
		t.Fatal("expected error when ActiveOrg is empty")
	}
}

func TestRequireActiveOrg_Present(t *testing.T) {
	withFakeHome(t)
	if err := Save(&Config{Server: "https://x", Token: "tok", ActiveOrg: "org-1"}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	cfg, err := RequireActiveOrg("")
	if err != nil {
		t.Fatalf("RequireActiveOrg: %v", err)
	}
	if cfg.ActiveOrg != "org-1" {
		t.Errorf("ActiveOrg = %q, want org-1", cfg.ActiveOrg)
	}
}
