package main

import (
	"github.com/spf13/cobra"

	"github.com/coffeenights/conure/internal/cli/apiclient"
	"github.com/coffeenights/conure/internal/cli/config"
	"github.com/coffeenights/conure/internal/cli/link"
	"github.com/coffeenights/conure/internal/cli/ui"
)

var (
	serverFlag string
	outputFlag string
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
	rootCmd.PersistentFlags().StringVar(&serverFlag, "server", "", "API server URL (overrides config)")
	rootCmd.PersistentFlags().StringVarP(&outputFlag, "output", "o", "text", "Output format (text, json)")
}

// addEnvFlag standardizes the --env flag used by every linked-component
// command. Centralized so the help text stays in sync.
func addEnvFlag(cmd *cobra.Command) {
	cmd.Flags().String("env", "", "Environment (overrides link)")
}

// linkedCtx bundles the four things every "act on the linked component"
// command needs: config, link, resolved env, and a ready-to-go API client.
type linkedCtx struct {
	Cfg    *config.Config
	Link   *link.Link
	Env    string
	Client *apiclient.Client
}

// requireLinked is the canonical preamble for linked-component commands.
// It enforces auth, loads the link file, applies the --env override (if
// the cobra command declared the flag), and returns a ready client.
func requireLinked(cmd *cobra.Command) (*linkedCtx, error) {
	cfg, err := config.RequireAuth(serverFlag)
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
		Cfg:    cfg,
		Link:   l,
		Env:    env,
		Client: apiclient.New(cfg.Server, cfg.Token),
	}, nil
}

// requireAuthClient is the lighter sibling of requireLinked: auth only,
// no link file. Used by org/app/component commands that operate at org
// scope.
func requireAuthClient() (*config.Config, *apiclient.Client, error) {
	cfg, err := config.RequireAuth(serverFlag)
	if err != nil {
		return nil, nil, err
	}
	return cfg, apiclient.New(cfg.Server, cfg.Token), nil
}

// requireActiveOrgClient enforces auth + active org and returns a client.
func requireActiveOrgClient() (*config.Config, *apiclient.Client, error) {
	cfg, err := config.RequireActiveOrg(serverFlag)
	if err != nil {
		return nil, nil, err
	}
	return cfg, apiclient.New(cfg.Server, cfg.Token), nil
}
