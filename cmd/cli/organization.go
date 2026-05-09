package main

import (
	"encoding/json"
	"fmt"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/spf13/cobra"
)

var orgCmd = &cobra.Command{
	Use:     "organization",
	Aliases: []string{"org"},
	Short:   "Manage organizations",
}

var orgListCmd = &cobra.Command{
	Use:   "list",
	Short: "List organizations",
	RunE:  runOrgList,
}

var switchCmd = &cobra.Command{
	Use:   "switch",
	Short: "Switch the active context",
}

var switchOrgCmd = &cobra.Command{
	Use:   "org <name-or-id>",
	Short: "Set the active organization in ~/.conure/config.json",
	Args:  cobra.ExactArgs(1),
	RunE:  runSwitchOrg,
}

func init() {
	orgCmd.AddCommand(orgListCmd)
	switchCmd.AddCommand(switchOrgCmd)
	rootCmd.AddCommand(orgCmd)
	rootCmd.AddCommand(switchCmd)
}

func runOrgList(cmd *cobra.Command, args []string) error {
	cfg, err := requireAuth()
	if err != nil {
		return err
	}
	client := newClient(cfg)
	orgs, err := listOrgs(client)
	if err != nil {
		return err
	}
	if outputFlag == "json" {
		out, _ := json.MarshalIndent(orgs, "", "  ")
		fmt.Println(string(out))
		return nil
	}
	if len(orgs) == 0 {
		info.Println("No organizations found")
		return nil
	}

	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212")).Padding(0, 1)
	cellStyle := lipgloss.NewStyle().Padding(0, 1)
	activeStyle := cellStyle.Foreground(lipgloss.Color("42"))
	t := table.New().
		Border(lipgloss.RoundedBorder()).
		BorderStyle(lipgloss.NewStyle().Foreground(lipgloss.Color("240"))).
		Headers("ACTIVE", "ID", "NAME").
		StyleFunc(func(row, col int) lipgloss.Style {
			if row == table.HeaderRow {
				return headerStyle
			}
			if col == 0 {
				return activeStyle
			}
			return cellStyle
		})
	for _, o := range orgs {
		marker := ""
		if o.ID == cfg.ActiveOrg {
			marker = "*"
		}
		t.Row(marker, o.ID, o.Name)
	}
	fmt.Println(t)
	return nil
}

func runSwitchOrg(cmd *cobra.Command, args []string) error {
	cfg, err := requireAuth()
	if err != nil {
		return err
	}
	client := newClient(cfg)
	orgs, err := listOrgs(client)
	if err != nil {
		return err
	}
	target := args[0]
	var matchID, matchName string
	for _, o := range orgs {
		if o.ID == target || o.Name == target {
			matchID = o.ID
			matchName = o.Name
			break
		}
	}
	if matchID == "" {
		errC.Printf("✗ No organization matches `%s`\n", target)
		fmt.Println("Available:")
		for _, o := range orgs {
			fmt.Printf("  %s  (%s)\n", o.Name, o.ID)
		}
		return fmt.Errorf("no match")
	}
	cfg.ActiveOrg = matchID
	if err := saveConfig(cfg); err != nil {
		return err
	}
	success.Printf("✓ Active org: %s (%s)\n", matchName, matchID)
	return nil
}
