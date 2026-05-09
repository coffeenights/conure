package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/spf13/cobra"
)

var historyCmd = &cobra.Command{
	Use:     "history",
	Aliases: []string{"revisions"},
	Short:   "List revisions for this component in this env (newest first)",
	RunE:    runHistory,
}

var rollbackCmd = &cobra.Command{
	Use:   "rollback <version>",
	Short: "Re-deploy a historical revision (creates a new deployed revision at head)",
	Args:  cobra.ExactArgs(1),
	RunE:  runRollback,
}

var promoteCmd = &cobra.Command{
	Use:   "promote",
	Short: "Promote the latest deployed revision from this env to another",
	RunE:  runPromote,
}

var draftCmd = &cobra.Command{
	Use:   "draft",
	Short: "Edit revision drafts",
}

var draftEditCmd = &cobra.Command{
	Use:   "edit",
	Short: "Open the latest draft (or last deployed) values in $EDITOR; saves as a new draft",
	RunE:  runDraftEdit,
}

func init() {
	historyCmd.Flags().String("env", "", "Environment (overrides link)")
	rollbackCmd.Flags().String("env", "", "Environment (overrides link)")
	promoteCmd.Flags().String("from", "", "Source environment (defaults to link)")
	promoteCmd.Flags().String("to", "", "Target environment (required)")
	_ = promoteCmd.MarkFlagRequired("to")
	draftEditCmd.Flags().String("env", "", "Environment (overrides link)")

	draftCmd.AddCommand(draftEditCmd)
	rootCmd.AddCommand(historyCmd)
	rootCmd.AddCommand(rollbackCmd)
	rootCmd.AddCommand(promoteCmd)
	rootCmd.AddCommand(draftCmd)
}

func runHistory(cmd *cobra.Command, args []string) error {
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
	revs, err := listRevisions(client, link.OrgID, link.AppID, env, link.ComponentID)
	if err != nil {
		return err
	}
	if outputFlag == "json" {
		out, _ := json.MarshalIndent(revs, "", "  ")
		fmt.Println(string(out))
		return nil
	}
	if len(revs) == 0 {
		info.Println("No revisions yet")
		return nil
	}

	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212")).Padding(0, 1)
	cellStyle := lipgloss.NewStyle().Padding(0, 1)
	t := table.New().
		Border(lipgloss.RoundedBorder()).
		BorderStyle(lipgloss.NewStyle().Foreground(lipgloss.Color("240"))).
		Headers("VERSION", "STATUS", "DEPLOYED AT", "REVISION ID").
		StyleFunc(func(row, col int) lipgloss.Style {
			if row == table.HeaderRow {
				return headerStyle
			}
			return cellStyle
		})
	for _, r := range revs {
		deployedAt := "-"
		if r.DeployedAt != nil {
			deployedAt = r.DeployedAt.Format("2006-01-02 15:04:05")
		}
		t.Row(fmt.Sprintf("v%d", r.Version), r.Status, deployedAt, r.ID)
	}
	fmt.Println(t)
	return nil
}

func runRollback(cmd *cobra.Command, args []string) error {
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

	versionArg := strings.TrimPrefix(args[0], "v")
	version, err := strconv.Atoi(versionArg)
	if err != nil {
		return fmt.Errorf("invalid version %q (expected number, optionally prefixed with 'v')", args[0])
	}

	client := newClient(cfg)
	revs, err := listRevisions(client, link.OrgID, link.AppID, env, link.ComponentID)
	if err != nil {
		return err
	}
	var revID string
	for _, r := range revs {
		if r.Version == version {
			revID = r.ID
			break
		}
	}
	if revID == "" {
		return fmt.Errorf("no revision v%d found in env %s", version, env)
	}

	sp := startSpinner(fmt.Sprintf("Rolling back to v%d…", version))
	rev, err := deployRevision(client, link.OrgID, link.AppID, env, link.ComponentID, revID)
	stopSpinner(sp)
	if err != nil {
		errC.Printf("✗ Rollback failed: %v\n", err)
		return err
	}
	success.Printf("✓ Deployed v%d (copied from v%d)\n", rev.Version, version)
	return nil
}

func runPromote(cmd *cobra.Command, args []string) error {
	cfg, err := requireAuth()
	if err != nil {
		return err
	}
	link, err := requireLink()
	if err != nil {
		return err
	}
	from, _ := cmd.Flags().GetString("from")
	if from == "" {
		from = link.Environment
	}
	to, _ := cmd.Flags().GetString("to")
	if from == to {
		return fmt.Errorf("--from and --to must differ (got %q)", from)
	}
	client := newClient(cfg)
	sp := startSpinner(fmt.Sprintf("Promoting %s → %s…", from, to))
	rev, err := promoteComponent(client, link.OrgID, link.AppID, link.ComponentID, from, to)
	stopSpinner(sp)
	if err != nil {
		errC.Printf("✗ Promote failed: %v\n", err)
		return err
	}
	success.Printf("✓ Created draft v%d in `%s` from latest deployed in `%s`\n", rev.Version, to, from)
	info.Println("  Run `conure deploy --env " + to + "` to apply.")
	return nil
}

func runDraftEdit(cmd *cobra.Command, args []string) error {
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

	resp, err := getComponentInEnv(client, link.OrgID, link.AppID, env, link.ComponentID)
	if err != nil {
		return err
	}

	// Pick the values to edit: prefer existing draft, else last deployed.
	var seedValues map[string]interface{}
	var draftRevID string
	if resp.LatestDraft != nil {
		seedValues = resp.LatestDraft.Values
		draftRevID = resp.LatestDraft.ID
	} else if resp.DeployedRevision != nil {
		seedValues = resp.DeployedRevision.Values
	} else {
		seedValues = map[string]interface{}{}
	}

	tmpDir, err := os.MkdirTemp("", "conure-draft-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)
	tmpPath := filepath.Join(tmpDir, "values.json")
	seedJSON, _ := json.MarshalIndent(seedValues, "", "  ")
	if err := os.WriteFile(tmpPath, seedJSON, 0600); err != nil {
		return err
	}

	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vi"
	}
	editCmd := exec.Command(editor, tmpPath)
	editCmd.Stdin = os.Stdin
	editCmd.Stdout = os.Stdout
	editCmd.Stderr = os.Stderr
	if err := editCmd.Run(); err != nil {
		return fmt.Errorf("editor exited with error: %w", err)
	}

	editedRaw, err := os.ReadFile(tmpPath)
	if err != nil {
		return err
	}
	var newValues map[string]interface{}
	if err := json.Unmarshal(editedRaw, &newValues); err != nil {
		return fmt.Errorf("edited file is not valid JSON: %w", err)
	}

	if draftRevID != "" {
		rev, err := updateRevision(client, link.OrgID, link.AppID, env, link.ComponentID, draftRevID, newValues)
		if err != nil {
			return err
		}
		success.Printf("✓ Updated draft v%d\n", rev.Version)
	} else {
		rev, err := createRevision(client, link.OrgID, link.AppID, env, link.ComponentID, newValues)
		if err != nil {
			return err
		}
		success.Printf("✓ Created draft v%d\n", rev.Version)
	}
	info.Println("  Run `conure deploy` to apply.")
	return nil
}
