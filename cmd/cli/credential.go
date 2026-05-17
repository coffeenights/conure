package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/coffeenights/conure/internal/cli/ui"
	"github.com/coffeenights/conure/pkg/api"
)

// Credentials are always org-scoped (unlike variables, which also have env
// and component tiers), so this command resolves the org the same way `var
// --scope org` does and has no --scope flag.

var (
	credKindFlag        string
	credRegistryFlag    string
	credUsernameFlag    string
	credSecretStdinFlag bool
)

var credCmd = &cobra.Command{
	Use:     "credential",
	Aliases: []string{"cred", "creds", "credentials"},
	Short:   "Manage registry and git credentials (org-scoped)",
	Long: `Manage credentials conure uses to pull private component templates,
push built images, and clone private git sources.

Credentials are stored encrypted by the API server and projected into a
Kubernetes Secret at deploy time. The secret material is never returned by
the API: 'list' shows metadata only, and you cannot read a value back.

A ComponentDefinition references a 'registry' credential by name for its
private template pull; a component references 'git'/'registry' credentials
by name in its values (the git.credentialRef / image.credentialRef field
roles) for private source clone and image push.`,
}

var credSetCmd = &cobra.Command{
	Use:   "set NAME --kind registry|git [flags]",
	Short: "Create or rotate a credential",
	Long: `Create a credential, or rotate an existing one (posting a name that
already exists replaces its material in place).

The secret (registry password / git token) is read from stdin so it never
lands in shell history:

    # interactive (prompts, input hidden):
    conure credential set ghcr --kind registry --registry ghcr.io --username me

    # piped (CI):
    echo "$TOKEN" | conure credential set ghcr --kind registry \
        --registry ghcr.io --username me --secret-stdin

Notes:
  - registry: --registry and --username are required. For ghcr.io the
    username must be the PAT owner and the token a *classic* PAT with
    write:packages (ghcr rejects fine-grained PATs).
  - git: --username defaults to x-access-token (GitHub/GitLab convention).`,
	Args: cobra.ExactArgs(1),
	RunE: runCredSet,
}

var credListCmd = &cobra.Command{
	Use:   "list",
	Short: "List credentials in the org (metadata only)",
	RunE:  runCredList,
}

var credDeleteCmd = &cobra.Command{
	Use:     "delete NAME",
	Aliases: []string{"rm"},
	Short:   "Delete a credential by name",
	Args:    cobra.ExactArgs(1),
	RunE:    runCredDelete,
}

func init() {
	credSetCmd.Flags().StringVar(&credKindFlag, "kind", "", "Credential kind: registry or git (required)")
	credSetCmd.Flags().StringVar(&credRegistryFlag, "registry", "", "Registry URL (registry kind; e.g. ghcr.io)")
	credSetCmd.Flags().StringVar(&credUsernameFlag, "username", "", "Username (registry: required; git: defaults to x-access-token)")
	credSetCmd.Flags().BoolVar(&credSecretStdinFlag, "secret-stdin", false,
		"Read the secret from stdin without prompting (for pipes/CI)")
	_ = credSetCmd.MarkFlagRequired("kind")

	credCmd.AddCommand(credSetCmd)
	credCmd.AddCommand(credListCmd)
	credCmd.AddCommand(credDeleteCmd)
	rootCmd.AddCommand(credCmd)
}

// readSecret pulls the secret material from stdin. When stdin is a TTY and
// --secret-stdin was not passed, it prompts with hidden input (like
// `conure account password`); otherwise it reads piped stdin to EOF. The
// trailing newline from `echo`/typed input is trimmed; a fully empty secret
// is rejected because an empty credential is never intentional.
func readSecret(prompt string) (string, error) {
	if !credSecretStdinFlag && term.IsTerminal(int(os.Stdin.Fd())) {
		fmt.Print(prompt)
		b, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Println()
		if err != nil {
			return "", fmt.Errorf("reading secret: %w", err)
		}
		s := strings.TrimRight(string(b), "\r\n")
		if s == "" {
			return "", fmt.Errorf("empty secret")
		}
		return s, nil
	}
	raw, err := io.ReadAll(os.Stdin)
	if err != nil {
		return "", fmt.Errorf("reading secret from stdin: %w", err)
	}
	return normalizeSecret(string(raw))
}

// normalizeSecret trims a single trailing newline (CRLF or LF, as `echo` or a
// typed line produces) and rejects an empty result. Split out from readSecret
// so the trimming/empty contract is unit-testable without an stdin fixture.
func normalizeSecret(raw string) (string, error) {
	s := strings.TrimRight(raw, "\r\n")
	if s == "" {
		return "", fmt.Errorf("empty secret")
	}
	return s, nil
}

// resolveCredInput validates the kind and normalizes the username (applying
// the git default), independent of any I/O. Pure so the flag contract is
// unit-testable. Returns the effective kind, username, and registry.
//
// reg is the registry to actually send: the flag value for a registry
// credential, but always "" for git — a git credential has no registry, and
// sending one would persist a misleading registryUrl (it shows up in
// `credential list` and survives a cross-kind rotation). Keeping the
// kind→effective-fields rule here means callers never re-derive it.
func resolveCredInput(kindFlag, registry, username string) (kind, user, reg string, err error) {
	kind = strings.ToLower(strings.TrimSpace(kindFlag))
	switch kind {
	case "registry":
		if registry == "" || username == "" {
			return "", "", "", fmt.Errorf("registry credentials require --registry and --username")
		}
		return kind, username, registry, nil
	case "git":
		if username == "" {
			username = "x-access-token"
		}
		// git has no registry; drop any --registry the user passed.
		return kind, username, "", nil
	default:
		return "", "", "", fmt.Errorf("--kind must be registry or git, got %q", kind)
	}
}

func runCredSet(cmd *cobra.Command, args []string) error {
	name := args[0]
	kind, username, registry, err := resolveCredInput(credKindFlag, credRegistryFlag, credUsernameFlag)
	if err != nil {
		return err
	}

	orgID, client, err := resolveOrgScope(cmd)
	if err != nil {
		return err
	}

	secret, err := readSecret(fmt.Sprintf("Secret for %q: ", name))
	if err != nil {
		return err
	}

	cred, err := client.SetCredential(cmd.Context(), orgID, api.CreateCredentialRequest{
		Name:        name,
		Kind:        kind,
		RegistryURL: registry,
		Username:    username,
		Secret:      secret,
	})
	if err != nil {
		return err
	}
	return ui.Render(cred, func() error {
		ui.Success("✓ Saved credential %q (%s) in org %s\n", cred.Name, cred.Kind, orgID)
		return nil
	})
}

func runCredList(cmd *cobra.Command, _ []string) error {
	orgID, client, err := resolveOrgScope(cmd)
	if err != nil {
		return err
	}
	creds, err := client.ListCredentials(cmd.Context(), orgID)
	if err != nil {
		return err
	}
	return ui.Render(creds, func() error {
		if len(creds) == 0 {
			ui.InfoLn("No credentials found")
			return nil
		}
		rows := make([][]string, len(creds))
		for i, c := range creds {
			reg := c.RegistryURL
			if reg == "" {
				reg = "-"
			}
			user := c.Username
			if user == "" {
				user = "-"
			}
			rows[i] = []string{c.Name, c.Kind, reg, user}
		}
		ui.RenderTable([]string{"NAME", "KIND", "REGISTRY", "USERNAME"}, rows, nil)
		return nil
	})
}

func runCredDelete(cmd *cobra.Command, args []string) error {
	orgID, client, err := resolveOrgScope(cmd)
	if err != nil {
		return err
	}
	name := args[0]
	if err := client.DeleteCredential(cmd.Context(), orgID, name); err != nil {
		return err
	}
	ui.Success("✓ Deleted credential %q from org %s\n", name, orgID)
	return nil
}
