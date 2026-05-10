package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/spf13/cobra"
)

var podsCmd = &cobra.Command{
	Use:   "pods",
	Short: "List pods backing this component in the linked environment",
	RunE:  runPods,
}

func init() {
	podsCmd.Flags().String("env", "", "Environment (overrides link)")
	rootCmd.AddCommand(podsCmd)
}

func runPods(cmd *cobra.Command, args []string) error {
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
	client := newClient(cfg)

	pods, err := listComponentPods(client, link.OrgID, link.AppID, env, link.ComponentID)
	if err != nil {
		return err
	}

	if outputFlag == "json" {
		out, _ := json.MarshalIndent(pods, "", "  ")
		fmt.Println(string(out))
		return nil
	}

	if len(pods) == 0 {
		info.Printf("No pods found for `%s` in env `%s`\n", link.ComponentName, env)
		return nil
	}

	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212")).Padding(0, 1)
	cellStyle := lipgloss.NewStyle().Padding(0, 1)
	t := table.New().
		Border(lipgloss.RoundedBorder()).
		BorderStyle(lipgloss.NewStyle().Foreground(lipgloss.Color("240"))).
		Headers("NAME", "READY", "PHASE", "RESTARTS", "CONTAINERS").
		StyleFunc(func(row, col int) lipgloss.Style {
			if row == table.HeaderRow {
				return headerStyle
			}
			return cellStyle
		})
	for _, p := range pods {
		ready := "false"
		if p.Ready {
			ready = "true"
		}
		t.Row(
			p.Name,
			ready,
			p.Phase,
			fmt.Sprintf("%d", p.Restarts),
			strings.Join(p.Containers, ", "),
		)
	}
	fmt.Println(t)
	return nil
}
