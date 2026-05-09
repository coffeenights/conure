package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/spf13/cobra"
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

func runAppList(cmd *cobra.Command, args []string) error {
	cfg, err := requireActiveOrg()
	if err != nil {
		return err
	}
	client := newClient(cfg)
	apps, err := listApps(client, cfg.ActiveOrg)
	if err != nil {
		return err
	}
	if outputFlag == "json" {
		out, _ := json.MarshalIndent(apps, "", "  ")
		fmt.Println(string(out))
		return nil
	}
	if len(apps) == 0 {
		info.Println("No applications found")
		return nil
	}

	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212")).Padding(0, 1)
	cellStyle := lipgloss.NewStyle().Padding(0, 1)
	t := table.New().
		Border(lipgloss.RoundedBorder()).
		BorderStyle(lipgloss.NewStyle().Foreground(lipgloss.Color("240"))).
		Headers("ID", "NAME", "ENVIRONMENTS", "COMPONENTS").
		StyleFunc(func(row, col int) lipgloss.Style {
			if row == table.HeaderRow {
				return headerStyle
			}
			return cellStyle
		})
	for _, a := range apps {
		envStr := "-"
		if len(a.Environments) > 0 {
			names := make([]string, len(a.Environments))
			for i, e := range a.Environments {
				names[i] = e.Name
			}
			envStr = strings.Join(names, ", ")
		}
		t.Row(a.ID, a.Name, envStr, fmt.Sprintf("%d", a.TotalComponents))
	}
	fmt.Println(t)
	return nil
}
