package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/coffeenights/conure/internal/cli/apiclient"
	"github.com/coffeenights/conure/pkg/api"
)

// resolveComponent / resolveAppTarget match the --component / --app flag by
// id OR name within an *already-resolved* scope. The interesting edge is
// name collisions:
//
//   - two components with the same name in the SAME app (the picker can't
//     disambiguate by name; first match must win deterministically),
//   - a component name that exists in a DIFFERENT app but not the resolved
//     one (must NOT leak across apps — the lookup is scoped to app.ID),
//   - the same two cases for --app within an org.
//
// These tests pin that behavior so a future refactor of the matching loop
// can't silently start picking a different component or matching across
// apps.

// fakeAPI serves the minimal endpoints resolveComponent/resolveAppTarget hit:
// list apps, get app, list components. Routes are keyed by org/app id baked
// into the fixtures below.
func fakeAPI(t *testing.T, apps []api.Application, compsByApp map[string][]api.Component) *apiclient.Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		p := r.URL.Path
		switch {
		// GET /organizations/{org}/a/{app}/c  -> components in app
		case strings.HasSuffix(p, "/c"):
			seg := strings.Split(strings.Trim(p, "/"), "/")
			appID := seg[len(seg)-2]
			_ = json.NewEncoder(w).Encode(api.ComponentListResponse{
				Components: compsByApp[appID],
			})
		// GET /organizations/{org}/a/{app}    -> single app detail
		// (path has 4 segments: organizations/{org}/a/{app})
		case strings.Contains(p, "/a/"):
			seg := strings.Split(strings.Trim(p, "/"), "/")
			appID := seg[len(seg)-1]
			for _, a := range apps {
				if a.ID == appID {
					_ = json.NewEncoder(w).Encode(a)
					return
				}
			}
			http.Error(w, `{"message":"not found"}`, http.StatusNotFound)
		// GET /organizations/{org}/a          -> app list
		case strings.HasSuffix(p, "/a"):
			_ = json.NewEncoder(w).Encode(api.ApplicationListResponse{Applications: apps})
		default:
			http.Error(w, `{"message":"unexpected path: `+p+`"}`, http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	return apiclient.New(srv.URL, "test-token")
}

func newCmd() *cobra.Command {
	// resolveComponent/resolveAppTarget only call cmd.Context(); a bare
	// command with a background context is enough.
	c := &cobra.Command{}
	c.SetContext(context.Background())
	return c
}

func TestResolveComponent_SameNameInSameApp_FirstMatchWins(t *testing.T) {
	app := api.Application{ID: "app1", Name: "web"}
	comps := map[string][]api.Component{
		"app1": {
			{ID: "comp-A", Name: "api"},
			{ID: "comp-B", Name: "api"}, // duplicate name in the same app
		},
	}
	client := fakeAPI(t, []api.Application{app}, comps)

	// Set the package-level flag the resolver reads, restore after.
	defer func() { componentFlag = "" }()
	componentFlag = "api"

	id, name, err := resolveComponent(context.Background(), newCmd(), client, "org1", &app, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Deterministic: the first component in list order must win, so
	// behavior is stable across invocations rather than map-order random.
	if id != "comp-A" || name != "api" {
		t.Errorf("got (%q,%q), want first match (comp-A,api)", id, name)
	}
}

func TestResolveComponent_NameOnlyInOtherApp_DoesNotLeak(t *testing.T) {
	resolved := api.Application{ID: "app1", Name: "web"}
	comps := map[string][]api.Component{
		"app1": {{ID: "comp-1", Name: "frontend"}},
		// "api" exists, but only under a different app the user didn't pick.
		"app2": {{ID: "comp-2", Name: "api"}},
	}
	client := fakeAPI(t, []api.Application{resolved}, comps)

	defer func() { componentFlag = "" }()
	componentFlag = "api"

	_, _, err := resolveComponent(context.Background(), newCmd(), client, "org1", &resolved, nil)
	if err == nil {
		t.Fatal("expected an error: 'api' is not in the resolved app")
	}
	if !strings.Contains(err.Error(), `no component matches "api"`) {
		t.Errorf("error should name the unmatched component, got: %v", err)
	}
}

func TestResolveComponent_NameMatchIsCaseInsensitive(t *testing.T) {
	app := api.Application{ID: "app1", Name: "web"}
	comps := map[string][]api.Component{
		"app1": {{ID: "comp-1", Name: "API"}},
	}
	client := fakeAPI(t, []api.Application{app}, comps)

	defer func() { componentFlag = "" }()
	componentFlag = "api" // lowercased; component is "API"

	id, _, err := resolveComponent(context.Background(), newCmd(), client, "org1", &app, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "comp-1" {
		t.Errorf("expected case-insensitive name match, got %q", id)
	}
}

func TestResolveComponent_IDMatchBeatsNameCollision(t *testing.T) {
	app := api.Application{ID: "app1", Name: "web"}
	comps := map[string][]api.Component{
		"app1": {
			{ID: "comp-A", Name: "api"},
			{ID: "comp-B", Name: "api"},
		},
	}
	client := fakeAPI(t, []api.Application{app}, comps)

	defer func() { componentFlag = "" }()
	componentFlag = "comp-B" // exact id targets the second despite name dupes

	id, _, err := resolveComponent(context.Background(), newCmd(), client, "org1", &app, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "comp-B" {
		t.Errorf("id should resolve unambiguously to comp-B, got %q", id)
	}
}

func TestResolveAppTarget_SameNameApps_FirstMatchWins(t *testing.T) {
	apps := []api.Application{
		{ID: "app-A", Name: "web"},
		{ID: "app-B", Name: "web"}, // same name, different app
	}
	client := fakeAPI(t, apps, map[string][]api.Component{})

	defer func() { appFlag = "" }()
	appFlag = "web"

	got, err := resolveAppTarget(context.Background(), newCmd(), client, "org1", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != "app-A" {
		t.Errorf("ambiguous app name should resolve to first match app-A, got %q", got.ID)
	}
}

func TestResolveAppTarget_NoMatchNamesValue(t *testing.T) {
	apps := []api.Application{{ID: "app-A", Name: "web"}}
	client := fakeAPI(t, apps, map[string][]api.Component{})

	defer func() { appFlag = "" }()
	appFlag = "nope"

	_, err := resolveAppTarget(context.Background(), newCmd(), client, "org1", nil)
	if err == nil || !strings.Contains(err.Error(), `no app matches "nope"`) {
		t.Errorf("expected actionable no-match error, got: %v", err)
	}
}
