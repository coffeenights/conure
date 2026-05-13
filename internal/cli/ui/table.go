package ui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
)

// CellStyler customizes a single cell's style. row is the row index;
// table.HeaderRow (-1) is the header. Return nil to fall back to the default
// (header or body) style. Pass nil to RenderTable when no override is needed.
type CellStyler func(row, col int) *lipgloss.Style

// RenderTable prints a bordered table with consistent header/cell styling
// across the CLI. `rows` is a slice of equally-sized string slices; missing
// values should be passed as "-" by the caller (the table doesn't guess).
func RenderTable(headers []string, rows [][]string, styler CellStyler) {
	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212")).Padding(0, 1)
	cellStyle := lipgloss.NewStyle().Padding(0, 1)

	t := table.New().
		Border(lipgloss.RoundedBorder()).
		BorderStyle(lipgloss.NewStyle().Foreground(lipgloss.Color("240"))).
		Headers(headers...).
		StyleFunc(func(row, col int) lipgloss.Style {
			if styler != nil {
				if s := styler(row, col); s != nil {
					return *s
				}
			}
			if row == table.HeaderRow {
				return headerStyle
			}
			return cellStyle
		})
	for _, r := range rows {
		t.Row(r...)
	}
	fmt.Println(t)
}
