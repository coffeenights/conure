package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"sigs.k8s.io/yaml"

	"github.com/coffeenights/conure/internal/cli/ui"
	"github.com/coffeenights/conure/pkg/api"
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

var revisionShowCmd = &cobra.Command{
	Use:   "show [version]",
	Short: "Print a revision (defaults to latest draft, falls back to deployed)",
	Long: `Print a single revision in the current --output format (text, json, yaml).

With no positional argument, shows the latest draft for this component+env,
falling back to the latest deployed revision when no draft exists. Pass a
version number (or v<number>) to target a historical revision.

Examples:
  conure revision show -o yaml
  conure revision show 3 -o json
  conure revision show v5 --values-only -o yaml > values.yaml`,
	Args: cobra.MaximumNArgs(1),
	RunE: runRevisionShow,
}

var revisionCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new draft revision from a values file (.json, .yaml, .yml)",
	Long: `Read values from a file and create a new draft revision for this
component+env. The file may be JSON or YAML; the format is detected by
extension, or with --format.

Examples:
  conure revision create -f values.yaml
  conure revision create -f -            # read from stdin (default YAML)
  conure revision create -f patch.json`,
	RunE: runRevisionCreate,
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
	revisionDraftEditCmd.Flags().StringP("comment", "m", "",
		"Optional note attached to the draft (shown in `revision list`)")
	addEnvFlag(revisionShowCmd)
	revisionShowCmd.Flags().Bool("values-only", false,
		"Print just the values map (drops metadata like id, version, status)")
	addEnvFlag(revisionCreateCmd)
	revisionCreateCmd.Flags().StringP("file", "f", "",
		"Path to a JSON or YAML values file (use - for stdin)")
	revisionCreateCmd.Flags().String("format", "",
		"Override format detection: json or yaml (default: by extension, yaml for stdin)")
	revisionCreateCmd.Flags().StringP("comment", "m", "",
		"Optional note attached to the draft (shown in `revision list`)")
	_ = revisionCreateCmd.MarkFlagRequired("file")

	revisionDraftCmd.AddCommand(revisionDraftEditCmd)
	revisionCmd.AddCommand(revisionListCmd)
	revisionCmd.AddCommand(revisionRollbackCmd)
	revisionCmd.AddCommand(revisionPromoteCmd)
	revisionCmd.AddCommand(revisionDraftCmd)
	revisionCmd.AddCommand(revisionShowCmd)
	revisionCmd.AddCommand(revisionCreateCmd)
	rootCmd.AddCommand(revisionCmd)

	// Mirror the same flag declarations on the hidden aliases so back-compat
	// invocations get identical behavior.
	addEnvFlag(historyAliasCmd)
	addEnvFlag(rollbackAliasCmd)
	promoteAliasCmd.Flags().String("from", "", "Source environment (defaults to link)")
	promoteAliasCmd.Flags().String("to", "", "Target environment (required)")
	_ = promoteAliasCmd.MarkFlagRequired("to")
	addEnvFlag(draftEditAliasCmd)
	draftEditAliasCmd.Flags().StringP("comment", "m", "",
		"Optional note attached to the draft (shown in `revision list`)")
	draftAliasCmd.AddCommand(draftEditAliasCmd)
	rootCmd.AddCommand(historyAliasCmd)
	rootCmd.AddCommand(rollbackAliasCmd)
	rootCmd.AddCommand(promoteAliasCmd)
	rootCmd.AddCommand(draftAliasCmd)
}

func runRevisionList(cmd *cobra.Command, _ []string) error {
	lc, err := resolveTarget(cmd)
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
			comment := r.Comment
			if comment == "" {
				comment = "-"
			}
			rows[i] = []string{fmt.Sprintf("v%d", r.Version), r.Status, deployedAt, r.ID, comment}
		}
		ui.RenderTable([]string{"VERSION", "STATUS", "DEPLOYED AT", "REVISION ID", "COMMENT"}, rows, nil)
		return nil
	})
}

func runRevisionRollback(cmd *cobra.Command, args []string) error {
	lc, err := resolveTarget(cmd)
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
	lc, err := resolveTarget(cmd)
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
	lc, err := resolveTarget(cmd)
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

	comment, _ := cmd.Flags().GetString("comment")
	if draftRevID != "" {
		rev, err := lc.Client.UpdateRevision(cmd.Context(), lc.Link.OrgID, lc.Link.AppID, lc.Env, lc.Link.ComponentID, draftRevID, newValues, comment)
		if err != nil {
			return err
		}
		ui.Success("✓ Updated draft v%d\n", rev.Version)
	} else {
		rev, err := lc.Client.CreateRevision(cmd.Context(), lc.Link.OrgID, lc.Link.AppID, lc.Env, lc.Link.ComponentID, newValues, comment)
		if err != nil {
			return err
		}
		ui.Success("✓ Created draft v%d\n", rev.Version)
	}
	ui.InfoLn("  Run `conure deploy` to apply.")
	return nil
}

func runRevisionShow(cmd *cobra.Command, args []string) error {
	lc, err := resolveTarget(cmd)
	if err != nil {
		return err
	}

	rev, err := pickRevision(cmd, lc, args)
	if err != nil {
		return err
	}

	valuesOnly, _ := cmd.Flags().GetBool("values-only")

	// Pick the payload up front so JSON/YAML and text branches stay in sync.
	var payload any = rev
	if valuesOnly {
		payload = rev.Values
	}

	return ui.Render(payload, func() error {
		// In text mode the JSON form is the most useful default — values
		// are arbitrary nested maps and a flat table would lose structure.
		// The user picked text by not passing -o; print pretty JSON
		// preceded by a one-line header so it's still scannable.
		deployedAt := "-"
		if rev.DeployedAt != nil {
			deployedAt = rev.DeployedAt.Format("2006-01-02 15:04:05")
		}
		ui.HeaderLn(fmt.Sprintf("v%d  [%s]  %s  (%s)", rev.Version, rev.Status, deployedAt, rev.ID))
		body, err := json.MarshalIndent(payload, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(body))
		return nil
	})
}

// pickRevision resolves which revision the user wants to print. With a
// positional version arg we look it up in the list; without one we fall
// back to the env-scoped view's latest draft (or, lacking that, the
// deployed revision). Returns an actionable error when nothing matches.
func pickRevision(cmd *cobra.Command, lc *linkedCtx, args []string) (*api.ComponentRevision, error) {
	if len(args) == 1 {
		versionArg := strings.TrimPrefix(args[0], "v")
		version, err := strconv.Atoi(versionArg)
		if err != nil {
			return nil, fmt.Errorf("invalid version %q (expected number, optionally prefixed with 'v')", args[0])
		}
		revs, err := lc.Client.ListRevisions(cmd.Context(), lc.Link.OrgID, lc.Link.AppID, lc.Env, lc.Link.ComponentID)
		if err != nil {
			return nil, err
		}
		for i, r := range revs {
			if r.Version == version {
				return &revs[i], nil
			}
		}
		return nil, fmt.Errorf("no revision v%d found in env %s", version, lc.Env)
	}

	resp, err := lc.Client.GetComponentInEnv(cmd.Context(), lc.Link.OrgID, lc.Link.AppID, lc.Env, lc.Link.ComponentID)
	if err != nil {
		return nil, err
	}
	if resp.LatestDraft != nil {
		return resp.LatestDraft, nil
	}
	if resp.DeployedRevision != nil {
		return resp.DeployedRevision, nil
	}
	return nil, fmt.Errorf("no revisions yet for component %s in env %s — pass a version", lc.Link.ComponentName, lc.Env)
}

func runRevisionCreate(cmd *cobra.Command, _ []string) error {
	lc, err := resolveTarget(cmd)
	if err != nil {
		return err
	}
	path, _ := cmd.Flags().GetString("file")
	formatOverride, _ := cmd.Flags().GetString("format")
	comment, _ := cmd.Flags().GetString("comment")

	values, err := loadValuesFile(path, formatOverride)
	if err != nil {
		return err
	}

	sp := ui.StartSpinner("Creating draft…")
	rev, err := lc.Client.CreateRevision(cmd.Context(), lc.Link.OrgID, lc.Link.AppID, lc.Env, lc.Link.ComponentID, values, comment)
	ui.StopSpinner(sp)
	if err != nil {
		return err
	}
	return ui.Render(rev, func() error {
		ui.Success("✓ Created draft v%d\n", rev.Version)
		ui.InfoLn("  Run `conure deploy` to apply.")
		return nil
	})
}

// loadValuesFile reads JSON or YAML from path (or stdin when path is "-")
// and returns a values map. Format is decided by --format if set, else by
// file extension, else YAML (a strict superset of JSON for sigs.k8s.io/yaml,
// so JSON-on-stdin still parses correctly).
func loadValuesFile(path, formatOverride string) (map[string]interface{}, error) {
	var raw []byte
	var err error
	if path == "-" {
		raw, err = io.ReadAll(os.Stdin)
	} else {
		raw, err = os.ReadFile(path)
	}
	if err != nil {
		return nil, fmt.Errorf("reading values: %w", err)
	}

	format := strings.ToLower(strings.TrimSpace(formatOverride))
	if format == "" {
		ext := strings.ToLower(filepath.Ext(path))
		switch ext {
		case ".json":
			format = "json"
		case ".yaml", ".yml":
			format = "yaml"
		default:
			format = "yaml"
		}
	}

	var values map[string]interface{}
	switch format {
	case "json":
		if err := json.Unmarshal(raw, &values); err != nil {
			return nil, fmt.Errorf("parsing JSON values: %w", err)
		}
	case "yaml", "yml":
		// sigs.k8s.io/yaml goes via JSON, so it also accepts pure-JSON
		// input. That's why "yaml" is a safe default when format is
		// ambiguous (e.g. stdin with no extension).
		if err := yaml.Unmarshal(raw, &values); err != nil {
			return nil, fmt.Errorf("parsing YAML values: %w", err)
		}
	default:
		return nil, fmt.Errorf("unknown --format %q (expected: json or yaml)", format)
	}
	if values == nil {
		return nil, fmt.Errorf("values file is empty or not an object")
	}
	return values, nil
}
