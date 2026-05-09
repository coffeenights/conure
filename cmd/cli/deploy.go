package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var deployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Deploy this component's latest draft",
	RunE:  runDeploy,
}

func init() {
	deployCmd.Flags().String("env", "", "Environment to deploy to (overrides link)")
	rootCmd.AddCommand(deployCmd)
}

func runDeploy(cmd *cobra.Command, args []string) error {
	cfg, err := requireAuth()
	if err != nil {
		return err
	}
	link, err := requireLink()
	if err != nil {
		return err
	}
	env := link.Environment
	if v, _ := cmd.Flags().GetString("env"); v != "" {
		env = v
	}

	client := newClient(cfg)
	sp := startSpinner(fmt.Sprintf("Deploying `%s` to `%s`…", link.ComponentName, env))
	rev, err := deployLatestDraft(client, link.OrgID, link.AppID, env, link.ComponentID)
	stopSpinner(sp)
	if err != nil {
		errC.Printf("✗ Deploy failed: %v\n", err)
		return err
	}
	success.Printf("✓ Deployed v%d (%s) to %s\n", rev.Version, rev.ID, env)
	return nil
}
