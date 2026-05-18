package main

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/coffeenights/conure/internal/cli/config"
	"github.com/coffeenights/conure/internal/cli/ui"
)

var orgCmd = &cobra.Command{
	Use:     "org",
	Aliases: []string{"organization"},
	Short:   "Manage organizations",
}

var orgListCmd = &cobra.Command{
	Use:   "list",
	Short: "List organizations",
	RunE:  runOrgList,
}

var orgCreateUseFlag bool

var orgCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create an organization owned by your account",
	Long: `Create a new organization. You become its owner.

By default the new org is also set as the active org in
~/.conure/config.json (it's almost always what you want right after
creating one); pass --use=false to leave the active org unchanged.`,
	Args: cobra.ExactArgs(1),
	RunE: runOrgCreate,
}

var orgUseCmd = &cobra.Command{
	Use:   "use <name-or-id>",
	Short: "Set the active organization in ~/.conure/config.json",
	Args:  cobra.ExactArgs(1),
	RunE:  runOrgUse,
}

var orgCurrentCmd = &cobra.Command{
	Use:   "current",
	Short: "Print the active organization",
	RunE:  runOrgCurrent,
}

// switchCmd is a hidden back-compat shim so `conure switch org <name>` keeps
// working after the move to `conure org use <name>`. It delegates to the
// same RunE.
var switchCmd = &cobra.Command{
	Use:    "switch",
	Hidden: true,
	Short:  "Deprecated: use `conure org use`",
}

var switchOrgCmd = &cobra.Command{
	Use:    "org <name-or-id>",
	Hidden: true,
	Args:   cobra.ExactArgs(1),
	RunE:   runOrgUse,
}

func init() {
	orgCreateCmd.Flags().BoolVar(&orgCreateUseFlag, "use", true,
		"Set the new org as the active org after creating it")
	orgCmd.AddCommand(orgListCmd)
	orgCmd.AddCommand(orgCreateCmd)
	orgCmd.AddCommand(orgUseCmd)
	orgCmd.AddCommand(orgCurrentCmd)
	switchCmd.AddCommand(switchOrgCmd)
	rootCmd.AddCommand(orgCmd)
	rootCmd.AddCommand(switchCmd)
}

func runOrgList(cmd *cobra.Command, _ []string) error {
	_, prof, client, err := requireAuthClient()
	if err != nil {
		return err
	}
	orgs, err := client.ListOrganizations(cmd.Context())
	if err != nil {
		return err
	}
	return ui.Render(orgs, func() error {
		if len(orgs) == 0 {
			ui.InfoLn("No organizations found")
			return nil
		}
		rows := make([][]string, len(orgs))
		for i, o := range orgs {
			marker := ""
			if o.ID == prof.ActiveOrg {
				marker = "*"
			}
			rows[i] = []string{marker, o.ID, o.Name}
		}
		active := lipgloss.NewStyle().Padding(0, 1).Foreground(lipgloss.Color("42"))
		ui.RenderTable(
			[]string{"ACTIVE", "ID", "NAME"},
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

func runOrgCreate(cmd *cobra.Command, args []string) error {
	cfg, prof, client, err := requireAuthClient()
	if err != nil {
		return err
	}
	name := args[0]
	org, err := client.CreateOrganization(cmd.Context(), name)
	if err != nil {
		return err
	}
	// Persist as active before rendering so a --output=json scripted run
	// still gets the side effect; mirrors resolveOrg/init sticky behavior.
	if orgCreateUseFlag {
		prof.ActiveOrg = org.ID
		if err := config.Save(cfg); err != nil {
			return fmt.Errorf("org created but saving active org failed: %w", err)
		}
	}
	return ui.Render(org, func() error {
		ui.Success("✓ Created org: %s (%s)\n", org.Name, org.ID)
		if orgCreateUseFlag {
			ui.Info("Set active org (%s) — change with `conure org use <name>`\n", org.ID)
		}
		return nil
	})
}

func runOrgUse(cmd *cobra.Command, args []string) error {
	cfg, prof, client, err := requireAuthClient()
	if err != nil {
		return err
	}
	orgs, err := client.ListOrganizations(cmd.Context())
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
		ui.Error("✗ No organization matches `%s`\n", target)
		fmt.Println("Available:")
		for _, o := range orgs {
			fmt.Printf("  %s  (%s)\n", o.Name, o.ID)
		}
		return fmt.Errorf("no match")
	}
	prof.ActiveOrg = matchID
	if err := config.Save(cfg); err != nil {
		return err
	}
	ui.Success("✓ Active org: %s (%s)\n", matchName, matchID)
	return nil
}

func runOrgCurrent(_ *cobra.Command, _ []string) error {
	_, prof, _, err := requireAuthClient()
	if err != nil {
		return err
	}
	if prof.ActiveOrg == "" {
		ui.InfoLn("No active organization — run `conure org use <name>`")
		return nil
	}
	fmt.Println(prof.ActiveOrg)
	return nil
}
