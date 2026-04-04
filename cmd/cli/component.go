package main

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/coffeenights/conure/pkg/api"
	"github.com/spf13/cobra"
)

var componentCmd = &cobra.Command{
	Use:     "component",
	Aliases: []string{"comp"},
	Short:   "Manage components",
}

var componentListCmd = &cobra.Command{
	Use:   "list",
	Short: "List components of an application",
	RunE:  runComponentList,
}

var componentStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Get component status",
	RunE:  runComponentStatus,
}

func init() {
	componentListCmd.Flags().String("org", "", "Organization ID")
	componentListCmd.Flags().String("app", "", "Application ID")
	componentListCmd.Flags().String("env", "", "Environment name")
	componentListCmd.MarkFlagRequired("org")
	componentListCmd.MarkFlagRequired("app")
	componentListCmd.MarkFlagRequired("env")

	componentStatusCmd.Flags().String("org", "", "Organization ID")
	componentStatusCmd.Flags().String("app", "", "Application ID")
	componentStatusCmd.Flags().String("env", "", "Environment name")
	componentStatusCmd.Flags().String("component", "", "Component ID")
	componentStatusCmd.MarkFlagRequired("org")
	componentStatusCmd.MarkFlagRequired("app")
	componentStatusCmd.MarkFlagRequired("env")
	componentStatusCmd.MarkFlagRequired("component")

	componentCmd.AddCommand(componentListCmd)
	componentCmd.AddCommand(componentStatusCmd)
	rootCmd.AddCommand(componentCmd)
}

func runComponentList(cmd *cobra.Command, args []string) error {
	cfg, err := requireAuth()
	if err != nil {
		return err
	}
	client := newClient(cfg)

	orgID, _ := cmd.Flags().GetString("org")
	appID, _ := cmd.Flags().GetString("app")
	env, _ := cmd.Flags().GetString("env")

	data, err := client.get(fmt.Sprintf("/organizations/%s/a/%s/e/%s/c", orgID, appID, env))
	if err != nil {
		return err
	}

	if outputFlag == "json" {
		fmt.Println(string(data))
		return nil
	}

	var resp api.ComponentListResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return fmt.Errorf("parsing response: %w", err)
	}

	if len(resp.Components) == 0 {
		info.Println("No components found")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	header.Fprintln(w, "ID\tNAME\tTYPE")
	for _, c := range resp.Components {
		fmt.Fprintf(w, "%s\t%s\t%s\n", c.ID, c.Name, c.Type)
	}
	return w.Flush()
}

func runComponentStatus(cmd *cobra.Command, args []string) error {
	cfg, err := requireAuth()
	if err != nil {
		return err
	}
	client := newClient(cfg)

	orgID, _ := cmd.Flags().GetString("org")
	appID, _ := cmd.Flags().GetString("app")
	env, _ := cmd.Flags().GetString("env")
	compID, _ := cmd.Flags().GetString("component")

	data, err := client.get(fmt.Sprintf("/organizations/%s/a/%s/e/%s/c/%s/status", orgID, appID, env, compID))
	if err != nil {
		return err
	}

	if outputFlag == "json" {
		fmt.Println(string(data))
		return nil
	}

	var resp api.ComponentStatusResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return fmt.Errorf("parsing response: %w", err)
	}

	header.Println("Component Status")
	fmt.Println()

	if resp.Properties.Source != nil {
		header.Println("  Source")
		fmt.Printf("    Image:   %s\n", resp.Properties.Source.ContainerImage)
		fmt.Printf("    Command: %s\n", resp.Properties.Source.Command)
		fmt.Println()
	}

	if resp.Properties.Resources != nil {
		header.Println("  Resources")
		fmt.Printf("    Replicas: %d\n", resp.Properties.Resources.Replicas)
		fmt.Printf("    CPU:      %s\n", resp.Properties.Resources.CPU)
		fmt.Printf("    Memory:   %s\n", resp.Properties.Resources.Memory)
		fmt.Println()
	}

	if resp.Properties.Network != nil {
		header.Println("  Network")
		fmt.Printf("    IP:          %s\n", resp.Properties.Network.IP)
		fmt.Printf("    External IP: %s\n", resp.Properties.Network.ExternalIP)
		if len(resp.Properties.Network.Ports) > 0 {
			fmt.Printf("    Ports:       %v\n", resp.Properties.Network.Ports)
		}
		fmt.Println()
	}

	if resp.Properties.Health != nil {
		header.Println("  Health")
		if resp.Properties.Health.Healthy {
			success.Printf("    Healthy: %v\n", resp.Properties.Health.Healthy)
		} else {
			errC.Printf("    Healthy: %v\n", resp.Properties.Health.Healthy)
		}
		if resp.Properties.Health.Message != "" {
			fmt.Printf("    Message: %s\n", resp.Properties.Health.Message)
		}
	}

	return nil
}
