package main

import (
	"github.com/spf13/cobra"

	"github.com/coffeenights/conure/internal/cli/apiclient"
	"github.com/coffeenights/conure/internal/cli/config"
	"github.com/coffeenights/conure/internal/cli/link"
	"github.com/coffeenights/conure/internal/cli/ui"
)

var (
	serverFlag  string
	outputFlag  string
	profileFlag string
)

var rootCmd = &cobra.Command{
	Use:   "conure",
	Short: "Conure CLI - manage application deployments",
	Long:  `Conure CLI is a command-line tool for interacting with the Conure platform to create and manage application deployments.`,
	// Separate usage errors from runtime errors. PersistentPreRunE runs only
	// after flag/arg parsing succeeds, so flipping SilenceUsage here keeps
	// the help dump for genuine CLI misuse (unknown command, missing flag)
	// while hiding it when RunE returns — at that point the failure is a
	// server or network problem, not a usage problem.
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		ui.SetJSONMode(outputFlag == "json")
		return nil
	},
}

func init() {
	rootCmd.PersistentFlags().StringVar(&serverFlag, "server", "", "API server URL (overrides the active profile's server for this command only)")
	rootCmd.PersistentFlags().StringVar(&profileFlag, "profile", "", "Profile to use for this command (overrides the active profile)")
	rootCmd.PersistentFlags().StringVarP(&outputFlag, "output", "o", "text", "Output format (text, json)")
}

// addEnvFlag standardizes the --env flag used by every linked-component
// command. Centralized so the help text stays in sync.
func addEnvFlag(cmd *cobra.Command) {
	cmd.Flags().String("env", "", "Environment (overrides link)")
}

// linkedCtx bundles the things every "act on the linked component"
// command needs. Config is included so commands that mutate (e.g. saving
// a changed ActiveOrg) can persist via config.Save.
type linkedCtx struct {
	Config  *config.Config
	Profile *config.Profile
	Link    *link.Link
	Env     string
	Client  *apiclient.Client
}

// requireLinked is the canonical preamble for linked-component commands:
// auth + link + env-flag resolution + ready API client.
func requireLinked(cmd *cobra.Command) (*linkedCtx, error) {
	cfg, prof, err := config.RequireAuth(serverFlag, profileFlag)
	if err != nil {
		return nil, err
	}
	l, err := link.Load()
	if err != nil {
		return nil, err
	}
	env := l.Environment
	if cmd != nil && cmd.Flags().Lookup("env") != nil {
		if v, _ := cmd.Flags().GetString("env"); v != "" {
			env = v
		}
	}
	return &linkedCtx{
		Config:  cfg,
		Profile: prof,
		Link:    l,
		Env:     env,
		Client:  apiclient.New(prof.Server, prof.Token),
	}, nil
}

// requireAuthClient is the lighter sibling of requireLinked: auth only,
// no link file. Used by org/app/component commands that operate at org
// scope.
func requireAuthClient() (*config.Config, *config.Profile, *apiclient.Client, error) {
	cfg, prof, err := config.RequireAuth(serverFlag, profileFlag)
	if err != nil {
		return nil, nil, nil, err
	}
	return cfg, prof, apiclient.New(prof.Server, prof.Token), nil
}

// requireActiveOrgClient enforces auth + active org and returns a client.
func requireActiveOrgClient() (*config.Config, *config.Profile, *apiclient.Client, error) {
	cfg, prof, err := config.RequireActiveOrg(serverFlag, profileFlag)
	if err != nil {
		return nil, nil, nil, err
	}
	return cfg, prof, apiclient.New(prof.Server, prof.Token), nil
}
