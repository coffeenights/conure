package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/coffeenights/conure/internal/cli/ui"
)

var deployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Deploy this component's latest draft",
	RunE:  runDeploy,
}

func init() {
	addEnvFlag(deployCmd)
	rootCmd.AddCommand(deployCmd)
}

func runDeploy(cmd *cobra.Command, _ []string) error {
	lc, err := requireLinked(cmd)
	if err != nil {
		return err
	}
	sp := ui.StartSpinner(fmt.Sprintf("Deploying `%s` to `%s`…", lc.Link.ComponentName, lc.Env))
	rev, err := lc.Client.DeployLatestDraft(cmd.Context(), lc.Link.OrgID, lc.Link.AppID, lc.Env, lc.Link.ComponentID)
	ui.StopSpinner(sp)
	if err != nil {
		return err
	}
	ui.Success("✓ Deployed v%d (%s) to %s\n", rev.Version, rev.ID, lc.Env)
	return nil
}
