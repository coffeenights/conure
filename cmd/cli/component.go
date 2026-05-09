package main

import (
	"encoding/json"
	"fmt"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/spf13/cobra"
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

func runComponentList(cmd *cobra.Command, args []string) error {
	cfg, err := requireAuth()
	if err != nil {
		return err
	}
	link, err := requireLink()
	if err != nil {
		return err
	}
	client := newClient(cfg)
	comps, err := listAppComponents(client, link.OrgID, link.AppID)
	if err != nil {
		return err
	}
	if outputFlag == "json" {
		out, _ := json.MarshalIndent(comps, "", "  ")
		fmt.Println(string(out))
		return nil
	}
	if len(comps) == 0 {
		info.Println("No components found")
		return nil
	}

	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212")).Padding(0, 1)
	cellStyle := lipgloss.NewStyle().Padding(0, 1)
	t := table.New().
		Border(lipgloss.RoundedBorder()).
		BorderStyle(lipgloss.NewStyle().Foreground(lipgloss.Color("240"))).
		Headers("ID", "NAME", "TYPE", "ENVS").
		StyleFunc(func(row, col int) lipgloss.Style {
			if row == table.HeaderRow {
				return headerStyle
			}
			return cellStyle
		})
	for _, c := range comps {
		envSummary := "-"
		if len(c.Environments) > 0 {
			parts := make([]string, len(c.Environments))
			for i, e := range c.Environments {
				flags := ""
				if e.HasDraft {
					flags += "*"
				}
				if e.Drifted {
					flags += "!"
				}
				parts[i] = fmt.Sprintf("%s%s", e.EnvironmentName, flags)
			}
			envSummary = ""
			for i, p := range parts {
				if i > 0 {
					envSummary += ", "
				}
				envSummary += p
			}
		}
		t.Row(c.ID, c.Name, c.Type, envSummary)
	}
	fmt.Println(t)
	return nil
}

func runComponentAdd(cmd *cobra.Command, args []string) error {
	cfg, err := requireAuth()
	if err != nil {
		return err
	}
	link, err := requireLink()
	if err != nil {
		return err
	}
	client := newClient(cfg)

	app, err := getApp(client, link.OrgID, link.AppID)
	if err != nil {
		return err
	}

	envName := link.Environment
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

	name := detectComponentName()
	if err := huh.NewInput().
		Title("Component name").
		Value(&name).
		Run(); err != nil {
		return err
	}

	defs, err := listComponentDefinitions(client, link.OrgID)
	if err != nil {
		return err
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

	created, err := createComponent(client, link.OrgID, link.AppID, name, compType, envName)
	if err != nil {
		return err
	}
	success.Printf("✓ Created component `%s` (%s) in env `%s`\n", created.Component.Name, compType, envName)
	info.Println("  This directory's link is unchanged. Use the UI or another repo to manage this component.")
	return nil
}
