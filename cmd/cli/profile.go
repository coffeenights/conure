package main

import (
	"fmt"
	"sort"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/coffeenights/conure/internal/cli/config"
	"github.com/coffeenights/conure/internal/cli/ui"
)

var profileCmd = &cobra.Command{
	Use:   "profile",
	Short: "Manage local CLI profiles (server + auth + active org)",
	Long: `A profile bundles a server URL, an auth token, and the active org
selected on that server. Use multiple profiles to keep credentials for
different Conure servers side by side.`,
}

var profileListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all profiles",
	RunE:  runProfileList,
}

var profileUseCmd = &cobra.Command{
	Use:   "use <name>",
	Short: "Switch the active profile",
	Args:  cobra.ExactArgs(1),
	RunE:  runProfileUse,
}

var profileCurrentCmd = &cobra.Command{
	Use:   "current",
	Short: "Print the active profile's name",
	RunE:  runProfileCurrent,
}

var profileRemoveCmd = &cobra.Command{
	Use:     "remove <name>",
	Aliases: []string{"rm"},
	Short:   "Delete a profile",
	Args:    cobra.ExactArgs(1),
	RunE:    runProfileRemove,
}

func init() {
	profileCmd.AddCommand(profileListCmd)
	profileCmd.AddCommand(profileUseCmd)
	profileCmd.AddCommand(profileCurrentCmd)
	profileCmd.AddCommand(profileRemoveCmd)
	rootCmd.AddCommand(profileCmd)
}

func runProfileList(_ *cobra.Command, _ []string) error {
	cfg, err := config.Load()
	if err != nil || len(cfg.Profiles) == 0 {
		ui.InfoLn("No profiles — run `conure login` to add one.")
		return nil
	}
	return ui.Render(cfg.Profiles, func() error {
		names := cfg.Names()
		sort.Strings(names)
		rows := make([][]string, len(names))
		for i, n := range names {
			p := cfg.Profiles[n]
			marker := ""
			if n == cfg.Active {
				marker = "*"
			}
			org := p.ActiveOrg
			if org == "" {
				org = "-"
			}
			rows[i] = []string{marker, n, p.Server, org}
		}
		active := lipgloss.NewStyle().Padding(0, 1).Foreground(lipgloss.Color("42"))
		ui.RenderTable(
			[]string{"ACTIVE", "NAME", "SERVER", "ORG"},
			rows,
			func(row, col int) *lipgloss.Style {
				if col == 0 && row >= 0 {
					return &active
				}
				return nil
			},
		)
		return nil
	})
}

func runProfileUse(_ *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}
	name := args[0]
	if err := cfg.Use(name); err != nil {
		// Help the user discover what they could have typed.
		ui.Error("✗ %v\n", err)
		fmt.Println("Available:")
		names := cfg.Names()
		sort.Strings(names)
		for _, n := range names {
			fmt.Printf("  %s  (%s)\n", n, cfg.Profiles[n].Server)
		}
		return fmt.Errorf("no such profile")
	}
	if err := config.Save(cfg); err != nil {
		return err
	}
	p := cfg.Profiles[name]
	ui.Success("✓ Active profile: %s  (%s)\n", name, p.Server)
	return nil
}

func runProfileCurrent(_ *cobra.Command, _ []string) error {
	cfg, err := config.Load()
	if err != nil || cfg.Active == "" {
		ui.InfoLn("No active profile — run `conure login` first")
		return nil
	}
	fmt.Println(cfg.Active)
	return nil
}

func runProfileRemove(_ *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}
	name := args[0]
	wasActive := cfg.Active == name
	if err := cfg.Remove(name); err != nil {
		return err
	}
	if err := config.Save(cfg); err != nil {
		return err
	}
	ui.Success("✓ Removed profile `%s`\n", name)
	if wasActive && len(cfg.Profiles) > 0 {
		ui.InfoLn("  That profile was active. Use `conure profile use <name>` to activate another.")
	}
	return nil
}
