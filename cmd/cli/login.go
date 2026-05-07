package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"
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

func runLogin(cmd *cobra.Command, args []string) error {
	reader := bufio.NewReader(os.Stdin)

	server := getServerURL()
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
	password := string(passwordBytes)

	payload, _ := json.Marshal(map[string]string{
		"email":    email,
		"password": password,
	})

	sp := startSpinner("Authenticating…")
	resp, err := http.Post(server+"/auth/login", "application/json", bytes.NewReader(payload))
	stopSpinner(sp)
	if err != nil {
		errC.Println("✗ Login failed")
		return fmt.Errorf("connecting to server: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		errC.Println("✗ Login failed")
		fmt.Printf("  Server responded with HTTP %d: %s\n", resp.StatusCode, string(body))
		return fmt.Errorf("authentication failed")
	}

	var result struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("parsing response: %w", err)
	}

	if err := saveConfig(&Config{Server: server, Token: result.Token}); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}

	success.Printf("✓ Logged in to %s\n", server)
	return nil
}

func runLogout(cmd *cobra.Command, args []string) error {
	if err := saveConfig(&Config{}); err != nil {
		return fmt.Errorf("clearing config: %w", err)
	}
	success.Println("✓ Logged out")
	return nil
}
