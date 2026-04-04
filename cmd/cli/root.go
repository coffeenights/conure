package main

import (
	"github.com/fatih/color"
	"github.com/spf13/cobra"
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

var rootCmd = &cobra.Command{
	Use:   "conure",
	Short: "Conure CLI - manage application deployments",
	Long:  `Conure CLI is a command-line tool for interacting with the Conure platform to create and manage application deployments.`,
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
