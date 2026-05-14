package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/coffeenights/conure/internal/cli/ui"
)

// accountCmd is the self-service surface for the currently logged-in user.
// Every authenticated user can use it — admins manage other users via
// `conure users`.
var accountCmd = &cobra.Command{
	Use:   "account",
	Short: "Manage your own account (email, password)",
}

var accountShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show the logged-in user's account",
	RunE:  runAccountShow,
}

var setEmailNewEmail string

var accountSetEmailCmd = &cobra.Command{
	Use:   "set-email",
	Short: "Change the logged-in user's email",
	RunE:  runAccountSetEmail,
}

var accountPasswordCmd = &cobra.Command{
	Use:   "password",
	Short: "Change the logged-in user's password",
	RunE:  runAccountPassword,
}

func init() {
	accountSetEmailCmd.Flags().StringVar(&setEmailNewEmail, "email", "", "New email (prompted if empty)")
	accountCmd.AddCommand(accountShowCmd, accountSetEmailCmd, accountPasswordCmd)
	rootCmd.AddCommand(accountCmd)
}

func runAccountShow(cmd *cobra.Command, _ []string) error {
	_, _, client, err := requireAuthClient()
	if err != nil {
		return err
	}
	u, err := client.GetMe(cmd.Context())
	if err != nil {
		return err
	}
	return ui.Render(u, func() error {
		printUser(u)
		return nil
	})
}

func runAccountSetEmail(cmd *cobra.Command, _ []string) error {
	_, _, client, err := requireAuthClient()
	if err != nil {
		return err
	}
	email := setEmailNewEmail
	if email == "" {
		fmt.Print("New email: ")
		var input string
		fmt.Scanln(&input)
		email = input
	}
	if email == "" {
		return fmt.Errorf("email cannot be empty")
	}
	u, err := client.UpdateMe(cmd.Context(), email)
	if err != nil {
		return err
	}
	ui.Success("✓ Email updated to %s\n", u.Email)
	return nil
}

func runAccountPassword(cmd *cobra.Command, _ []string) error {
	_, _, client, err := requireAuthClient()
	if err != nil {
		return err
	}
	fmt.Print("Current password: ")
	oldBytes, err := term.ReadPassword(int(os.Stdin.Fd()))
	if err != nil {
		return fmt.Errorf("reading current password: %w", err)
	}
	fmt.Println()
	fmt.Print("New password: ")
	newBytes, err := term.ReadPassword(int(os.Stdin.Fd()))
	if err != nil {
		return fmt.Errorf("reading new password: %w", err)
	}
	fmt.Println()
	fmt.Print("Confirm new password: ")
	confirmBytes, err := term.ReadPassword(int(os.Stdin.Fd()))
	if err != nil {
		return fmt.Errorf("reading confirmation: %w", err)
	}
	fmt.Println()
	if string(newBytes) != string(confirmBytes) {
		return fmt.Errorf("new passwords do not match")
	}
	if err := client.ChangePassword(cmd.Context(), string(oldBytes), string(newBytes)); err != nil {
		return err
	}
	ui.SuccessLn("✓ Password changed")
	return nil
}
