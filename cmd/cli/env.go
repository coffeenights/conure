package main

import (
	"context"
	"fmt"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"github.com/coffeenights/conure/internal/cli/apiclient"
	"github.com/coffeenights/conure/internal/cli/link"
	"github.com/coffeenights/conure/internal/cli/ui"
	"github.com/coffeenights/conure/pkg/api"
)

var envCmd = &cobra.Command{
	Use:     "env",
	Aliases: []string{"environment", "environments"},
	Short:   "Manage application environments",
	Long: `Create, list, and delete environments on a Conure application.

By default these commands act on the app this directory is linked to. Pass
--app <id-or-name> to target a different app in the active organization.`,
}

var envListCmd = &cobra.Command{
	Use:   "list",
	Short: "List environments on the target app",
	RunE:  runEnvList,
}

var envCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a new environment",
	Args:  cobra.ExactArgs(1),
	RunE:  runEnvCreate,
}

var envDeleteCmd = &cobra.Command{
	Use:     "delete <name>",
	Aliases: []string{"rm"},
	Short:   "Delete an environment (and everything deployed to it)",
	Args:    cobra.ExactArgs(1),
	RunE:    runEnvDelete,
}

func init() {
	for _, c := range []*cobra.Command{envListCmd, envCreateCmd, envDeleteCmd} {
		c.Flags().String("app", "",
			"App ID or name (defaults to the linked app)")
	}
	envDeleteCmd.Flags().Bool("yes", false,
		"Skip the confirmation prompt")

	envCmd.AddCommand(envListCmd)
	envCmd.AddCommand(envCreateCmd)
	envCmd.AddCommand(envDeleteCmd)
	appCmd.AddCommand(envCmd)
}

// appCtx resolves the org + app a `conure app env …` command should act
// on. The lookup honors --app when given (matching by id or name in the
// active org) and falls back to the link otherwise. Returning the client
// alongside means callers don't have to re-derive auth.
type appCtx struct {
	Client *apiclient.Client
	OrgID  string
	App    *api.Application
}

func resolveApp(cmd *cobra.Command) (*appCtx, error) {
	appFlag, _ := cmd.Flags().GetString("app")

	// No --app: fall back to the link. We still allow running from a
	// non-linked directory, in which case --app is required.
	if appFlag == "" {
		if !link.FileExists() {
			return nil, fmt.Errorf("no .conure/link.json found and no --app provided — run 'conure init' or pass --app")
		}
		lc, err := requireLinked(cmd)
		if err != nil {
			return nil, err
		}
		app, err := lc.Client.GetApp(cmd.Context(), lc.Link.OrgID, lc.Link.AppID)
		if err != nil {
			return nil, fmt.Errorf("loading linked app: %w", err)
		}
		return &appCtx{Client: lc.Client, OrgID: lc.Link.OrgID, App: app}, nil
	}

	// --app given: need an active org (from link or profile) plus a
	// listing to match the value against id-or-name.
	orgID, client, err := resolveOrgClient(cmd)
	if err != nil {
		return nil, err
	}
	apps, err := client.ListApps(cmd.Context(), orgID)
	if err != nil {
		return nil, fmt.Errorf("listing apps: %w", err)
	}
	var match *api.Application
	for i, a := range apps {
		if a.ID == appFlag || a.Name == appFlag {
			match = &apps[i]
			break
		}
	}
	if match == nil {
		return nil, fmt.Errorf("no app matches %q in org %s", appFlag, orgID)
	}
	// ListApps doesn't populate Environments — refetch the detail view so
	// `env list` has them without a second round-trip in the caller.
	full, err := client.GetApp(cmd.Context(), orgID, match.ID)
	if err != nil {
		return nil, fmt.Errorf("loading app %s: %w", match.Name, err)
	}
	return &appCtx{Client: client, OrgID: orgID, App: full}, nil
}

// resolveOrgClient picks the org for a command that doesn't rely on the
// link's app. Preference order: linked org → active org. Mirrors the same
// fallbacks used by `conure var --scope org`.
func resolveOrgClient(cmd *cobra.Command) (string, *apiclient.Client, error) {
	if link.FileExists() {
		lc, err := requireLinked(cmd)
		if err != nil {
			return "", nil, err
		}
		return lc.Link.OrgID, lc.Client, nil
	}
	_, prof, client, err := requireActiveOrgClient()
	if err != nil {
		return "", nil, err
	}
	return prof.ActiveOrg, client, nil
}

func runEnvList(cmd *cobra.Command, _ []string) error {
	ac, err := resolveApp(cmd)
	if err != nil {
		return err
	}
	envs := ac.App.Environments
	return ui.Render(envs, func() error {
		if len(envs) == 0 {
			ui.InfoLn("No environments yet — run `conure app env create <name>`")
			return nil
		}
		rows := make([][]string, len(envs))
		for i, e := range envs {
			rows[i] = []string{e.Name, e.ID}
		}
		ui.RenderTable([]string{"NAME", "ID"}, rows, nil)
		return nil
	})
}

func runEnvCreate(cmd *cobra.Command, args []string) error {
	ac, err := resolveApp(cmd)
	if err != nil {
		return err
	}
	name := args[0]
	// Catch the duplicate up front so we get a clean error instead of a
	// server-side 4xx. The server enforces uniqueness too; this is just
	// for the message quality.
	for _, e := range ac.App.Environments {
		if e.Name == name {
			return fmt.Errorf("environment %q already exists in app %s", name, ac.App.Name)
		}
	}
	if err := ac.Client.CreateEnvironment(cmd.Context(), ac.OrgID, ac.App.ID, name); err != nil {
		return err
	}
	ui.Success("✓ Created environment `%s` in app `%s`\n", name, ac.App.Name)
	return nil
}

func runEnvDelete(cmd *cobra.Command, args []string) error {
	ac, err := resolveApp(cmd)
	if err != nil {
		return err
	}
	name := args[0]
	if !envExists(ac.App, name) {
		return fmt.Errorf("no environment %q in app %s", name, ac.App.Name)
	}
	skipConfirm, _ := cmd.Flags().GetBool("yes")
	if !skipConfirm {
		if err := confirmDelete(cmd.Context(), name, ac.App.Name); err != nil {
			return err
		}
	}
	if err := ac.Client.DeleteEnvironment(cmd.Context(), ac.OrgID, ac.App.ID, name); err != nil {
		return err
	}
	ui.Success("✓ Deleted environment `%s` from app `%s`\n", name, ac.App.Name)
	return nil
}

func envExists(app *api.Application, name string) bool {
	for _, e := range app.Environments {
		if e.Name == name {
			return true
		}
	}
	return false
}

// confirmDelete refuses to proceed unless the user types the env name back.
// We require the name (not just y/n) because env deletion cascades to all
// deployed revisions; a stray keystroke shouldn't be enough.
func confirmDelete(_ context.Context, envName, appName string) error {
	ui.Error("This will delete environment `%s` from app `%s` and all revisions in it.\n", envName, appName)
	var typed string
	if err := huh.NewInput().
		Title(fmt.Sprintf("Type %q to confirm", envName)).
		Value(&typed).
		Run(); err != nil {
		return err
	}
	if typed != envName {
		return fmt.Errorf("aborted: confirmation did not match")
	}
	return nil
}
