package main

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/coffeenights/conure/pkg/api"
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

var orgCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create an organization",
	RunE:  runOrgCreate,
}

func init() {
	orgCreateCmd.Flags().String("name", "", "Organization name")
	orgCreateCmd.MarkFlagRequired("name")

	orgCmd.AddCommand(orgListCmd)
	orgCmd.AddCommand(orgCreateCmd)
	rootCmd.AddCommand(orgCmd)
}

func runOrgList(cmd *cobra.Command, args []string) error {
	cfg, err := requireAuth()
	if err != nil {
		return err
	}
	client := newClient(cfg)

	data, err := client.get("/organizations/")
	if err != nil {
		return err
	}

	if outputFlag == "json" {
		fmt.Println(string(data))
		return nil
	}

	var resp api.OrganizationListResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return fmt.Errorf("parsing response: %w", err)
	}

	if len(resp.Organizations) == 0 {
		info.Println("No organizations found")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	header.Fprintln(w, "ID\tNAME")
	for _, org := range resp.Organizations {
		fmt.Fprintf(w, "%s\t%s\n", org.ID, org.Name)
	}
	return w.Flush()
}

func runOrgCreate(cmd *cobra.Command, args []string) error {
	cfg, err := requireAuth()
	if err != nil {
		return err
	}
	client := newClient(cfg)

	name, _ := cmd.Flags().GetString("name")

	data, err := client.post("/organizations/", api.CreateOrganizationRequest{Name: name})
	if err != nil {
		return err
	}

	if outputFlag == "json" {
		fmt.Println(string(data))
		return nil
	}

	var resp api.OrganizationResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return fmt.Errorf("parsing response: %w", err)
	}

	success.Printf("Organization created: %s (%s)\n", resp.Name, resp.ID)
	return nil
}
