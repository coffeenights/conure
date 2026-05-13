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

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate to a Conure server",
	RunE:  runLogin,
}

var logoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Clear stored credentials",
	RunE:  runLogout,
}

func init() {
	rootCmd.AddCommand(loginCmd)
	rootCmd.AddCommand(logoutCmd)
}

func runLogin(cmd *cobra.Command, _ []string) error {
	reader := bufio.NewReader(os.Stdin)

	server := serverFlag
	if server == "" {
		if cfg, err := config.Load(); err == nil {
			server = cfg.Server
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

	if err := config.Save(&config.Config{Server: server, Token: token}); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}
	ui.Success("✓ Logged in to %s\n", server)
	return nil
}

func runLogout(_ *cobra.Command, _ []string) error {
	if err := config.Save(&config.Config{}); err != nil {
		return fmt.Errorf("clearing config: %w", err)
	}
	ui.SuccessLn("✓ Logged out")
	return nil
}
