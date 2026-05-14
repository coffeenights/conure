package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/coffeenights/conure/internal/cli/ui"
	"github.com/coffeenights/conure/pkg/api"
)

// usersCmd hosts the admin-only user management subcommands. Regular users
// edit their own profile via `conure account` — that path is open to every
// authenticated user, while everything under `conure users` is gated behind
// the admin role on the server side.
var usersCmd = &cobra.Command{
	Use:   "users",
	Short: "Manage users (admin only)",
}

var (
	createUserEmail string
	createUserPwd   string
	createUserRole  string
	createUserOrg   string
)

var usersListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all users",
	RunE:  runUsersList,
}

var usersGetCmd = &cobra.Command{
	Use:   "get <user-id>",
	Short: "Show a single user",
	Args:  cobra.ExactArgs(1),
	RunE:  runUsersGet,
}

var usersCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a user",
	RunE:  runUsersCreate,
}

var (
	updateUserEmail    string
	updateUserRole     string
	updateUserOrg      string
	updateUserActive   bool
	updateUserInactive bool
)

var usersUpdateCmd = &cobra.Command{
	Use:   "update <user-id>",
	Short: "Update a user's email, role, organization, or active flag",
	Args:  cobra.ExactArgs(1),
	RunE:  runUsersUpdate,
}

var usersDeleteCmd = &cobra.Command{
	Use:   "delete <user-id>",
	Short: "Delete a user",
	Args:  cobra.ExactArgs(1),
	RunE:  runUsersDelete,
}

var resetPwdPassword string

var usersResetPwdCmd = &cobra.Command{
	Use:   "reset-password <user-id>",
	Short: "Reset a user's password (random if --password not supplied)",
	Args:  cobra.ExactArgs(1),
	RunE:  runUsersResetPassword,
}

func init() {
	usersCreateCmd.Flags().StringVar(&createUserEmail, "email", "", "Email address (required)")
	usersCreateCmd.Flags().StringVar(&createUserPwd, "password", "", "Initial password (prompted if empty)")
	usersCreateCmd.Flags().StringVar(&createUserRole, "role", "developer", "Role: admin or developer")
	usersCreateCmd.Flags().StringVar(&createUserOrg, "org", "", "Organization ID to assign (optional)")
	_ = usersCreateCmd.MarkFlagRequired("email")

	usersUpdateCmd.Flags().StringVar(&updateUserEmail, "email", "", "New email")
	usersUpdateCmd.Flags().StringVar(&updateUserRole, "role", "", "New role: admin or developer")
	usersUpdateCmd.Flags().StringVar(&updateUserOrg, "org", "", "New organization ID (use 'none' to clear)")
	usersUpdateCmd.Flags().BoolVar(&updateUserActive, "activate", false, "Mark user active")
	usersUpdateCmd.Flags().BoolVar(&updateUserInactive, "deactivate", false, "Mark user inactive")

	usersResetPwdCmd.Flags().StringVar(&resetPwdPassword, "password", "", "Specific password (random if empty)")

	usersCmd.AddCommand(usersListCmd, usersGetCmd, usersCreateCmd, usersUpdateCmd, usersDeleteCmd, usersResetPwdCmd)
	rootCmd.AddCommand(usersCmd)
}

func runUsersList(cmd *cobra.Command, _ []string) error {
	_, _, client, err := requireAuthClient()
	if err != nil {
		return err
	}
	users, err := client.ListUsers(cmd.Context())
	if err != nil {
		return err
	}
	// JSON/YAML outputs keep raw IDs so they remain machine-friendly; the
	// name-resolution only affects the human table.
	orgNames := orgNameLookup(cmd, client)
	return ui.Render(users, func() error {
		if len(users) == 0 {
			ui.InfoLn("No users found")
			return nil
		}
		rows := make([][]string, len(users))
		for i, u := range users {
			active := "yes"
			if !u.IsActive {
				active = "no"
			}
			rows[i] = []string{u.ID, u.Email, u.Role, displayOrgName(u.OrganizationID, orgNames), active}
		}
		ui.RenderTable([]string{"ID", "EMAIL", "ROLE", "ORG", "ACTIVE"}, rows, nil)
		return nil
	})
}

// orgNameLookup tries to build an id→name map by listing the orgs visible
// to the caller. Failure is non-fatal — we fall back to showing IDs, since
// org-name resolution is a presentation nicety, not a correctness need.
func orgNameLookup(cmd *cobra.Command, client interface {
	ListOrganizations(ctx context.Context) ([]api.Organization, error)
}) map[string]string {
	orgs, err := client.ListOrganizations(cmd.Context())
	if err != nil {
		return nil
	}
	names := make(map[string]string, len(orgs))
	for _, o := range orgs {
		names[o.ID] = o.Name
	}
	return names
}

func runUsersGet(cmd *cobra.Command, args []string) error {
	_, _, client, err := requireAuthClient()
	if err != nil {
		return err
	}
	u, err := client.GetUser(cmd.Context(), args[0])
	if err != nil {
		return err
	}
	orgNames := orgNameLookup(cmd, client)
	return ui.Render(u, func() error {
		printUser(u, orgNames)
		return nil
	})
}

func runUsersCreate(cmd *cobra.Command, _ []string) error {
	_, _, client, err := requireAuthClient()
	if err != nil {
		return err
	}
	pwd := createUserPwd
	if pwd == "" {
		fmt.Print("Password: ")
		b, err := term.ReadPassword(int(os.Stdin.Fd()))
		if err != nil {
			return fmt.Errorf("reading password: %w", err)
		}
		fmt.Println()
		pwd = string(b)
	}
	req := api.CreateUserRequest{
		Email:          createUserEmail,
		Password:       pwd,
		Role:           createUserRole,
		OrganizationID: createUserOrg,
	}
	u, err := client.CreateUser(cmd.Context(), req)
	if err != nil {
		return err
	}
	ui.Success("✓ Created user %s (%s)\n", u.Email, u.ID)
	orgNames := orgNameLookup(cmd, client)
	return ui.Render(u, func() error {
		printUser(u, orgNames)
		return nil
	})
}

func runUsersUpdate(cmd *cobra.Command, args []string) error {
	_, _, client, err := requireAuthClient()
	if err != nil {
		return err
	}
	if updateUserActive && updateUserInactive {
		return fmt.Errorf("--activate and --deactivate are mutually exclusive")
	}
	req := api.UpdateUserRequest{}
	if cmd.Flags().Changed("email") {
		req.Email = &updateUserEmail
	}
	if cmd.Flags().Changed("role") {
		req.Role = &updateUserRole
	}
	if cmd.Flags().Changed("org") {
		val := updateUserOrg
		// "none" is the explicit clear sentinel; an empty string clears too
		// but accepting "none" makes the intent obvious in shell history.
		if strings.EqualFold(val, "none") {
			val = ""
		}
		req.OrganizationID = &val
	}
	if updateUserActive {
		t := true
		req.IsActive = &t
	}
	if updateUserInactive {
		f := false
		req.IsActive = &f
	}
	u, err := client.UpdateUser(cmd.Context(), args[0], req)
	if err != nil {
		return err
	}
	ui.Success("✓ Updated user %s\n", u.Email)
	orgNames := orgNameLookup(cmd, client)
	return ui.Render(u, func() error {
		printUser(u, orgNames)
		return nil
	})
}

func runUsersDelete(cmd *cobra.Command, args []string) error {
	_, _, client, err := requireAuthClient()
	if err != nil {
		return err
	}
	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("Delete user %s? [y/N]: ", args[0])
	answer, _ := reader.ReadString('\n')
	if strings.ToLower(strings.TrimSpace(answer)) != "y" {
		ui.InfoLn("Aborted")
		return nil
	}
	if err := client.DeleteUser(cmd.Context(), args[0]); err != nil {
		return err
	}
	ui.Success("✓ Deleted user %s\n", args[0])
	return nil
}

func runUsersResetPassword(cmd *cobra.Command, args []string) error {
	_, _, client, err := requireAuthClient()
	if err != nil {
		return err
	}
	pwd, err := client.ResetUserPassword(cmd.Context(), args[0], resetPwdPassword)
	if err != nil {
		return err
	}
	ui.Success("✓ Password reset for user %s\n", args[0])
	ui.Info("  New password: %s\n", pwd)
	return nil
}

func printUser(u *api.User, orgNames map[string]string) {
	ui.Plain("ID:    %s\n", u.ID)
	ui.Plain("Email: %s\n", u.Email)
	ui.Plain("Role:  %s\n", u.Role)
	ui.Plain("Org:   %s\n", displayOrgName(u.OrganizationID, orgNames))
	ui.Plain("Active: %t\n", u.IsActive)
}

// displayOrgName renders an organisation reference for humans. It collapses
// the "no organization" case (the all-zero ObjectID, which primitive.ObjectID
// serialises explicitly because it's a fixed-size array rather than a slice
// or pointer) to "-", and substitutes the org name when we have one in the
// lookup. Falls back to the raw ID when the lookup misses — that happens
// when the caller can't list orgs or the user's home org was deleted.
func displayOrgName(id string, names map[string]string) string {
	if id == "" || id == "000000000000000000000000" {
		return "-"
	}
	if name, ok := names[id]; ok {
		return name
	}
	return id
}
