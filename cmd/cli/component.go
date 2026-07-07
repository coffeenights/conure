package main

import (
	"fmt"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"github.com/coffeenights/conure/internal/cli/ui"
)

var componentCmd = &cobra.Command{
	Use:     "component",
	Aliases: []string{"comp"},
	Short:   "Manage components",
}

var componentListCmd = &cobra.Command{
	Use:   "list",
	Short: "List components in the linked application",
	Long: `List components in the linked application, with a per-environment
presence summary in the ENVS column.

Env flag:
  *  draft revision present (not yet deployed)

Example: "staging*, production" means staging has an undeployed draft,
production is clean.`,
	RunE: runComponentList,
}

var componentAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a new component to the linked application",
	Long: `Interactive prompt to add another component to the app this directory is
linked to. Does not change the link — the linked component stays the same.`,
	RunE: runComponentAdd,
}

var componentRestartCmd = &cobra.Command{
	Use:   "restart",
	Short: "Trigger a rolling restart of the linked component in this env",
	Long: `Stamps a fresh conure.io/restartedAt annotation on the Component CRD,
re-applies the latest deployed revision, and records a new deployed
revision in history (auto-commented "Restart at <ts>"). Component types
without a pod template (e.g. pure config) are a silent no-op.`,
	RunE: runComponentRestart,
}

func init() {
	addEnvFlag(componentRestartCmd)
	componentRestartCmd.Flags().Bool("approve", false, "Skip the confirmation prompt")
	componentCmd.AddCommand(componentListCmd)
	componentCmd.AddCommand(componentAddCmd)
	componentCmd.AddCommand(componentRestartCmd)
	rootCmd.AddCommand(componentCmd)
}

func runComponentList(cmd *cobra.Command, _ []string) error {
	orgID, app, client, err := resolveAppScope(cmd)
	if err != nil {
		return err
	}
	comps, err := client.ListAppComponents(cmd.Context(), orgID, app.ID)
	if err != nil {
		return err
	}
	return ui.Render(comps, func() error {
		if len(comps) == 0 {
			ui.InfoLn("No components found")
			return nil
		}
		rows := make([][]string, len(comps))
		for i, c := range comps {
			envSummary := "-"
			if len(c.Environments) > 0 {
				envSummary = ""
				for j, e := range c.Environments {
					if j > 0 {
						envSummary += ", "
					}
					flags := ""
					if e.HasDraft {
						flags += "*"
					}
					envSummary += fmt.Sprintf("%s%s", e.EnvironmentName, flags)
				}
			}
			rows[i] = []string{c.ID, c.Name, c.Type, envSummary}
		}
		ui.RenderTable([]string{"ID", "NAME", "TYPE", "ENVS"}, rows, nil)
		return nil
	})
}

func runComponentAdd(cmd *cobra.Command, _ []string) error {
	orgID, app, client, err := resolveAppScope(cmd)
	if err != nil {
		return err
	}
	ctx := cmd.Context()

	// Default the env to the linked one when this directory's link points at
	// the same app; otherwise the picker (or the single env) decides.
	envName := ""
	if l := loadLinkForProfile(); l != nil && l.AppID == app.ID {
		envName = l.Environment
	}
	if len(app.Environments) == 1 {
		envName = app.Environments[0].Name
	} else if len(app.Environments) > 1 {
		envOpts := make([]huh.Option[string], len(app.Environments))
		for i, e := range app.Environments {
			envOpts[i] = huh.NewOption(e.Name, e.Name)
		}
		if err := huh.NewSelect[string]().
			Title("Environment for first draft").
			Options(envOpts...).
			Value(&envName).
			Run(); err != nil {
			return err
		}
	}

	if _, _, err := createComponentFlow(ctx, client, orgID, app.ID, envName); err != nil {
		return err
	}
	ui.InfoLn("  This directory's link is unchanged. Use the UI or another repo to manage this component.")
	return nil
}

func runComponentRestart(cmd *cobra.Command, _ []string) error {
	lc, err := resolveTarget(cmd)
	if err != nil {
		return err
	}
	approve, _ := cmd.Flags().GetBool("approve")
	if !approve {
		ui.Error("This will roll pods of `%s` in `%s` — in-flight requests on existing pods may be interrupted.\n", lc.Link.ComponentName, lc.Env)
		var ok bool
		if err := huh.NewConfirm().
			Title(fmt.Sprintf("Restart %s in %s?", lc.Link.ComponentName, lc.Env)).
			Affirmative("Restart").
			Negative("Cancel").
			Value(&ok).
			Run(); err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("aborted")
		}
	}
	sp := ui.StartSpinner(fmt.Sprintf("Restarting `%s` in `%s`…", lc.Link.ComponentName, lc.Env))
	rev, err := lc.Client.RestartComponent(cmd.Context(), lc.Link.OrgID, lc.Link.AppID, lc.Env, lc.Link.ComponentID)
	ui.StopSpinner(sp)
	if err != nil {
		return err
	}
	ui.Success("✓ Restart recorded as v%d (%s)\n", rev.Version, rev.ID)
	if rev.Comment != "" {
		ui.InfoLn("  " + rev.Comment)
	}
	return nil
}
