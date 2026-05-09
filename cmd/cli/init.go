package main

import (
	"fmt"

	"github.com/charmbracelet/huh"
	"github.com/coffeenights/conure/pkg/api"
	"github.com/spf13/cobra"
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

func runInit(cmd *cobra.Command, args []string) error {
	if linkExists() {
		path, _, _ := linkPath()
		errC.Printf("✗ Already linked: %s\n", path)
		fmt.Println("  Delete the file to re-link this directory.")
		return fmt.Errorf("already linked")
	}

	cfg, err := requireAuth()
	if err != nil {
		return err
	}
	client := newClient(cfg)

	// Step 0: pick the active org if none is set yet.
	orgID := cfg.ActiveOrg
	if orgID == "" {
		orgs, err := listOrgs(client)
		if err != nil {
			return fmt.Errorf("listing organizations: %w", err)
		}
		if len(orgs) == 0 {
			return fmt.Errorf("no organizations available — create one in the UI first")
		}
		options := make([]huh.Option[string], len(orgs))
		for i, o := range orgs {
			options[i] = huh.NewOption(o.Name, o.ID)
		}
		err = huh.NewSelect[string]().
			Title("Organization").
			Options(options...).
			Value(&orgID).
			Run()
		if err != nil {
			return err
		}
		cfg.ActiveOrg = orgID
		if err := saveConfig(cfg); err != nil {
			return fmt.Errorf("saving config: %w", err)
		}
	}

	// Step 1: app picker — create new vs use existing.
	apps, err := listApps(client, orgID)
	if err != nil {
		return fmt.Errorf("listing apps: %w", err)
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
		return err
	}

	var (
		app          *api.Application
		envName      = defaultEnvName
		componentID  string
		compName     string
		creatingComp = true
	)

	if appChoice == newAppSentinel {
		// Step 1a: create new app.
		appName := cwdBasename()
		if err := huh.NewInput().
			Title("App name").
			Value(&appName).
			Run(); err != nil {
			return err
		}
		newApp, err := createApp(client, orgID, appName, "")
		if err != nil {
			return fmt.Errorf("creating app: %w", err)
		}
		app = newApp
		success.Printf("✓ Created app `%s`\n", app.Name)

		// Auto-create the default env on a brand-new app.
		if err := createEnv(client, orgID, app.ID, defaultEnvName); err != nil {
			return fmt.Errorf("creating environment: %w", err)
		}
		success.Printf("✓ Created environment `%s`\n", defaultEnvName)
	} else {
		// Step 1b: existing app.
		full, err := getApp(client, orgID, appChoice)
		if err != nil {
			return fmt.Errorf("loading app: %w", err)
		}
		app = full

		// Step 1b': pick env (or create).
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
			return err
		}
		if envChoice == newEnvSentinel {
			newEnv := defaultEnvName
			if err := huh.NewInput().
				Title("Environment name").
				Value(&newEnv).
				Run(); err != nil {
				return err
			}
			if err := createEnv(client, orgID, app.ID, newEnv); err != nil {
				return fmt.Errorf("creating environment: %w", err)
			}
			envName = newEnv
		} else {
			envName = envChoice
		}

		// Step 2: component picker — create new vs pick existing.
		comps, err := listAppComponents(client, orgID, app.ID)
		if err != nil {
			return fmt.Errorf("listing components: %w", err)
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
				return err
			}
			if compChoice != newCompSentinel {
				creatingComp = false
				componentID = compChoice
				for _, c := range comps {
					if c.ID == compChoice {
						compName = c.Name
						break
					}
				}
			}
		}
	}

	// Step 3: create component (if not picking an existing one).
	if creatingComp {
		name := detectComponentName()
		if err := huh.NewInput().
			Title("Component name").
			Value(&name).
			Run(); err != nil {
			return err
		}

		defs, err := listComponentDefinitions(client, orgID)
		if err != nil {
			return fmt.Errorf("loading component definitions: %w", err)
		}
		if len(defs) == 0 {
			return fmt.Errorf("no component definitions registered for this org")
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
			return err
		}

		created, err := createComponent(client, orgID, app.ID, name, compType, envName)
		if err != nil {
			return fmt.Errorf("creating component: %w", err)
		}
		componentID = created.Component.ID
		compName = created.Component.Name
		success.Printf("✓ Created component `%s` (%s) in env `%s`\n", compName, compType, envName)
	}

	link := &Link{
		OrgID:         orgID,
		AppID:         app.ID,
		ComponentID:   componentID,
		ComponentName: compName,
		Environment:   envName,
	}
	if err := saveLink(link); err != nil {
		return fmt.Errorf("writing link: %w", err)
	}
	path, _, _ := linkPath()
	success.Printf("✓ Linked %s\n", path)
	fmt.Println()
	info.Println("Next: run `conure deploy` to deploy.")
	return nil
}
