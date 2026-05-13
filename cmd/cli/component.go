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
	RunE:  runComponentList,
}

var componentAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a new component to the linked application",
	Long: `Interactive prompt to add another component to the app this directory is
linked to. Does not change the link — the linked component stays the same.`,
	RunE: runComponentAdd,
}

func init() {
	componentCmd.AddCommand(componentListCmd)
	componentCmd.AddCommand(componentAddCmd)
	rootCmd.AddCommand(componentCmd)
}

func runComponentList(cmd *cobra.Command, _ []string) error {
	lc, err := requireLinked(cmd)
	if err != nil {
		return err
	}
	comps, err := lc.Client.ListAppComponents(cmd.Context(), lc.Link.OrgID, lc.Link.AppID)
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
					if e.Drifted {
						flags += "!"
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
	lc, err := requireLinked(cmd)
	if err != nil {
		return err
	}
	ctx := cmd.Context()

	app, err := lc.Client.GetApp(ctx, lc.Link.OrgID, lc.Link.AppID)
	if err != nil {
		return err
	}

	envName := lc.Link.Environment
	if len(app.Environments) > 1 {
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

	if _, _, err := createComponentFlow(ctx, lc.Client, lc.Link.OrgID, lc.Link.AppID, envName); err != nil {
		return err
	}
	ui.InfoLn("  This directory's link is unchanged. Use the UI or another repo to manage this component.")
	return nil
}
