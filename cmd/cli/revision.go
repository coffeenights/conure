package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/coffeenights/conure/internal/cli/ui"
)

// Canonical command group: `conure revision <verb>` (alias: `rev`).
var revisionCmd = &cobra.Command{
	Use:     "revision",
	Aliases: []string{"rev"},
	Short:   "Inspect and manage revisions of the linked component",
}

var revisionListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"history", "revisions"},
	Short:   "List revisions for this component in this env (newest first)",
	RunE:    runRevisionList,
}

var revisionRollbackCmd = &cobra.Command{
	Use:   "rollback <version>",
	Short: "Re-deploy a historical revision (creates a new deployed revision at head)",
	Args:  cobra.ExactArgs(1),
	RunE:  runRevisionRollback,
}

var revisionPromoteCmd = &cobra.Command{
	Use:   "promote",
	Short: "Promote the latest deployed revision from this env to another",
	RunE:  runRevisionPromote,
}

var revisionDraftCmd = &cobra.Command{
	Use:   "draft",
	Short: "Edit revision drafts",
}

var revisionDraftEditCmd = &cobra.Command{
	Use:   "edit",
	Short: "Open the latest draft (or last deployed) values in $EDITOR; saves as a new draft",
	RunE:  runRevisionDraftEdit,
}

// ---- back-compat hidden aliases at the top level ------------------------
//
// Users who learned the older flat layout still get the same command. They
// are hidden from `--help` so the canonical surface is just `revision …`.

var historyAliasCmd = &cobra.Command{
	Use:     "history",
	Aliases: []string{"revisions"},
	Hidden:  true,
	Short:   "Alias for `revision list`",
	RunE:    runRevisionList,
}

var rollbackAliasCmd = &cobra.Command{
	Use:    "rollback <version>",
	Hidden: true,
	Short:  "Alias for `revision rollback`",
	Args:   cobra.ExactArgs(1),
	RunE:   runRevisionRollback,
}

var promoteAliasCmd = &cobra.Command{
	Use:    "promote",
	Hidden: true,
	Short:  "Alias for `revision promote`",
	RunE:   runRevisionPromote,
}

var draftAliasCmd = &cobra.Command{
	Use:    "draft",
	Hidden: true,
	Short:  "Alias for `revision draft`",
}

var draftEditAliasCmd = &cobra.Command{
	Use:    "edit",
	Hidden: true,
	Short:  "Alias for `revision draft edit`",
	RunE:   runRevisionDraftEdit,
}

func init() {
	addEnvFlag(revisionListCmd)
	addEnvFlag(revisionRollbackCmd)
	revisionPromoteCmd.Flags().String("from", "", "Source environment (defaults to link)")
	revisionPromoteCmd.Flags().String("to", "", "Target environment (required)")
	_ = revisionPromoteCmd.MarkFlagRequired("to")
	addEnvFlag(revisionDraftEditCmd)

	revisionDraftCmd.AddCommand(revisionDraftEditCmd)
	revisionCmd.AddCommand(revisionListCmd)
	revisionCmd.AddCommand(revisionRollbackCmd)
	revisionCmd.AddCommand(revisionPromoteCmd)
	revisionCmd.AddCommand(revisionDraftCmd)
	rootCmd.AddCommand(revisionCmd)

	// Mirror the same flag declarations on the hidden aliases so back-compat
	// invocations get identical behavior.
	addEnvFlag(historyAliasCmd)
	addEnvFlag(rollbackAliasCmd)
	promoteAliasCmd.Flags().String("from", "", "Source environment (defaults to link)")
	promoteAliasCmd.Flags().String("to", "", "Target environment (required)")
	_ = promoteAliasCmd.MarkFlagRequired("to")
	addEnvFlag(draftEditAliasCmd)
	draftAliasCmd.AddCommand(draftEditAliasCmd)
	rootCmd.AddCommand(historyAliasCmd)
	rootCmd.AddCommand(rollbackAliasCmd)
	rootCmd.AddCommand(promoteAliasCmd)
	rootCmd.AddCommand(draftAliasCmd)
}

func runRevisionList(cmd *cobra.Command, _ []string) error {
	lc, err := requireLinked(cmd)
	if err != nil {
		return err
	}
	revs, err := lc.Client.ListRevisions(cmd.Context(), lc.Link.OrgID, lc.Link.AppID, lc.Env, lc.Link.ComponentID)
	if err != nil {
		return err
	}
	return ui.Render(revs, func() error {
		if len(revs) == 0 {
			ui.InfoLn("No revisions yet")
			return nil
		}
		rows := make([][]string, len(revs))
		for i, r := range revs {
			deployedAt := "-"
			if r.DeployedAt != nil {
				deployedAt = r.DeployedAt.Format("2006-01-02 15:04:05")
			}
			rows[i] = []string{fmt.Sprintf("v%d", r.Version), r.Status, deployedAt, r.ID}
		}
		ui.RenderTable([]string{"VERSION", "STATUS", "DEPLOYED AT", "REVISION ID"}, rows, nil)
		return nil
	})
}

func runRevisionRollback(cmd *cobra.Command, args []string) error {
	lc, err := requireLinked(cmd)
	if err != nil {
		return err
	}

	versionArg := strings.TrimPrefix(args[0], "v")
	version, err := strconv.Atoi(versionArg)
	if err != nil {
		return fmt.Errorf("invalid version %q (expected number, optionally prefixed with 'v')", args[0])
	}

	revs, err := lc.Client.ListRevisions(cmd.Context(), lc.Link.OrgID, lc.Link.AppID, lc.Env, lc.Link.ComponentID)
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
		return fmt.Errorf("no revision v%d found in env %s", version, lc.Env)
	}

	sp := ui.StartSpinner(fmt.Sprintf("Rolling back to v%d…", version))
	rev, err := lc.Client.DeployRevision(cmd.Context(), lc.Link.OrgID, lc.Link.AppID, lc.Env, lc.Link.ComponentID, revID)
	ui.StopSpinner(sp)
	if err != nil {
		return err
	}
	ui.Success("✓ Deployed v%d (copied from v%d)\n", rev.Version, version)
	return nil
}

func runRevisionPromote(cmd *cobra.Command, _ []string) error {
	lc, err := requireLinked(cmd)
	if err != nil {
		return err
	}
	from, _ := cmd.Flags().GetString("from")
	if from == "" {
		from = lc.Link.Environment
	}
	to, _ := cmd.Flags().GetString("to")
	if from == to {
		return fmt.Errorf("--from and --to must differ (got %q)", from)
	}
	sp := ui.StartSpinner(fmt.Sprintf("Promoting %s → %s…", from, to))
	rev, err := lc.Client.Promote(cmd.Context(), lc.Link.OrgID, lc.Link.AppID, lc.Link.ComponentID, from, to)
	ui.StopSpinner(sp)
	if err != nil {
		return err
	}
	ui.Success("✓ Created draft v%d in `%s` from latest deployed in `%s`\n", rev.Version, to, from)
	ui.InfoLn("  Run `conure deploy --env " + to + "` to apply.")
	return nil
}

func runRevisionDraftEdit(cmd *cobra.Command, _ []string) error {
	lc, err := requireLinked(cmd)
	if err != nil {
		return err
	}

	resp, err := lc.Client.GetComponentInEnv(cmd.Context(), lc.Link.OrgID, lc.Link.AppID, lc.Env, lc.Link.ComponentID)
	if err != nil {
		return err
	}

	// Pick the values to edit: prefer existing draft, else last deployed.
	var seedValues map[string]interface{}
	var draftRevID string
	switch {
	case resp.LatestDraft != nil:
		seedValues = resp.LatestDraft.Values
		draftRevID = resp.LatestDraft.ID
	case resp.DeployedRevision != nil:
		seedValues = resp.DeployedRevision.Values
	default:
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
		rev, err := lc.Client.UpdateRevision(cmd.Context(), lc.Link.OrgID, lc.Link.AppID, lc.Env, lc.Link.ComponentID, draftRevID, newValues)
		if err != nil {
			return err
		}
		ui.Success("✓ Updated draft v%d\n", rev.Version)
	} else {
		rev, err := lc.Client.CreateRevision(cmd.Context(), lc.Link.OrgID, lc.Link.AppID, lc.Env, lc.Link.ComponentID, newValues)
		if err != nil {
			return err
		}
		ui.Success("✓ Created draft v%d\n", rev.Version)
	}
	ui.InfoLn("  Run `conure deploy` to apply.")
	return nil
}
