package main

import (
	"context"
	"fmt"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"github.com/coffeenights/conure/internal/cli/apiclient"
	"github.com/coffeenights/conure/internal/cli/config"
	"github.com/coffeenights/conure/internal/cli/link"
	"github.com/coffeenights/conure/internal/cli/ui"
	"github.com/coffeenights/conure/pkg/api"
)

const defaultEnvName = "production"

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Set up this repo to deploy into Conure",
	Long: `Interactive wizard that links this directory to a Conure component.

Creates (or attaches to) an app and a component, then writes
.conure/link.json. Does not deploy — run 'conure deploy' next.`,
	RunE: runInit,
}

func init() {
	rootCmd.AddCommand(initCmd)
}

func runInit(cmd *cobra.Command, _ []string) error {
	if link.Exists() {
		path, _, _ := link.Path()
		ui.Error("✗ Already linked: %s\n", path)
		fmt.Println("  Delete the file to re-link this directory.")
		return fmt.Errorf("already linked")
	}

	cfg, err := config.RequireAuth(serverFlag)
	if err != nil {
		return err
	}
	client := apiclient.New(cfg.Server, cfg.Token)
	ctx := cmd.Context()

	orgID, err := pickOrCreateOrgContext(ctx, client, cfg)
	if err != nil {
		return err
	}

	app, envName, existing, err := pickOrCreateApp(ctx, client, orgID)
	if err != nil {
		return err
	}

	componentID, compName, err := pickOrCreateComponent(ctx, client, orgID, app, envName, existing)
	if err != nil {
		return err
	}

	l := &link.Link{
		OrgID:         orgID,
		AppID:         app.ID,
		ComponentID:   componentID,
		ComponentName: compName,
		Environment:   envName,
	}
	if err := link.Save(l); err != nil {
		return fmt.Errorf("writing link: %w", err)
	}
	path, _, _ := link.Path()
	ui.Success("✓ Linked %s\n", path)
	fmt.Println()
	ui.InfoLn("Next: run `conure deploy` to deploy.")
	return nil
}

// pickOrCreateOrgContext returns the active org. If config already has one,
// we trust it; otherwise we prompt and persist the choice so subsequent
// commands inherit it.
func pickOrCreateOrgContext(ctx context.Context, client *apiclient.Client, cfg *config.Config) (string, error) {
	if cfg.ActiveOrg != "" {
		return cfg.ActiveOrg, nil
	}
	orgs, err := client.ListOrganizations(ctx)
	if err != nil {
		return "", fmt.Errorf("listing organizations: %w", err)
	}
	if len(orgs) == 0 {
		return "", fmt.Errorf("no organizations available — create one in the UI first")
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
	cfg.ActiveOrg = orgID
	if err := config.Save(cfg); err != nil {
		return "", fmt.Errorf("saving config: %w", err)
	}
	return orgID, nil
}

// pickOrCreateApp prompts the user to pick an existing app or create a new
// one, and resolves the env name for the new link. The returned bool is
// true when the user picked an existing app (which then unlocks reusing an
// existing component in the next step).
func pickOrCreateApp(ctx context.Context, client *apiclient.Client, orgID string) (*api.Application, string, bool, error) {
	apps, err := client.ListApps(ctx, orgID)
	if err != nil {
		return nil, "", false, fmt.Errorf("listing apps: %w", err)
	}

	const newAppSentinel = "__new__"
	appChoice := newAppSentinel
	options := []huh.Option[string]{huh.NewOption("+ Create new app", newAppSentinel)}
	for _, a := range apps {
		options = append(options, huh.NewOption(a.Name, a.ID))
	}
	if err := huh.NewSelect[string]().
		Title("App").
		Description("Pick an existing app or create a new one.").
		Options(options...).
		Value(&appChoice).
		Run(); err != nil {
		return nil, "", false, err
	}

	if appChoice == newAppSentinel {
		app, err := createAppFlow(ctx, client, orgID)
		if err != nil {
			return nil, "", false, err
		}
		return app, defaultEnvName, false, nil
	}

	app, err := client.GetApp(ctx, orgID, appChoice)
	if err != nil {
		return nil, "", false, fmt.Errorf("loading app: %w", err)
	}
	envName, err := pickOrCreateEnv(ctx, client, orgID, app)
	if err != nil {
		return nil, "", false, err
	}
	return app, envName, true, nil
}

// createAppFlow prompts for an app name, creates it, and seeds the default
// environment. Splits out of pickOrCreateApp so the prompt-side path is
// self-contained.
func createAppFlow(ctx context.Context, client *apiclient.Client, orgID string) (*api.Application, error) {
	appName := cwdBasename()
	if err := huh.NewInput().
		Title("App name").
		Value(&appName).
		Run(); err != nil {
		return nil, err
	}
	app, err := client.CreateApp(ctx, orgID, appName, "")
	if err != nil {
		return nil, fmt.Errorf("creating app: %w", err)
	}
	ui.Success("✓ Created app `%s`\n", app.Name)

	if err := client.CreateEnvironment(ctx, orgID, app.ID, defaultEnvName); err != nil {
		return nil, fmt.Errorf("creating environment: %w", err)
	}
	ui.Success("✓ Created environment `%s`\n", defaultEnvName)
	return app, nil
}

// pickOrCreateEnv lets the user pick one of the app's existing environments
// or create a new one. Only called for existing apps; brand-new apps go
// straight to defaultEnvName.
func pickOrCreateEnv(ctx context.Context, client *apiclient.Client, orgID string, app *api.Application) (string, error) {
	const newEnvSentinel = "__newenv__"
	envChoice := defaultEnvName
	envOpts := []huh.Option[string]{huh.NewOption("+ Create new environment", newEnvSentinel)}
	for _, e := range app.Environments {
		envOpts = append(envOpts, huh.NewOption(e.Name, e.Name))
		envChoice = e.Name
	}
	if err := huh.NewSelect[string]().
		Title("Environment").
		Options(envOpts...).
		Value(&envChoice).
		Run(); err != nil {
		return "", err
	}
	if envChoice != newEnvSentinel {
		return envChoice, nil
	}

	newEnv := defaultEnvName
	if err := huh.NewInput().
		Title("Environment name").
		Value(&newEnv).
		Run(); err != nil {
		return "", err
	}
	if err := client.CreateEnvironment(ctx, orgID, app.ID, newEnv); err != nil {
		return "", fmt.Errorf("creating environment: %w", err)
	}
	return newEnv, nil
}

// pickOrCreateComponent either reuses one of the app's existing components
// (only offered when the user is attaching to an existing app) or runs the
// create-component flow. Returns the resolved component id + display name
// so the caller can write the link file.
func pickOrCreateComponent(ctx context.Context, client *apiclient.Client, orgID string, app *api.Application, envName string, existingApp bool) (string, string, error) {
	if existingApp {
		comps, err := client.ListAppComponents(ctx, orgID, app.ID)
		if err != nil {
			return "", "", fmt.Errorf("listing components: %w", err)
		}
		if len(comps) > 0 {
			const newCompSentinel = "__newcomp__"
			compChoice := newCompSentinel
			compOpts := []huh.Option[string]{huh.NewOption("+ Create new component", newCompSentinel)}
			for _, c := range comps {
				compOpts = append(compOpts, huh.NewOption(c.Name, c.ID))
			}
			if err := huh.NewSelect[string]().
				Title("Component").
				Options(compOpts...).
				Value(&compChoice).
				Run(); err != nil {
				return "", "", err
			}
			if compChoice != newCompSentinel {
				for _, c := range comps {
					if c.ID == compChoice {
						return c.ID, c.Name, nil
					}
				}
			}
		}
	}
	return createComponentFlow(ctx, client, orgID, app.ID, envName)
}

// createComponentFlow walks the user through picking a name + type and then
// creates the component server-side. Shared by the init wizard and the
// `component add` command.
func createComponentFlow(ctx context.Context, client *apiclient.Client, orgID, appID, envName string) (string, string, error) {
	name := detectComponentName()
	if err := huh.NewInput().
		Title("Component name").
		Value(&name).
		Run(); err != nil {
		return "", "", err
	}

	defs, err := client.ListComponentDefinitions(ctx, orgID)
	if err != nil {
		return "", "", fmt.Errorf("loading component definitions: %w", err)
	}
	if len(defs) == 0 {
		return "", "", fmt.Errorf("no component definitions registered for this org")
	}
	compType := defs[0].Type
	typeOpts := make([]huh.Option[string], len(defs))
	for i, d := range defs {
		label := d.Name
		if label == "" {
			label = d.Type
		}
		typeOpts[i] = huh.NewOption(label, d.Type)
	}
	if err := huh.NewSelect[string]().
		Title("Component type").
		Options(typeOpts...).
		Value(&compType).
		Run(); err != nil {
		return "", "", err
	}

	created, err := client.CreateComponent(ctx, orgID, appID, name, compType, envName)
	if err != nil {
		return "", "", fmt.Errorf("creating component: %w", err)
	}
	ui.Success("✓ Created component `%s` (%s) in env `%s`\n", created.Component.Name, compType, envName)
	return created.Component.ID, created.Component.Name, nil
}
