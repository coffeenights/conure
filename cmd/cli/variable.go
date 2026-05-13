package main

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/spf13/cobra"

	"github.com/coffeenights/conure/internal/cli/apiclient"
	"github.com/coffeenights/conure/internal/cli/envfile"
	"github.com/coffeenights/conure/internal/cli/link"
	"github.com/coffeenights/conure/internal/cli/ui"
	"github.com/coffeenights/conure/pkg/api"
)

// Variable name validation mirrors models.Variable.ValidateName on the
// server. We pre-check on the client so bulk imports can fail fast with a
// useful line number instead of one round-trip per bad entry.
var varNamePattern = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

var (
	varScopeFlag     string
	varSecretFlag    bool
	varOverwriteFlag bool
	varRevealFlag    bool
)

var varCmd = &cobra.Command{
	Use:     "var",
	Aliases: []string{"vars", "variable", "variables"},
	Short:   "Manage variables and secrets",
	Long: `Manage variables and secrets across organization, environment, and component scopes.

By default, commands operate at the scope of this directory's link (a linked
component). Use --scope to widen to env or org.`,
}

var varListCmd = &cobra.Command{
	Use:   "list",
	Short: "List variables in the chosen scope",
	RunE:  runVarList,
}

var varSetCmd = &cobra.Command{
	Use:   "set NAME=VALUE [NAME=VALUE...]",
	Short: "Create one or more variables",
	Long: `Create one or more variables in the chosen scope.

Existing variables are not overwritten unless --overwrite is set; this is
intentional so a typo doesn't silently replace a known-good value.`,
	Args: cobra.MinimumNArgs(1),
	RunE: runVarSet,
}

var varDeleteCmd = &cobra.Command{
	Use:     "delete NAME",
	Aliases: []string{"rm"},
	Short:   "Delete a variable by name",
	Args:    cobra.ExactArgs(1),
	RunE:    runVarDelete,
}

var varImportCmd = &cobra.Command{
	Use:   "import FILE",
	Short: "Bulk-import variables from a .env file",
	Long: `Parse a dotenv file and create one variable per KEY=VALUE line.

Mark an entry as a secret (encrypted at rest) by placing a comment
'# @secret' on the line directly above it:

    # @secret
    DB_PASSWORD=hunter2
    API_KEY=public

Existing variables are skipped by default. Pass --overwrite to replace them.`,
	Args: cobra.ExactArgs(1),
	RunE: runVarImport,
}

func init() {
	for _, c := range []*cobra.Command{varListCmd, varSetCmd, varDeleteCmd, varImportCmd} {
		c.Flags().StringVar(&varScopeFlag, "scope", "",
			"Scope: component (default if linked), env, or org")
		addEnvFlag(c)
	}
	varSetCmd.Flags().BoolVar(&varSecretFlag, "secret", false,
		"Mark variable(s) as encrypted at rest")
	varSetCmd.Flags().BoolVar(&varOverwriteFlag, "overwrite", false,
		"Replace existing variables with the same name")
	varImportCmd.Flags().BoolVar(&varOverwriteFlag, "overwrite", false,
		"Replace existing variables with the same name")
	varImportCmd.Flags().BoolVar(&varSecretFlag, "secret", false,
		"Mark every imported entry as a secret (regardless of '# @secret' markers)")
	varListCmd.Flags().BoolVar(&varRevealFlag, "reveal", false,
		"Print decrypted secret values instead of masking them with "+secretMask)

	varCmd.AddCommand(varListCmd)
	varCmd.AddCommand(varSetCmd)
	varCmd.AddCommand(varDeleteCmd)
	varCmd.AddCommand(varImportCmd)
	rootCmd.AddCommand(varCmd)
}

// scopeCtx is the resolved target of a var command: the API client to use
// plus the variable scope (org/app/env/component IDs). Built once per
// command from --scope plus the linked directory's defaults.
type scopeCtx struct {
	Client *apiclient.Client
	Scope  apiclient.VariableScope
	// Label is a short human-readable description ("component <name>")
	// used in success/skip messages.
	Label string
}

// resolveScope figures out which org/app/env/component the command should
// act on. The rules:
//
//   - scope=component (or unset with a link present) requires the full link
//   - scope=env requires org+app+env (from link, with --env override)
//   - scope=org only needs the active org (or the link's org)
//
// Errors are actionable: "run conure init" or "pass --scope".
func resolveScope(cmd *cobra.Command) (*scopeCtx, error) {
	scope := strings.ToLower(strings.TrimSpace(varScopeFlag))
	if scope == "" {
		if link.Exists() {
			scope = "component"
		} else {
			scope = "org"
		}
	}
	switch scope {
	case "component", "comp", "c":
		lc, err := requireLinked(cmd)
		if err != nil {
			return nil, err
		}
		return &scopeCtx{
			Client: lc.Client,
			Scope: apiclient.VariableScope{
				OrgID:         lc.Link.OrgID,
				ApplicationID: lc.Link.AppID,
				EnvironmentID: lc.Env,
				ComponentID:   lc.Link.ComponentID,
			},
			Label: fmt.Sprintf("component %s (env %s)", lc.Link.ComponentName, lc.Env),
		}, nil
	case "env", "environment", "e":
		lc, err := requireLinked(cmd)
		if err != nil {
			return nil, err
		}
		return &scopeCtx{
			Client: lc.Client,
			Scope: apiclient.VariableScope{
				OrgID:         lc.Link.OrgID,
				ApplicationID: lc.Link.AppID,
				EnvironmentID: lc.Env,
			},
			Label: fmt.Sprintf("environment %s", lc.Env),
		}, nil
	case "org", "organization", "o":
		// Org scope works without a link — fall back to the active org
		// if the current directory isn't linked.
		if link.Exists() {
			lc, err := requireLinked(cmd)
			if err != nil {
				return nil, err
			}
			return &scopeCtx{
				Client: lc.Client,
				Scope:  apiclient.VariableScope{OrgID: lc.Link.OrgID},
				Label:  fmt.Sprintf("organization %s", lc.Link.OrgID),
			}, nil
		}
		_, prof, client, err := requireActiveOrgClient()
		if err != nil {
			return nil, err
		}
		return &scopeCtx{
			Client: client,
			Scope:  apiclient.VariableScope{OrgID: prof.ActiveOrg},
			Label:  fmt.Sprintf("organization %s", prof.ActiveOrg),
		}, nil
	default:
		return nil, fmt.Errorf("unknown --scope %q (expected: component, env, org)", scope)
	}
}

func runVarList(cmd *cobra.Command, _ []string) error {
	sc, err := resolveScope(cmd)
	if err != nil {
		return err
	}
	vars, err := sc.Client.ListVariables(cmd.Context(), sc.Scope)
	if err != nil {
		return err
	}
	return ui.Render(vars, func() error {
		if len(vars) == 0 {
			ui.InfoLn("No variables found")
			return nil
		}
		rows := make([][]string, len(vars))
		for i, v := range vars {
			secretMark := "-"
			if v.IsEncrypted {
				secretMark = "✓"
			}
			rows[i] = []string{v.Name, displayValue(v), v.Type, secretMark}
		}
		ui.RenderTable([]string{"NAME", "VALUE", "SCOPE", "SECRET"}, rows, nil)
		if varRevealFlag {
			ui.DimLn("  (secret values revealed via --reveal)")
		}
		return nil
	})
}

// secretMask is the placeholder rendered for encrypted values in the text
// listing. JSON/YAML output still emits real values — the mask is purely
// to keep terminals from leaking secrets to over-the-shoulder readers.
const secretMask = "*****"

// displayValue masks encrypted values by default. --reveal opts out, for
// the operator who genuinely needs to read the value back at the terminal.
func displayValue(v api.Variable) string {
	if v.IsEncrypted && !varRevealFlag {
		return secretMask
	}
	return v.Value
}

func runVarSet(cmd *cobra.Command, args []string) error {
	sc, err := resolveScope(cmd)
	if err != nil {
		return err
	}
	pairs, err := parseSetArgs(args)
	if err != nil {
		return err
	}
	created, skipped, overwritten, err := upsertVariables(cmd.Context(), sc, pairs, varSecretFlag, varOverwriteFlag)
	if err != nil {
		return err
	}
	summarize(sc.Label, created, skipped, overwritten)
	return nil
}

func runVarDelete(cmd *cobra.Command, args []string) error {
	sc, err := resolveScope(cmd)
	if err != nil {
		return err
	}
	name := args[0]
	existing, err := sc.Client.ListVariables(cmd.Context(), sc.Scope)
	if err != nil {
		return err
	}
	for _, v := range existing {
		if v.Name == name {
			if err := sc.Client.DeleteVariable(cmd.Context(), sc.Scope.OrgID, v.ID); err != nil {
				return err
			}
			ui.Success("✓ Deleted %s from %s\n", name, sc.Label)
			return nil
		}
	}
	return fmt.Errorf("no variable named %q in %s", name, sc.Label)
}

func runVarImport(cmd *cobra.Command, args []string) error {
	sc, err := resolveScope(cmd)
	if err != nil {
		return err
	}
	entries, err := envfile.ParseFile(args[0])
	if err != nil {
		return fmt.Errorf("reading %s: %w", args[0], err)
	}
	if len(entries) == 0 {
		ui.InfoLn("No variables found in file")
		return nil
	}
	// Validate names up front so a bulk import doesn't half-succeed before
	// hitting a bad line. The server enforces the same regex, but failing
	// here gives the user a line number.
	for _, e := range entries {
		if !varNamePattern.MatchString(e.Name) {
			return fmt.Errorf("line %d: invalid variable name %q", e.Line, e.Name)
		}
	}
	pairs := make([]varPair, len(entries))
	for i, e := range entries {
		pairs[i] = varPair{
			Name:     e.Name,
			Value:    e.Value,
			IsSecret: e.IsSecret || varSecretFlag,
		}
	}
	created, skipped, overwritten, err := upsertVariables(cmd.Context(), sc, pairs, false, varOverwriteFlag)
	if err != nil {
		return err
	}
	summarize(sc.Label, created, skipped, overwritten)
	return nil
}

// varPair is the input shape that both `var set` and `var import` reduce
// to: a name+value plus whether it should be encrypted. Centralizing the
// upsert path means the two commands have identical create/skip/overwrite
// semantics.
type varPair struct {
	Name     string
	Value    string
	IsSecret bool
}

// parseSetArgs accepts either `KEY=VALUE` tokens or, when exactly two args
// are given, `KEY VALUE`. The dual form is convenient for shell quoting
// when the value contains `=`.
func parseSetArgs(args []string) ([]varPair, error) {
	if len(args) == 2 && !strings.Contains(args[0], "=") {
		if !varNamePattern.MatchString(args[0]) {
			return nil, fmt.Errorf("invalid variable name %q", args[0])
		}
		return []varPair{{Name: args[0], Value: args[1], IsSecret: varSecretFlag}}, nil
	}
	pairs := make([]varPair, 0, len(args))
	for _, a := range args {
		eq := strings.IndexByte(a, '=')
		if eq <= 0 {
			return nil, fmt.Errorf("expected NAME=VALUE, got %q", a)
		}
		name := strings.TrimSpace(a[:eq])
		val := a[eq+1:]
		if !varNamePattern.MatchString(name) {
			return nil, fmt.Errorf("invalid variable name %q", name)
		}
		pairs = append(pairs, varPair{Name: name, Value: val, IsSecret: varSecretFlag})
	}
	return pairs, nil
}

// upsertVariables creates each pair in scope, honoring overwrite by
// deleting the existing entry first. Returns the names that were created,
// skipped (already exist + !overwrite), and overwritten so the caller can
// print a single summary.
func upsertVariables(ctx context.Context, sc *scopeCtx, pairs []varPair, defaultSecret, overwrite bool) (created, skipped, overwritten []string, err error) {
	existing, err := sc.Client.ListVariables(ctx, sc.Scope)
	if err != nil {
		return nil, nil, nil, err
	}
	byName := make(map[string]api.Variable, len(existing))
	for _, v := range existing {
		byName[v.Name] = v
	}
	for _, p := range pairs {
		isSecret := p.IsSecret || defaultSecret
		if cur, ok := byName[p.Name]; ok {
			if !overwrite {
				skipped = append(skipped, p.Name)
				continue
			}
			if err := sc.Client.DeleteVariable(ctx, sc.Scope.OrgID, cur.ID); err != nil {
				return created, skipped, overwritten, fmt.Errorf("overwriting %s: %w", p.Name, err)
			}
			overwritten = append(overwritten, p.Name)
		}
		if _, err := sc.Client.CreateVariable(ctx, sc.Scope, p.Name, p.Value, isSecret); err != nil {
			return created, skipped, overwritten, fmt.Errorf("creating %s: %w", p.Name, err)
		}
		if _, was := byName[p.Name]; !was {
			created = append(created, p.Name)
		}
	}
	return created, skipped, overwritten, nil
}

func summarize(label string, created, skipped, overwritten []string) {
	if len(created) > 0 {
		ui.Success("✓ Created %d in %s: %s\n", len(created), label, strings.Join(created, ", "))
	}
	if len(overwritten) > 0 {
		ui.Success("✓ Overwrote %d: %s\n", len(overwritten), strings.Join(overwritten, ", "))
	}
	if len(skipped) > 0 {
		ui.Info("• Skipped %d (already exists; use --overwrite to replace): %s\n",
			len(skipped), strings.Join(skipped, ", "))
	}
	if len(created)+len(overwritten)+len(skipped) == 0 {
		ui.InfoLn("No changes")
	}
}
