package main

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
)

var logsCmd = &cobra.Command{
	Use:   "logs",
	Short: "Stream logs from pods backing this component",
	Long: `Stream logs from one or more pods backing the linked component.

By default, the last few lines from every pod are printed. Use --pod to
restrict to specific pods (a list from 'conure pods'). Use -f/--follow to
tail forever; Ctrl-C ends the stream.`,
	RunE: runLogs,
}

func init() {
	logsCmd.Flags().String("env", "", "Environment (overrides link)")
	logsCmd.Flags().StringSliceP("pod", "p", nil, "Restrict to one or more pod names (repeatable, or comma-separated)")
	logsCmd.Flags().StringP("container", "c", "", "Container name (when a pod has more than one)")
	logsCmd.Flags().BoolP("follow", "f", false, "Tail logs continuously")
	logsCmd.Flags().Int64("tail", 200, "Show the last N lines per pod (0 means no limit)")
	logsCmd.Flags().String("since", "", "Only logs newer than this duration (e.g. 5m, 1h)")
	logsCmd.Flags().Bool("previous", false, "Read the previous container instance's log")
	rootCmd.AddCommand(logsCmd)
}

func runLogs(cmd *cobra.Command, args []string) error {
	cfg, err := requireAuth()
	if err != nil {
		return err
	}
	link, err := requireLink()
	if err != nil {
		return err
	}
	env := link.Environment
	if v, _ := cmd.Flags().GetString("env"); v != "" {
		env = v
	}

	pods, _ := cmd.Flags().GetStringSlice("pod")
	container, _ := cmd.Flags().GetString("container")
	follow, _ := cmd.Flags().GetBool("follow")
	tail, _ := cmd.Flags().GetInt64("tail")
	since, _ := cmd.Flags().GetString("since")
	previous, _ := cmd.Flags().GetBool("previous")

	q := url.Values{}
	if len(pods) > 0 {
		q.Set("pods", strings.Join(pods, ","))
	}
	if container != "" {
		q.Set("container", container)
	}
	if follow {
		q.Set("follow", "true")
	}
	if previous {
		q.Set("previous", "true")
	}
	if tail > 0 {
		q.Set("tail", strconv.FormatInt(tail, 10))
	}
	if since != "" {
		q.Set("since", since)
	}

	ctx, cancel := context.WithCancel(cmd.Context())
	defer cancel()

	// Translate Ctrl-C into context cancellation so the stream tears down
	// cleanly instead of dropping mid-line.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		cancel()
	}()
	defer signal.Stop(sigCh)

	client := newClient(cfg)
	path := fmt.Sprintf("/organizations/%s/a/%s/e/%s/c/%s/logs", link.OrgID, link.AppID, env, link.ComponentID)
	body, err := client.stream(ctx, path, q)
	if err != nil {
		return err
	}
	defer body.Close()

	if _, err := io.Copy(os.Stdout, body); err != nil {
		// Cancellation surfaces as a context error here; treat that as a
		// clean exit, not a failure.
		if ctx.Err() != nil {
			return nil
		}
		return err
	}
	return nil
}
