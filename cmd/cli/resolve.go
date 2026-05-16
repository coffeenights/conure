package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/coffeenights/conure/internal/cli/apiclient"
	"github.com/coffeenights/conure/internal/cli/config"
	"github.com/coffeenights/conure/internal/cli/link"
	"github.com/coffeenights/conure/internal/cli/ui"
	"github.com/coffeenights/conure/pkg/api"
)

// Target-override flags. Declared as persistent flags on the root command so
// every linked-component command accepts them uniformly. Each accepts an ID
// *or* a name — IDs are unmemorable, so name matching is the point.
var (
	orgFlag       string
	appFlag       string
	componentFlag string
)

func init() {
	rootCmd.PersistentFlags().StringVar(&orgFlag, "org", "",
		"Organization ID or name (overrides the link / active org)")
	rootCmd.PersistentFlags().StringVar(&appFlag, "app", "",
		"App ID or name (overrides the linked app)")
	rootCmd.PersistentFlags().StringVar(&componentFlag, "component", "",
		"Component ID or name (overrides the linked component)")
}

// resolveTarget is the link-optional successor to requireLinked. It produces
// the same *linkedCtx every linked-component command consumes, but the link
// file is now just one of three default sources. Per field the precedence is:
//
//	explicit flag (--org/--app/--component/--env, id or name)
//	  → link file entry for the active profile
//	    → interactive picker (auto-selecting singletons)
//
// A command run in a linked directory with no flags behaves exactly as
// before: the link supplies everything and nothing is prompted. Run anywhere
// else, the user picks interactively; with flags, it's fully non-interactive
// and works from any directory.
func resolveTarget(cmd *cobra.Command) (*linkedCtx, error) {
	cfg, prof, err := config.RequireAuth(serverFlag, profileFlag)
	if err != nil {
		return nil, err
	}

	// The link is best-effort: a directory may not be linked, or not linked
	// for this profile. Either way we degrade to flags + prompts.
	l := loadLinkForProfile()

	client := apiclient.New(prof.Server, prof.Token)
	ctx := cmd.Context()

	orgID, err := resolveOrg(ctx, cmd, client, cfg, prof, l)
	if err != nil {
		return nil, err
	}

	app, err := resolveAppTarget(ctx, cmd, client, orgID, l)
	if err != nil {
		return nil, err
	}

	compID, compName, err := resolveComponent(ctx, cmd, client, orgID, app, l)
	if err != nil {
		return nil, err
	}

	env, err := resolveEnv(cmd, app, l)
	if err != nil {
		return nil, err
	}

	return &linkedCtx{
		Config:  cfg,
		Profile: prof,
		Link: &link.Link{
			OrgID:         orgID,
			AppID:         app.ID,
			ComponentID:   compID,
			ComponentName: compName,
			Environment:   env,
		},
		Env:    env,
		Client: client,
	}, nil
}

// loadLinkForProfile returns the link entry for the active (or --profile)
// profile, or nil when there's no link file or no entry for that profile.
// The link is always best-effort here — a nil result just means "fall back
// to flags/prompts", never an error.
func loadLinkForProfile() *link.Link {
	if !link.FileExists() {
		return nil
	}
	cfg, err := config.Load()
	if err != nil {
		return nil
	}
	profName := profileFlag
	if profName == "" {
		profName = cfg.Active
	}
	if got, gerr := link.Get(profName); gerr == nil {
		return got
	}
	return nil
}

// resolveAppScope resolves just the org + app (no component, no env) for
// commands that operate at app scope — `conure app env …`. Same precedence
// as resolveTarget: explicit --org/--app → link → interactive picker. The
// returned app is the detail view, so callers have its Environments.
func resolveAppScope(cmd *cobra.Command) (orgID string, app *api.Application, client *apiclient.Client, err error) {
	cfg, prof, err := config.RequireAuth(serverFlag, profileFlag)
	if err != nil {
		return "", nil, nil, err
	}
	l := loadLinkForProfile()
	client = apiclient.New(prof.Server, prof.Token)
	ctx := cmd.Context()

	orgID, err = resolveOrg(ctx, cmd, client, cfg, prof, l)
	if err != nil {
		return "", nil, nil, err
	}
	app, err = resolveAppTarget(ctx, cmd, client, orgID, l)
	if err != nil {
		return "", nil, nil, err
	}
	return orgID, app, client, nil
}

// resolveOrgScope resolves just the org for commands that don't need an app
// at all (e.g. `conure var --scope org`). --org → link → active org →
// picker (persisting the interactive choice).
func resolveOrgScope(cmd *cobra.Command) (orgID string, client *apiclient.Client, err error) {
	cfg, prof, err := config.RequireAuth(serverFlag, profileFlag)
	if err != nil {
		return "", nil, err
	}
	l := loadLinkForProfile()
	client = apiclient.New(prof.Server, prof.Token)
	orgID, err = resolveOrg(cmd.Context(), cmd, client, cfg, prof, l)
	if err != nil {
		return "", nil, err
	}
	return orgID, client, nil
}

// resolveOrg: --org (id or name) → link → profile active org → picker. When
// the org is chosen interactively we persist it to the profile so subsequent
// commands inherit it, matching `conure init`'s sticky behavior.
func resolveOrg(ctx context.Context, _ *cobra.Command, client *apiclient.Client, cfg *config.Config, prof *config.Profile, l *link.Link) (string, error) {
	if orgFlag != "" {
		orgs, err := client.ListOrganizations(ctx)
		if err != nil {
			return "", fmt.Errorf("listing organizations: %w", err)
		}
		for _, o := range orgs {
			if o.ID == orgFlag || strings.EqualFold(o.Name, orgFlag) {
				return o.ID, nil
			}
		}
		return "", fmt.Errorf("no organization matches %q", orgFlag)
	}
	if l != nil && l.OrgID != "" {
		return l.OrgID, nil
	}
	if prof.ActiveOrg != "" {
		return prof.ActiveOrg, nil
	}
	orgID, err := selectOrg(ctx, client)
	if err != nil {
		return "", err
	}
	// Persist the interactive choice so the next command doesn't re-ask.
	prof.ActiveOrg = orgID
	if err := config.Save(cfg); err != nil {
		return "", fmt.Errorf("saving active org: %w", err)
	}
	ui.Info("Set active org (%s) — change with `conure org use <name>`\n", orgID)
	return orgID, nil
}

// resolveAppTarget: --app (id or name in org) → link → picker. Always returns
// the detail view so Environments are populated for env resolution.
func resolveAppTarget(ctx context.Context, _ *cobra.Command, client *apiclient.Client, orgID string, l *link.Link) (*api.Application, error) {
	if appFlag != "" {
		apps, err := client.ListApps(ctx, orgID)
		if err != nil {
			return nil, fmt.Errorf("listing apps: %w", err)
		}
		for _, a := range apps {
			if a.ID == appFlag || strings.EqualFold(a.Name, appFlag) {
				full, err := client.GetApp(ctx, orgID, a.ID)
				if err != nil {
					return nil, fmt.Errorf("loading app %s: %w", a.Name, err)
				}
				return full, nil
			}
		}
		return nil, fmt.Errorf("no app matches %q in org %s", appFlag, orgID)
	}
	if l != nil && l.AppID != "" {
		full, err := client.GetApp(ctx, orgID, l.AppID)
		if err != nil {
			return nil, fmt.Errorf("loading linked app: %w", err)
		}
		return full, nil
	}
	return selectApp(ctx, client, orgID)
}

// resolveComponent: --component (id or name in app) → link → picker.
func resolveComponent(ctx context.Context, _ *cobra.Command, client *apiclient.Client, orgID string, app *api.Application, l *link.Link) (id, name string, err error) {
	if componentFlag != "" {
		comps, err := client.ListAppComponents(ctx, orgID, app.ID)
		if err != nil {
			return "", "", fmt.Errorf("listing components: %w", err)
		}
		for _, c := range comps {
			if c.ID == componentFlag || strings.EqualFold(c.Name, componentFlag) {
				return c.ID, c.Name, nil
			}
		}
		return "", "", fmt.Errorf("no component matches %q in app %s", componentFlag, app.Name)
	}
	// Only trust the linked component when it belongs to the resolved app —
	// a stale link plus an explicit --app would otherwise act on the wrong
	// component.
	if l != nil && l.ComponentID != "" && (appFlag == "" || l.AppID == app.ID) {
		return l.ComponentID, l.ComponentName, nil
	}
	return selectComponent(ctx, client, orgID, app.ID)
}

// resolveEnv: --env → link → picker. The --env flag is registered per-command
// via addEnvFlag, so look it up defensively.
func resolveEnv(cmd *cobra.Command, app *api.Application, l *link.Link) (string, error) {
	if cmd != nil && cmd.Flags().Lookup("env") != nil {
		if v, _ := cmd.Flags().GetString("env"); v != "" {
			if !envExists(app, v) {
				return "", fmt.Errorf("no environment %q in app %s", v, app.Name)
			}
			return v, nil
		}
	}
	if l != nil && l.Environment != "" && (appFlag == "" || l.AppID == app.ID) {
		return l.Environment, nil
	}
	return selectEnv(app)
}
