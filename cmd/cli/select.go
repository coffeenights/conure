package main

import (
	"context"
	"fmt"
	"os"

	"github.com/charmbracelet/huh"
	"golang.org/x/term"

	"github.com/coffeenights/conure/internal/cli/apiclient"
	"github.com/coffeenights/conure/pkg/api"
)

// interactive reports whether stdin is a real terminal. The picker helpers
// below refuse to prompt when it isn't (CI, pipes, `conure ... < /dev/null`)
// so a missing flag/link surfaces as an actionable error instead of a
// process that hangs forever waiting on a TTY that will never answer.
func interactive() bool {
	return term.IsTerminal(int(os.Stdin.Fd()))
}

// errNoTTY is returned (wrapped with context) when a value can't be resolved
// from a flag or the link file and we can't fall back to a prompt because
// there's no terminal. Callers add the "what was missing" prefix.
type errNoTTY struct{ what string }

func (e errNoTTY) Error() string {
	return fmt.Sprintf("no %s: pass the flag or run from a linked directory (no terminal available for interactive selection)", e.what)
}

// selectOrg resolves an organization id. Preference: the profile's already
// chosen org (silent), else — when interactive — a picker over the user's
// orgs. A single org is auto-selected without prompting. The returned id is
// not persisted here; callers that want it sticky save it themselves (see
// resolveTarget), mirroring how `conure init` behaves.
func selectOrg(ctx context.Context, client *apiclient.Client) (string, error) {
	orgs, err := client.ListOrganizations(ctx)
	if err != nil {
		return "", fmt.Errorf("listing organizations: %w", err)
	}
	if len(orgs) == 0 {
		return "", fmt.Errorf("no organizations available — create one in the UI first")
	}
	if len(orgs) == 1 {
		return orgs[0].ID, nil
	}
	if !interactive() {
		return "", errNoTTY{"organization (--org)"}
	}
	options := make([]huh.Option[string], len(orgs))
	for i, o := range orgs {
		options[i] = huh.NewOption(o.Name, o.ID)
	}
	var orgID string
	if err := huh.NewSelect[string]().
		Title("Organization").
		Options(options...).
		Value(&orgID).
		Run(); err != nil {
		return "", err
	}
	return orgID, nil
}

// selectApp resolves an app in orgID. A single app is auto-selected; with
// several, the user picks one (interactive only). Returns the full app so
// callers get its Environments without a second round-trip.
func selectApp(ctx context.Context, client *apiclient.Client, orgID string) (*api.Application, error) {
	apps, err := client.ListApps(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("listing apps: %w", err)
	}
	if len(apps) == 0 {
		return nil, fmt.Errorf("no apps in this organization — run 'conure init' to create one")
	}
	var chosenID string
	if len(apps) == 1 {
		chosenID = apps[0].ID
	} else {
		if !interactive() {
			return nil, errNoTTY{"app (--app)"}
		}
		options := make([]huh.Option[string], len(apps))
		for i, a := range apps {
			options[i] = huh.NewOption(a.Name, a.ID)
		}
		if err := huh.NewSelect[string]().
			Title("App").
			Options(options...).
			Value(&chosenID).
			Run(); err != nil {
			return nil, err
		}
	}
	// ListApps doesn't populate Environments — refetch the detail view so
	// env resolution downstream has them.
	full, err := client.GetApp(ctx, orgID, chosenID)
	if err != nil {
		return nil, fmt.Errorf("loading app: %w", err)
	}
	return full, nil
}

// selectComponent resolves a component in the app. A single component is
// auto-selected; with several, the user picks one (interactive only).
// Returns id + display name so callers can populate a linkedCtx.
func selectComponent(ctx context.Context, client *apiclient.Client, orgID, appID string) (id, name string, err error) {
	comps, err := client.ListAppComponents(ctx, orgID, appID)
	if err != nil {
		return "", "", fmt.Errorf("listing components: %w", err)
	}
	if len(comps) == 0 {
		return "", "", fmt.Errorf("no components in this app — run 'conure init' or 'conure component add'")
	}
	if len(comps) == 1 {
		return comps[0].ID, comps[0].Name, nil
	}
	if !interactive() {
		return "", "", errNoTTY{"component (--component)"}
	}
	options := make([]huh.Option[string], len(comps))
	for i, c := range comps {
		options[i] = huh.NewOption(c.Name, c.ID)
	}
	var chosen string
	if err := huh.NewSelect[string]().
		Title("Component").
		Options(options...).
		Value(&chosen).
		Run(); err != nil {
		return "", "", err
	}
	for _, c := range comps {
		if c.ID == chosen {
			return c.ID, c.Name, nil
		}
	}
	return "", "", fmt.Errorf("internal: selected component %q not found", chosen)
}

// selectEnv resolves an environment name on app. A single env is
// auto-selected; with several, the user picks one (interactive only).
func selectEnv(app *api.Application) (string, error) {
	switch len(app.Environments) {
	case 0:
		return "", fmt.Errorf("app %s has no environments — run 'conure app env create <name>'", app.Name)
	case 1:
		return app.Environments[0].Name, nil
	}
	if !interactive() {
		return "", errNoTTY{"environment (--env)"}
	}
	options := make([]huh.Option[string], len(app.Environments))
	for i, e := range app.Environments {
		options[i] = huh.NewOption(e.Name, e.Name)
	}
	var env string
	if err := huh.NewSelect[string]().
		Title("Environment").
		Options(options...).
		Value(&env).
		Run(); err != nil {
		return "", err
	}
	return env, nil
}
