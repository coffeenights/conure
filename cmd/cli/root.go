package main

import (
	"os"
	"time"

	"github.com/briandowns/spinner"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var (
	serverFlag string
	outputFlag string
)

var (
	success = color.New(color.FgGreen, color.Bold)
	errC    = color.New(color.FgRed, color.Bold)
	info    = color.New(color.FgCyan)
	header  = color.New(color.FgWhite, color.Bold)
	dim     = color.New(color.FgHiBlack)
)

// startSpinner returns a started spinner, or nil when output is non-TTY or
// json so we don't smear escape codes into pipes and CI logs. Callers should
// always defer stopSpinner(s) — it tolerates a nil receiver.
func startSpinner(suffix string) *spinner.Spinner {
	if outputFlag == "json" || !term.IsTerminal(int(os.Stdout.Fd())) {
		return nil
	}
	s := spinner.New(spinner.CharSets[14], 100*time.Millisecond)
	s.Suffix = "  " + suffix
	s.Start()
	return s
}

func stopSpinner(s *spinner.Spinner) {
	if s != nil {
		s.Stop()
	}
}

var rootCmd = &cobra.Command{
	Use:   "conure",
	Short: "Conure CLI - manage application deployments",
	Long:  `Conure CLI is a command-line tool for interacting with the Conure platform to create and manage application deployments.`,
	// Separate usage errors from runtime errors. PersistentPreRunE runs only
	// after flag/arg parsing succeeds, so flipping SilenceUsage here keeps the
	// help dump for genuine CLI misuse (unknown command, missing flag) while
	// hiding it when RunE returns — at that point the failure is a server or
	// network problem, not a usage problem.
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		return nil
	},
}

func init() {
	rootCmd.PersistentFlags().StringVar(&serverFlag, "server", "", "API server URL (overrides config)")
	rootCmd.PersistentFlags().StringVarP(&outputFlag, "output", "o", "text", "Output format (text, json)")
}

func getServerURL() string {
	if serverFlag != "" {
		return serverFlag
	}
	cfg, err := loadConfig()
	if err != nil {
		return ""
	}
	return cfg.Server
}
