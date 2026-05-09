package main

import (
	"encoding/json"
	"fmt"

	"github.com/coffeenights/conure/pkg/api"
	"github.com/spf13/cobra"
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
	statusCmd.Flags().String("env", "", "Environment (overrides link)")
	diffCmd.Flags().String("env", "", "Environment (overrides link)")
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(diffCmd)
}

func runStatus(cmd *cobra.Command, args []string) error {
	resp, err := loadComponentInEnv(cmd)
	if err != nil {
		return err
	}

	if outputFlag == "json" {
		out, _ := json.MarshalIndent(resp, "", "  ")
		fmt.Println(string(out))
		return nil
	}

	header.Printf("Component: ")
	fmt.Printf("%s  (%s)\n", resp.Name, resp.ComponentID)
	header.Printf("Environment: ")
	fmt.Printf("%s\n\n", resp.EnvironmentName)

	if resp.DeployedRevision != nil {
		header.Println("Deployed")
		fmt.Printf("  v%d  (%s)\n", resp.DeployedRevision.Version, resp.DeployedRevision.ID)
		if resp.DeployedRevision.DeployedAt != nil {
			fmt.Printf("  at %s\n", resp.DeployedRevision.DeployedAt.Format("2006-01-02 15:04:05"))
		}
		fmt.Println()
	}
	if resp.LatestDraft != nil {
		header.Println("Latest draft")
		fmt.Printf("  v%d  (%s)\n\n", resp.LatestDraft.Version, resp.LatestDraft.ID)
	}

	header.Println("Health")
	if resp.HealthCondition != "" {
		switch resp.HealthStatus {
		case "True":
			success.Printf("  %s: %s\n", resp.HealthCondition, resp.HealthStatus)
		case "False":
			errC.Printf("  %s: %s\n", resp.HealthCondition, resp.HealthStatus)
		default:
			info.Printf("  %s: %s\n", resp.HealthCondition, resp.HealthStatus)
		}
		if resp.HealthMessage != "" {
			fmt.Printf("  %s\n", resp.HealthMessage)
		}
	} else {
		dim.Println("  unknown")
	}
	fmt.Println()

	header.Println("Drift")
	if !resp.Drifted {
		success.Println("  none")
	} else {
		errC.Printf("  %d entries\n", len(resp.Diff))
		printDriftEntries(resp.Diff)
	}
	return nil
}

func runDiff(cmd *cobra.Command, args []string) error {
	resp, err := loadComponentInEnv(cmd)
	if err != nil {
		return err
	}
	if outputFlag == "json" {
		out, _ := json.MarshalIndent(resp.Diff, "", "  ")
		fmt.Println(string(out))
		return nil
	}
	if !resp.Drifted {
		success.Println("✓ No drift")
		return nil
	}
	printDriftEntries(resp.Diff)
	return nil
}

func loadComponentInEnv(cmd *cobra.Command) (*api.ComponentInEnvResponse, error) {
	cfg, err := requireAuth()
	if err != nil {
		return nil, err
	}
	link, err := requireLink()
	if err != nil {
		return nil, err
	}
	env := link.Environment
	if v, _ := cmd.Flags().GetString("env"); v != "" {
		env = v
	}
	client := newClient(cfg)
	return getComponentInEnv(client, link.OrgID, link.AppID, env, link.ComponentID)
}

func printDriftEntries(entries []api.DriftEntry) {
	for _, e := range entries {
		fmt.Printf("  %s\n", e.Path)
		fmt.Printf("    live:     %v\n", e.Live)
		fmt.Printf("    deployed: %v\n", e.Deployed)
	}
}
