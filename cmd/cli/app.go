package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/coffeenights/conure/pkg/api"
	"github.com/spf13/cobra"
)

var appCmd = &cobra.Command{
	Use:   "app",
	Short: "Manage applications",
}

var appListCmd = &cobra.Command{
	Use:   "list",
	Short: "List applications in an organization",
	RunE:  runAppList,
}

var appCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create an application",
	RunE:  runAppCreate,
}

var appDeployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Deploy an application to an environment",
	RunE:  runAppDeploy,
}

var appStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Get application status",
	RunE:  runAppStatus,
}

func init() {
	appListCmd.Flags().String("org", "", "Organization ID")
	appListCmd.MarkFlagRequired("org")

	appCreateCmd.Flags().String("org", "", "Organization ID")
	appCreateCmd.Flags().String("name", "", "Application name")
	appCreateCmd.Flags().String("description", "", "Application description")
	appCreateCmd.MarkFlagRequired("org")
	appCreateCmd.MarkFlagRequired("name")

	appDeployCmd.Flags().String("org", "", "Organization ID")
	appDeployCmd.Flags().String("app", "", "Application ID")
	appDeployCmd.Flags().String("env", "", "Environment name")
	appDeployCmd.MarkFlagRequired("org")
	appDeployCmd.MarkFlagRequired("app")
	appDeployCmd.MarkFlagRequired("env")

	appStatusCmd.Flags().String("org", "", "Organization ID")
	appStatusCmd.Flags().String("app", "", "Application ID")
	appStatusCmd.Flags().String("env", "", "Environment name")
	appStatusCmd.MarkFlagRequired("org")
	appStatusCmd.MarkFlagRequired("app")
	appStatusCmd.MarkFlagRequired("env")

	appCmd.AddCommand(appListCmd)
	appCmd.AddCommand(appCreateCmd)
	appCmd.AddCommand(appDeployCmd)
	appCmd.AddCommand(appStatusCmd)
	rootCmd.AddCommand(appCmd)
}

func runAppList(cmd *cobra.Command, args []string) error {
	cfg, err := requireAuth()
	if err != nil {
		return err
	}
	client := newClient(cfg)
	orgID, _ := cmd.Flags().GetString("org")

	data, err := client.get(fmt.Sprintf("/organizations/%s/a", orgID))
	if err != nil {
		return err
	}

	if outputFlag == "json" {
		fmt.Println(string(data))
		return nil
	}

	var resp api.ApplicationListResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return fmt.Errorf("parsing response: %w", err)
	}

	if len(resp.Applications) == 0 {
		info.Println("No applications found")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	header.Fprintln(w, "ID\tNAME\tDESCRIPTION\tENVIRONMENTS")
	for _, app := range resp.Applications {
		envNames := make([]string, len(app.Environments))
		for i, e := range app.Environments {
			envNames[i] = e.Name
		}
		envStr := "-"
		if len(envNames) > 0 {
			envStr = strings.Join(envNames, ", ")
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", app.ID, app.Name, app.Description, envStr)
	}
	return w.Flush()
}

func runAppCreate(cmd *cobra.Command, args []string) error {
	cfg, err := requireAuth()
	if err != nil {
		return err
	}
	client := newClient(cfg)

	orgID, _ := cmd.Flags().GetString("org")
	name, _ := cmd.Flags().GetString("name")
	description, _ := cmd.Flags().GetString("description")

	req := api.CreateApplicationRequest{Name: name, Description: description}

	data, err := client.post(fmt.Sprintf("/organizations/%s/a", orgID), req)
	if err != nil {
		return err
	}

	if outputFlag == "json" {
		fmt.Println(string(data))
		return nil
	}

	var resp api.ApplicationResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return fmt.Errorf("parsing response: %w", err)
	}

	success.Printf("Application created: %s (%s)\n", resp.Name, resp.ID)
	return nil
}

func runAppDeploy(cmd *cobra.Command, args []string) error {
	cfg, err := requireAuth()
	if err != nil {
		return err
	}
	client := newClient(cfg)

	orgID, _ := cmd.Flags().GetString("org")
	appID, _ := cmd.Flags().GetString("app")
	env, _ := cmd.Flags().GetString("env")

	info.Printf("Deploying application %s to %s...\n", appID, env)

	data, err := client.put(fmt.Sprintf("/organizations/%s/a/%s/e/%s", orgID, appID, env), nil)
	if err != nil {
		return err
	}

	if outputFlag == "json" {
		fmt.Println(string(data))
		return nil
	}

	success.Println("Application deployed successfully")
	return nil
}

func runAppStatus(cmd *cobra.Command, args []string) error {
	cfg, err := requireAuth()
	if err != nil {
		return err
	}
	client := newClient(cfg)

	orgID, _ := cmd.Flags().GetString("org")
	appID, _ := cmd.Flags().GetString("app")
	env, _ := cmd.Flags().GetString("env")

	data, err := client.get(fmt.Sprintf("/organizations/%s/a/%s/e/%s/status", orgID, appID, env))
	if err != nil {
		return err
	}

	if outputFlag == "json" {
		fmt.Println(string(data))
		return nil
	}

	var resp api.ApplicationStatusResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return fmt.Errorf("parsing response: %w", err)
	}

	header.Printf("Application Status: ")
	switch resp.Status {
	case "running":
		success.Println(resp.Status)
	default:
		info.Println(resp.Status)
	}
	return nil
}
