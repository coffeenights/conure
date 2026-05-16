package main

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/coffeenights/conure/internal/cli/ui"
)

var podsCmd = &cobra.Command{
	Use:   "pods",
	Short: "List pods backing this component in the linked environment",
	RunE:  runPods,
}

func init() {
	addEnvFlag(podsCmd)
	rootCmd.AddCommand(podsCmd)
}

func runPods(cmd *cobra.Command, _ []string) error {
	lc, err := resolveTarget(cmd)
	if err != nil {
		return err
	}
	pods, err := lc.Client.ListComponentPods(cmd.Context(), lc.Link.OrgID, lc.Link.AppID, lc.Env, lc.Link.ComponentID)
	if err != nil {
		return err
	}
	return ui.Render(pods, func() error {
		if len(pods) == 0 {
			ui.Info("No pods found for `%s` in env `%s`\n", lc.Link.ComponentName, lc.Env)
			return nil
		}
		rows := make([][]string, len(pods))
		for i, p := range pods {
			ready := "false"
			if p.Ready {
				ready = "true"
			}
			rows[i] = []string{
				p.Name,
				ready,
				p.Phase,
				fmt.Sprintf("%d", p.Restarts),
				strings.Join(p.Containers, ", "),
			}
		}
		ui.RenderTable([]string{"NAME", "READY", "PHASE", "RESTARTS", "CONTAINERS"}, rows, nil)
		return nil
	})
}
