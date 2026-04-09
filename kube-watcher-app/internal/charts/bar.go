package charts

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type Bar struct {
	Label string
	Value float64
	Color lipgloss.Color
}

type BarChart struct {
	Bars       []Bar
	MaxWidth   int
	BarColor   lipgloss.Color
	LabelColor lipgloss.Color
	ValueColor lipgloss.Color
}

func (bc BarChart) Render() string {
	if len(bc.Bars) == 0 {
		return ""
	}

	maxVal := 0.0
	maxLabelLen := 0
	for _, b := range bc.Bars {
		if b.Value > maxVal {
			maxVal = b.Value
		}
		if len(b.Label) > maxLabelLen {
			maxLabelLen = len(b.Label)
		}
	}
	if maxLabelLen > 20 {
		maxLabelLen = 20
	}

	valStr := fmt.Sprintf("%.0f", maxVal)
	valWidth := len(valStr) + 1
	barZone := bc.MaxWidth - maxLabelLen - valWidth - 4
	if barZone < 4 {
		barZone = 4
	}

	var rows []string
	for _, b := range bc.Bars {
		label := b.Label
		if len(label) > maxLabelLen {
			label = label[:maxLabelLen-1] + "…"
		}
		label = fmt.Sprintf("%-*s", maxLabelLen, label)

		barLen := 0
		if maxVal > 0 {
			barLen = int((b.Value / maxVal) * float64(barZone))
		}

		color := bc.BarColor
		if b.Color != "" {
			color = b.Color
		}
		bar := lipgloss.NewStyle().Foreground(color).Render(strings.Repeat("█", barLen))
		if barLen < barZone {
			bar += lipgloss.NewStyle().Foreground(lipgloss.Color("#2D2D35")).Render(strings.Repeat("░", barZone-barLen))
		}

		labelRendered := lipgloss.NewStyle().Foreground(bc.LabelColor).Render(label)
		valRendered := lipgloss.NewStyle().Foreground(bc.ValueColor).Render(fmt.Sprintf(" %.1f", b.Value))

		rows = append(rows, labelRendered+" "+bar+valRendered)
	}
	return strings.Join(rows, "\n")
}
