package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/coffeenights/conure/internal/cli/apiclient"
	"github.com/coffeenights/conure/internal/cli/config"
	"github.com/coffeenights/conure/internal/cli/ui"
)

var (
	logoutAll bool
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate to a Conure server",
	Long: `Authenticate to a Conure server and store the token as a named profile.

You can keep credentials for multiple servers side by side. Re-running
login against a server you already have a profile for replaces that
profile's token in place.`,
	RunE: runLogin,
}

var logoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Clear stored credentials for the active profile",
	Long: `Clear the active profile so it can no longer authenticate. Other
profiles remain in place. Use --all to wipe every profile from the file.`,
	RunE: runLogout,
}

func init() {
	logoutCmd.Flags().BoolVar(&logoutAll, "all", false, "Remove every profile, not just the active one")
	rootCmd.AddCommand(loginCmd)
	rootCmd.AddCommand(logoutCmd)
}

func runLogin(cmd *cobra.Command, _ []string) error {
	reader := bufio.NewReader(os.Stdin)

	// Resolve a starting server hint: --server flag wins, otherwise the
	// active profile's server (so re-logging-in defaults to the current
	// server), otherwise prompt.
	server := serverFlag
	cfg, _ := config.Load()
	if cfg == nil {
		cfg = &config.Config{Profiles: map[string]*config.Profile{}}
	}
	if server == "" {
		if active := cfg.GetActive(); active != nil {
			server = active.Server
		}
	}
	if server == "" {
		fmt.Print("Server URL: ")
		input, _ := reader.ReadString('\n')
		server = strings.TrimSpace(input)
	}
	server = strings.TrimRight(server, "/")

	fmt.Print("Email: ")
	email, _ := reader.ReadString('\n')
	email = strings.TrimSpace(email)

	fmt.Print("Password: ")
	passwordBytes, err := term.ReadPassword(int(os.Stdin.Fd()))
	if err != nil {
		return fmt.Errorf("reading password: %w", err)
	}
	fmt.Println()

	sp := ui.StartSpinner("Authenticating…")
	token, err := apiclient.Login(cmd.Context(), server, email, string(passwordBytes))
	ui.StopSpinner(sp)
	if err != nil {
		return err
	}

	// Profile naming: if there's already a profile for this server, reuse
	// its name (overwrite the token); otherwise prompt with a hostname
	// default. --profile=<name> on the command line skips the prompt.
	name := profileFlag
	if name == "" {
		if existingName, _ := cfg.FindByServer(server); existingName != "" {
			name = existingName
		} else {
			name = config.DefaultProfileName(server)
			fmt.Printf("Profile name [%s]: ", name)
			input, _ := reader.ReadString('\n')
			if t := strings.TrimSpace(input); t != "" {
				name = t
			}
		}
	}

	// Preserve any existing ActiveOrg on overwrite so the user doesn't
	// lose their org selection on a re-login.
	var keepOrg string
	if prev := cfg.Get(name); prev != nil {
		keepOrg = prev.ActiveOrg
	}
	cfg.Upsert(name, &config.Profile{
		Server:    server,
		Token:     token,
		ActiveOrg: keepOrg,
	})
	cfg.Active = name
	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}
	ui.Success("✓ Logged in to %s as profile `%s`\n", server, name)
	return nil
}

func runLogout(_ *cobra.Command, _ []string) error {
	if logoutAll {
		if err := config.Save(&config.Config{Profiles: map[string]*config.Profile{}}); err != nil {
			return fmt.Errorf("clearing config: %w", err)
		}
		ui.SuccessLn("✓ Cleared all profiles")
		return nil
	}

	cfg, err := config.Load()
	if err != nil {
		// File doesn't exist — already logged out.
		ui.InfoLn("Already logged out")
		return nil
	}
	if cfg.Active == "" {
		ui.InfoLn("No active profile to log out of")
		return nil
	}
	name := cfg.Active
	if err := cfg.Remove(name); err != nil {
		return err
	}
	if err := config.Save(cfg); err != nil {
		return err
	}
	ui.Success("✓ Logged out of profile `%s`\n", name)
	if len(cfg.Profiles) > 0 {
		ui.InfoLn("  Other profiles remain. Use `conure profile use <name>` to activate one.")
	}
	return nil
}
