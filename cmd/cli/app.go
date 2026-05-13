package main

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/coffeenights/conure/internal/cli/ui"
	"github.com/coffeenights/conure/pkg/api"
)

var appCmd = &cobra.Command{
	Use:   "app",
	Short: "Manage applications",
}

var appListCmd = &cobra.Command{
	Use:   "list",
	Short: "List applications in the active organization",
	RunE:  runAppList,
}

func init() {
	appCmd.AddCommand(appListCmd)
	rootCmd.AddCommand(appCmd)
}

func runAppList(cmd *cobra.Command, _ []string) error {
	cfg, client, err := requireActiveOrgClient()
	if err != nil {
		return err
	}
	apps, err := client.ListApps(cmd.Context(), cfg.ActiveOrg)
	if err != nil {
		return err
	}
	return ui.Render(apps, func() error {
		if len(apps) == 0 {
			ui.InfoLn("No applications found")
			return nil
		}
		rows := make([][]string, len(apps))
		for i, a := range apps {
			rows[i] = []string{a.ID, a.Name, summarizeEnvs(a.Environments), fmt.Sprintf("%d", a.TotalComponents)}
		}
		ui.RenderTable([]string{"ID", "NAME", "ENVIRONMENTS", "COMPONENTS"}, rows, nil)
		return nil
	})
}

func summarizeEnvs(envs []api.Environment) string {
	if len(envs) == 0 {
		return "-"
	}
	names := make([]string, len(envs))
	for i, e := range envs {
		names[i] = e.Name
	}
	return strings.Join(names, ", ")
}
