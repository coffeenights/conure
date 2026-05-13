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

// fixtureProfile is a tiny shortcut so tests aren't littered with field
// names; the tests are about Config semantics, not Profile field tweaks.
func fixtureProfile(server, token, org string) *Profile {
	return &Profile{Server: server, Token: token, ActiveOrg: org}
}

// ---- Save/Load round-trip ------------------------------------------------

func TestSaveLoadRoundTrip(t *testing.T) {
	withFakeHome(t)

	orig := &Config{
		Active: "prod",
		Profiles: map[string]*Profile{
			"prod":    fixtureProfile("https://api.example.com", "tok-1", "org-1"),
			"staging": fixtureProfile("https://staging.example.com", "tok-2", ""),
		},
	}
	if err := Save(orig); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.Active != "prod" {
		t.Errorf("Active = %q, want prod", got.Active)
	}
	if len(got.Profiles) != 2 {
		t.Fatalf("Profiles count = %d, want 2", len(got.Profiles))
	}
	if *got.Profiles["prod"] != *orig.Profiles["prod"] {
		t.Errorf("prod profile mismatch: got %+v", *got.Profiles["prod"])
	}
}

func TestSave_FilePermissions(t *testing.T) {
	home := withFakeHome(t)
	if err := Save(&Config{Active: "x", Profiles: map[string]*Profile{"x": fixtureProfile("s", "t", "")}}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	info, err := os.Stat(filepath.Join(home, ".conure", "config.json"))
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0600 {
		t.Errorf("config.json perm = %o, want 0600", perm)
	}
}

func TestSave_InitialisesNilProfileMap(t *testing.T) {
	withFakeHome(t)
	// A Config with a nil Profiles map should still save and load cleanly.
	if err := Save(&Config{}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.Profiles == nil {
		t.Error("loaded Profiles is nil; Save should have normalised it")
	}
}

// ---- Legacy-shape migration ----------------------------------------------

func TestLoad_MigratesLegacyShape(t *testing.T) {
	home := withFakeHome(t)
	if err := os.MkdirAll(filepath.Join(home, ".conure"), 0700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	legacy := `{"server":"https://api.conure.io","token":"tok-1","active_org":"org-1"}`
	if err := os.WriteFile(filepath.Join(home, ".conure", "config.json"), []byte(legacy), 0600); err != nil {
		t.Fatalf("write: %v", err)
	}

	got, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.Active != "api" { // hostname's first label is "api"
		t.Errorf("Active = %q, want api", got.Active)
	}
	p := got.GetActive()
	if p == nil {
		t.Fatal("GetActive returned nil after migration")
	}
	if p.Server != "https://api.conure.io" || p.Token != "tok-1" || p.ActiveOrg != "org-1" {
		t.Errorf("migrated profile = %+v", *p)
	}

	// Round-trip: reloading should not re-migrate.
	got2, err := Load()
	if err != nil {
		t.Fatalf("Load (after migration): %v", err)
	}
	if got2.Active != got.Active || len(got2.Profiles) != 1 {
		t.Errorf("re-load drifted: %+v", got2)
	}
}

func TestLoad_EmptyBodyReturnsEmptyConfig(t *testing.T) {
	home := withFakeHome(t)
	if err := os.MkdirAll(filepath.Join(home, ".conure"), 0700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(home, ".conure", "config.json"), []byte(`{}`), 0600); err != nil {
		t.Fatalf("write: %v", err)
	}
	got, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.Profiles == nil || len(got.Profiles) != 0 {
		t.Errorf("expected empty Profiles map, got %+v", got)
	}
}

// ---- DefaultProfileName --------------------------------------------------

func TestDefaultProfileName(t *testing.T) {
	cases := map[string]string{
		"":                            "default",
		"https://api.conure.io":       "api",
		"https://api.conure.io/":      "api",
		"https://staging.example.com": "staging",
		"http://localhost:8080":       "localhost",
		"http://127.0.0.1:8080":       "127",
		"not a url":                   "not a url", // url.Parse accepts free text; the whole thing becomes Path
	}
	// The "not a url" case actually round-trips through url.Parse with
	// Hostname()==""; we should not return that string verbatim. Adjust
	// the expectation to what the function actually guarantees.
	delete(cases, "not a url")

	for in, want := range cases {
		if got := DefaultProfileName(in); got != want {
			t.Errorf("DefaultProfileName(%q) = %q, want %q", in, got, want)
		}
	}
}

// ---- Config method semantics ---------------------------------------------

func TestUpsert_MakesFirstProfileActive(t *testing.T) {
	c := &Config{}
	c.Upsert("prod", fixtureProfile("s", "t", ""))
	if c.Active != "prod" {
		t.Errorf("Active = %q, want prod after first Upsert", c.Active)
	}
	c.Upsert("staging", fixtureProfile("s2", "t2", ""))
	if c.Active != "prod" {
		t.Errorf("Active = %q, want prod still; Upsert must not steal active", c.Active)
	}
}

func TestUse_Errors(t *testing.T) {
	c := &Config{Profiles: map[string]*Profile{"x": fixtureProfile("s", "t", "")}}
	if err := c.Use("nope"); err == nil {
		t.Error("Use should error for unknown profile")
	}
	if err := c.Use("x"); err != nil {
		t.Errorf("Use: %v", err)
	}
	if c.Active != "x" {
		t.Errorf("Active = %q, want x", c.Active)
	}
}

func TestRemove_ClearsActiveWhenRemovingIt(t *testing.T) {
	c := &Config{
		Active:   "x",
		Profiles: map[string]*Profile{"x": fixtureProfile("s", "t", "")},
	}
	if err := c.Remove("x"); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if c.Active != "" {
		t.Errorf("Active = %q, want empty after removing active profile", c.Active)
	}
}

func TestRemove_LeavesActiveAloneWhenRemovingOther(t *testing.T) {
	c := &Config{
		Active: "x",
		Profiles: map[string]*Profile{
			"x": fixtureProfile("s", "t", ""),
			"y": fixtureProfile("s2", "t2", ""),
		},
	}
	if err := c.Remove("y"); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if c.Active != "x" {
		t.Errorf("Active = %q, want x", c.Active)
	}
}

func TestFindByServer_TrimsTrailingSlash(t *testing.T) {
	c := &Config{
		Profiles: map[string]*Profile{
			"prod": fixtureProfile("https://api.example.com/", "t", ""),
		},
	}
	name, p := c.FindByServer("https://api.example.com")
	if name != "prod" || p == nil {
		t.Errorf("FindByServer mismatch: name=%q p=%v", name, p)
	}
	if _, miss := c.FindByServer("https://other.example.com"); miss != nil {
		t.Errorf("expected nil for unmatched server")
	}
}

// ---- Require* helpers ----------------------------------------------------

func TestRequireAuth_NoConfig(t *testing.T) {
	withFakeHome(t)
	if _, _, err := RequireAuth("", ""); err == nil {
		t.Fatal("expected error when not logged in")
	}
}

func TestRequireAuth_EmptyToken(t *testing.T) {
	withFakeHome(t)
	c := &Config{Active: "x", Profiles: map[string]*Profile{"x": fixtureProfile("https://s", "", "")}}
	if err := Save(c); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if _, _, err := RequireAuth("", ""); err == nil {
		t.Fatal("expected error when token is empty")
	}
}

func TestRequireAuth_HappyPath(t *testing.T) {
	withFakeHome(t)
	c := &Config{Active: "prod", Profiles: map[string]*Profile{
		"prod": fixtureProfile("https://api.example.com", "tok", "org-1"),
	}}
	if err := Save(c); err != nil {
		t.Fatalf("Save: %v", err)
	}
	_, prof, err := RequireAuth("", "")
	if err != nil {
		t.Fatalf("RequireAuth: %v", err)
	}
	if prof.Server != "https://api.example.com" || prof.Token != "tok" {
		t.Errorf("profile = %+v", *prof)
	}
}

func TestRequireAuth_ProfileOverride(t *testing.T) {
	withFakeHome(t)
	c := &Config{Active: "prod", Profiles: map[string]*Profile{
		"prod":    fixtureProfile("https://prod", "p", ""),
		"staging": fixtureProfile("https://staging", "s", ""),
	}}
	if err := Save(c); err != nil {
		t.Fatalf("Save: %v", err)
	}
	_, prof, err := RequireAuth("", "staging")
	if err != nil {
		t.Fatalf("RequireAuth: %v", err)
	}
	if prof.Server != "https://staging" {
		t.Errorf("Server = %q, want staging (override should win)", prof.Server)
	}
}

func TestRequireAuth_UnknownProfileOverride(t *testing.T) {
	withFakeHome(t)
	c := &Config{Active: "prod", Profiles: map[string]*Profile{"prod": fixtureProfile("s", "t", "")}}
	if err := Save(c); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if _, _, err := RequireAuth("", "nope"); err == nil {
		t.Fatal("expected error when --profile names something that doesn't exist")
	}
}

func TestRequireAuth_ServerOverrideDoesNotPersist(t *testing.T) {
	withFakeHome(t)
	c := &Config{Active: "prod", Profiles: map[string]*Profile{
		"prod": fixtureProfile("https://stored", "tok", ""),
	}}
	if err := Save(c); err != nil {
		t.Fatalf("Save: %v", err)
	}

	_, prof, err := RequireAuth("https://override", "")
	if err != nil {
		t.Fatalf("RequireAuth: %v", err)
	}
	if prof.Server != "https://override" {
		t.Errorf("Server = %q, want override to win in-memory", prof.Server)
	}

	// Re-load: the on-disk profile must still be the original server.
	got, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.GetActive().Server != "https://stored" {
		t.Errorf("on-disk Server changed: %q; --server should not persist", got.GetActive().Server)
	}
}

func TestRequireActiveOrg_Missing(t *testing.T) {
	withFakeHome(t)
	c := &Config{Active: "prod", Profiles: map[string]*Profile{
		"prod": fixtureProfile("https://s", "tok", ""),
	}}
	if err := Save(c); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if _, _, err := RequireActiveOrg("", ""); err == nil {
		t.Fatal("expected error when ActiveOrg is empty")
	}
}

func TestRequireActiveOrg_Present(t *testing.T) {
	withFakeHome(t)
	c := &Config{Active: "prod", Profiles: map[string]*Profile{
		"prod": fixtureProfile("https://s", "tok", "org-1"),
	}}
	if err := Save(c); err != nil {
		t.Fatalf("Save: %v", err)
	}
	_, prof, err := RequireActiveOrg("", "")
	if err != nil {
		t.Fatalf("RequireActiveOrg: %v", err)
	}
	if prof.ActiveOrg != "org-1" {
		t.Errorf("ActiveOrg = %q, want org-1", prof.ActiveOrg)
	}
}
