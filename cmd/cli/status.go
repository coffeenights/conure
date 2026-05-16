package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/coffeenights/conure/internal/cli/ui"
	"github.com/coffeenights/conure/pkg/api"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show live state, last deploy, and drift for this component",
	RunE:  runStatus,
}

var diffCmd = &cobra.Command{
	Use:   "diff",
	Short: "Show only the drift entries between live state and last deploy",
	RunE:  runDiff,
}

func init() {
	addEnvFlag(statusCmd)
	addEnvFlag(diffCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(diffCmd)
}

func runStatus(cmd *cobra.Command, _ []string) error {
	resp, err := loadComponentInEnv(cmd)
	if err != nil {
		return err
	}
	return ui.Render(resp, func() error {
		ui.Header("Component: ")
		fmt.Printf("%s  (%s)\n", resp.Name, resp.ComponentID)
		ui.Header("Environment: ")
		fmt.Printf("%s\n\n", resp.EnvironmentName)

		if resp.DeployedRevision != nil {
			ui.HeaderLn("Deployed")
			fmt.Printf("  v%d  (%s)\n", resp.DeployedRevision.Version, resp.DeployedRevision.ID)
			if resp.DeployedRevision.DeployedAt != nil {
				fmt.Printf("  at %s\n", resp.DeployedRevision.DeployedAt.Format("2006-01-02 15:04:05"))
			}
			fmt.Println()
		}
		if resp.LatestDraft != nil {
			ui.HeaderLn("Latest draft")
			fmt.Printf("  v%d  (%s)\n\n", resp.LatestDraft.Version, resp.LatestDraft.ID)
		}

		ui.HeaderLn("Health")
		if resp.HealthCondition != "" {
			switch resp.HealthStatus {
			case "True":
				ui.Success("  %s: %s\n", resp.HealthCondition, resp.HealthStatus)
			case "False":
				ui.Error("  %s: %s\n", resp.HealthCondition, resp.HealthStatus)
			default:
				ui.Info("  %s: %s\n", resp.HealthCondition, resp.HealthStatus)
			}
			if resp.HealthMessage != "" {
				fmt.Printf("  %s\n", resp.HealthMessage)
			}
		} else {
			ui.DimLn("  unknown")
		}
		fmt.Println()

		ui.HeaderLn("Drift")
		if !resp.Drifted {
			ui.SuccessLn("  none")
		} else {
			ui.Error("  %d entries\n", len(resp.Diff))
			printDriftEntries(resp.Diff)
		}
		return nil
	})
}

func runDiff(cmd *cobra.Command, _ []string) error {
	resp, err := loadComponentInEnv(cmd)
	if err != nil {
		return err
	}
	return ui.Render(resp.Diff, func() error {
		if !resp.Drifted {
			ui.SuccessLn("✓ No drift")
			return nil
		}
		printDriftEntries(resp.Diff)
		return nil
	})
}

func loadComponentInEnv(cmd *cobra.Command) (*api.ComponentInEnvResponse, error) {
	lc, err := resolveTarget(cmd)
	if err != nil {
		return nil, err
	}
	return lc.Client.GetComponentInEnv(cmd.Context(), lc.Link.OrgID, lc.Link.AppID, lc.Env, lc.Link.ComponentID)
}

func printDriftEntries(entries []api.DriftEntry) {
	for _, e := range entries {
		fmt.Printf("  %s\n", e.Path)
		fmt.Printf("    live:     %v\n", e.Live)
		fmt.Printf("    deployed: %v\n", e.Deployed)
	}
}
