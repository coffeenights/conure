package main

import (
	"fmt"
	"os"
	"sort"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"sigs.k8s.io/yaml"

	conurev1alpha1 "github.com/coffeenights/conure/apis/core/v1alpha1"
	"github.com/coffeenights/conure/internal/cli/ui"
	"github.com/coffeenights/conure/pkg/api"
)

// Admin tooling for the org-scoped component definitions. Definitions are
// MongoDB-backed: the platform ships defaults that every org inherits
// (source "default"), and an org can override or hide one per (type, engine)
// (source "organization"). These commands drive the same REST endpoints the
// UI uses, scoped to the org `resolveOrgScope` resolves (--org id-or-name →
// link → active org → picker), so an operator can manage an org's effective
// catalog from any directory.

var (
	cdSetFile           string
	cdSetEngine         string
	cdSetDescription    string
	cdSetOCIRepository  string
	cdSetOCITag         string
	cdSetOCIDigest      string
	cdSetOCIRegistry    string
	cdSetRegistrySecret string
	cdSetBuildable      bool
	cdSetIconURL        string
	cdHideEngine        string
)

var componentDefCmd = &cobra.Command{
	Use:     "component-definition",
	Aliases: []string{"compdef", "cd"},
	Short:   "Manage an organization's component definitions (admin)",
	Long: `Manage the component definitions an organization can use.

Definitions are org-scoped with MongoDB as the source of truth. The platform
ships defaults that every org inherits (source "default"). An org can layer
its own definition over a (type, engine) to override it, or hide it with a
reversible tombstone — both show as source "organization".

Org targeting follows the standard precedence: --org (id or name) → directory
link → active org → interactive picker.`,
}

var componentDefListCmd = &cobra.Command{
	Use:   "list",
	Short: "List the effective component definitions for the org",
	Long: `List the org's effective catalog: shipped defaults overlaid with the
org's own overrides and tombstones — exactly what the deploy path
materializes into the cluster. The SOURCE column shows "default" (inherited,
read-only here) vs "organization" (created or overridden by this org).`,
	RunE: runComponentDefList,
}

var componentDefSetCmd = &cobra.Command{
	Use:   "set <type>",
	Short: "Create or override a definition for a (type, engine) in the org",
	Long: `Create or override the org's definition for a component type.

Posting for a (type, engine) the org already owns updates it in place (and
un-hides a tombstone); otherwise it inserts a new org-owned override that
shadows the shipped default for THIS org only. Shipped defaults are never
mutated.

Provide fields with flags, or pass a ComponentDefinition YAML with
--from-file (the same CRD shape the Helm chart ships and
seeddefaultcomponentdefinitions consumes), so a hand-authored default file
and an org override are interchangeable source material. With --from-file the
type argument is optional and, if given, must match the file.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runComponentDefSet,
}

var componentDefHideCmd = &cobra.Command{
	Use:   "hide <type>",
	Short: "Hide an inherited definition for the org (reversible tombstone)",
	Long: `Write a tombstone for a (type, engine) so this org stops inheriting the
shipped default. Reversible: 'component-definition delete <id>' on the
resulting row restores the default. Hiding an existing org override replaces
it with a tombstone.`,
	Args: cobra.ExactArgs(1),
	RunE: runComponentDefHide,
}

var componentDefDeleteCmd = &cobra.Command{
	Use:     "delete <id>",
	Aliases: []string{"rm", "unhide"},
	Short:   "Delete an org-owned definition row (override or tombstone)",
	Long: `Delete an org-owned row by id (from the ID column of 'list'). Deleting an
override restores the inherited default; deleting a tombstone un-hides it
(hence the 'unhide' alias). Shipped defaults (source "default") are shared
and cannot be deleted — hide them instead.`,
	Args: cobra.ExactArgs(1),
	RunE: runComponentDefDelete,
}

func init() {
	componentDefSetCmd.Flags().StringVarP(&cdSetFile, "from-file", "f", "", "ComponentDefinition YAML file (CRD shape; same as the Helm seed files)")
	componentDefSetCmd.Flags().StringVar(&cdSetEngine, "engine", "", "Rendering engine (timoni or helm; defaults to timoni)")
	componentDefSetCmd.Flags().StringVar(&cdSetDescription, "description", "", "Human-readable description")
	componentDefSetCmd.Flags().StringVar(&cdSetOCIRepository, "oci-repository", "", "OCI repository for the module/chart")
	componentDefSetCmd.Flags().StringVar(&cdSetOCITag, "oci-tag", "", "OCI tag")
	componentDefSetCmd.Flags().StringVar(&cdSetOCIDigest, "oci-digest", "", "OCI digest (pins over tag)")
	componentDefSetCmd.Flags().StringVar(&cdSetOCIRegistry, "oci-registry", "", "OCI registry host")
	componentDefSetCmd.Flags().StringVar(&cdSetRegistrySecret, "registry-secret", "", "Name of a dockerconfigjson Secret in the controller namespace")
	componentDefSetCmd.Flags().BoolVar(&cdSetBuildable, "buildable", false, "Whether conure can build this component's image")
	componentDefSetCmd.Flags().StringVar(&cdSetIconURL, "icon-url", "", "Icon URL surfaced in the UI")

	componentDefHideCmd.Flags().StringVar(&cdHideEngine, "engine", "", "Engine of the definition to hide (defaults to timoni)")

	componentDefDeleteCmd.Flags().Bool("approve", false, "Skip the confirmation prompt")

	componentDefCmd.AddCommand(componentDefListCmd)
	componentDefCmd.AddCommand(componentDefSetCmd)
	componentDefCmd.AddCommand(componentDefHideCmd)
	componentDefCmd.AddCommand(componentDefDeleteCmd)
	rootCmd.AddCommand(componentDefCmd)
}

func runComponentDefList(cmd *cobra.Command, _ []string) error {
	orgID, client, err := resolveOrgScope(cmd)
	if err != nil {
		return err
	}
	defs, err := client.ListComponentDefinitions(cmd.Context(), orgID)
	if err != nil {
		return err
	}
	sort.Slice(defs, func(i, j int) bool {
		if defs[i].Type != defs[j].Type {
			return defs[i].Type < defs[j].Type
		}
		return defs[i].Engine < defs[j].Engine
	})
	return ui.Render(defs, func() error {
		if len(defs) == 0 {
			ui.InfoLn("No component definitions for this org")
			return nil
		}
		rows := make([][]string, len(defs))
		for i, d := range defs {
			engine := d.Engine
			if engine == "" {
				engine = "timoni"
			}
			repo := d.OCIRepository
			if d.OCITag != "" {
				repo = fmt.Sprintf("%s:%s", repo, d.OCITag)
			}
			rows[i] = []string{d.Type, engine, d.Source, repo, d.ID}
		}
		org := lipgloss.NewStyle().Padding(0, 1).Foreground(lipgloss.Color("42"))
		ui.RenderTable(
			[]string{"TYPE", "ENGINE", "SOURCE", "OCI", "ID"},
			rows,
			func(row, col int) *lipgloss.Style {
				if col == 2 && row >= 0 && defs[row].Source == "organization" {
					return &org
				}
				return nil
			},
		)
		return nil
	})
}

func runComponentDefSet(cmd *cobra.Command, args []string) error {
	orgID, client, err := resolveOrgScope(cmd)
	if err != nil {
		return err
	}

	var req api.ComponentDefinitionRequest
	if cdSetFile != "" {
		req, err = componentDefRequestFromFile(cdSetFile)
		if err != nil {
			return err
		}
		if len(args) == 1 && args[0] != req.Type {
			return fmt.Errorf("type argument %q does not match type %q in %s", args[0], req.Type, cdSetFile)
		}
	} else {
		if len(args) != 1 {
			return fmt.Errorf("a <type> argument is required unless --from-file is given")
		}
		req = api.ComponentDefinitionRequest{
			Type:               args[0],
			Engine:             cdSetEngine,
			Description:        cdSetDescription,
			OCIRepository:      cdSetOCIRepository,
			OCITag:             cdSetOCITag,
			OCIDigest:          cdSetOCIDigest,
			OCIRegistry:        cdSetOCIRegistry,
			RegistrySecretName: cdSetRegistrySecret,
			Buildable:          cdSetBuildable,
		}
		if cdSetIconURL != "" {
			req.IconURL = &cdSetIconURL
		}
	}

	def, err := client.SetComponentDefinition(cmd.Context(), orgID, req)
	if err != nil {
		return err
	}
	engine := def.Engine
	if engine == "" {
		engine = "timoni"
	}
	ui.Success("✓ Set %s/%s for org %s (id %s)\n", def.Type, engine, orgID, def.ID)
	return ui.Render(def, nil)
}

func runComponentDefHide(cmd *cobra.Command, args []string) error {
	orgID, client, err := resolveOrgScope(cmd)
	if err != nil {
		return err
	}
	def, err := client.HideComponentDefinition(cmd.Context(), orgID, api.HideComponentDefinitionRequest{
		Type:   args[0],
		Engine: cdHideEngine,
	})
	if err != nil {
		return err
	}
	engine := cdHideEngine
	if engine == "" {
		engine = "timoni"
	}
	ui.Success("✓ Hid %s/%s for org %s — restore with `conure component-definition delete %s`\n",
		args[0], engine, orgID, def.ID)
	return nil
}

func runComponentDefDelete(cmd *cobra.Command, args []string) error {
	orgID, client, err := resolveOrgScope(cmd)
	if err != nil {
		return err
	}
	defID := args[0]
	approve, _ := cmd.Flags().GetBool("approve")
	if !approve {
		ui.Error("This deletes org-owned definition row `%s`. An override will revert to the shipped default; a tombstone will un-hide it.\n", defID)
		var ok bool
		if err := huh.NewConfirm().
			Title(fmt.Sprintf("Delete component-definition row %s in org %s?", defID, orgID)).
			Affirmative("Delete").
			Negative("Cancel").
			Value(&ok).
			Run(); err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("aborted")
		}
	}
	if err := client.DeleteComponentDefinition(cmd.Context(), orgID, defID); err != nil {
		return err
	}
	ui.Success("✓ Deleted %s\n", defID)
	return nil
}

// componentDefRequestFromFile parses a single ComponentDefinition document in
// the cluster-CRD authoring shape (the same shape the Helm chart ships and
// `seeddefaultcomponentdefinitions` consumes) and projects it onto the API
// request body, so a hand-authored default file and an org override are
// interchangeable source material.
func componentDefRequestFromFile(path string) (api.ComponentDefinitionRequest, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return api.ComponentDefinitionRequest{}, fmt.Errorf("reading %s: %w", path, err)
	}
	var crd conurev1alpha1.ComponentDefinition
	if err := yaml.Unmarshal(raw, &crd); err != nil {
		return api.ComponentDefinitionRequest{}, fmt.Errorf("parsing %s: %w", path, err)
	}
	if crd.Kind != "" && crd.Kind != "ComponentDefinition" {
		return api.ComponentDefinitionRequest{}, fmt.Errorf("%s is kind %q, not ComponentDefinition", path, crd.Kind)
	}
	if crd.Spec.ComponentType == "" {
		return api.ComponentDefinitionRequest{}, fmt.Errorf("%s has no spec.type", path)
	}
	req := api.ComponentDefinitionRequest{
		Type:          crd.Spec.ComponentType,
		Description:   crd.Spec.Description,
		Engine:        string(crd.Spec.Engine),
		OCIRepository: crd.Spec.OCIRepository,
		OCITag:        crd.Spec.OCITag,
		OCIDigest:     crd.Spec.OCIDigest,
		OCIRegistry:   crd.Spec.OCIRegistry,
		Buildable:     crd.Spec.Buildable,
		FieldRoles:    crd.Spec.FieldRoles,
	}
	if crd.Spec.RegistrySecretRef != nil {
		req.RegistrySecretName = crd.Spec.RegistrySecretRef.Name
	}
	return req, nil
}
