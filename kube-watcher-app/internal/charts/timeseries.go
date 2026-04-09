package charts

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// TimePoint is a (time, value) sample.
type TimePoint struct {
	Time  time.Time
	Value float64
}

// TimeSeries renders a simple ASCII line chart.
type TimeSeries struct {
	Points     []TimePoint
	Width      int
	Height     int
	LineColor  lipgloss.Color
	LabelColor lipgloss.Color
	Title      string
}

// Render produces the time series chart as a multi-line string.
func (ts TimeSeries) Render() string {
	if len(ts.Points) == 0 || ts.Width < 4 || ts.Height < 3 {
		return lipgloss.NewStyle().Foreground(ts.LabelColor).Render("  No data")
	}

	vals := make([]float64, len(ts.Points))
	for i, p := range ts.Points {
		vals[i] = p.Value
	}

	sampled := sample(vals, ts.Width-4)
	maxV := maxFloat(sampled)
	minV := minFloat(sampled)
	rangeV := maxV - minV
	if rangeV == 0 {
		rangeV = 1
	}

	// Build grid
	innerH := ts.Height - 2 // reserve top+bottom labels
	grid := make([][]rune, innerH)
	for i := range grid {
		grid[i] = make([]rune, len(sampled))
		for j := range grid[i] {
			grid[i][j] = ' '
		}
	}

	// Plot points
	for col, v := range sampled {
		row := innerH - 1 - int((v-minV)/rangeV*float64(innerH-1))
		if row < 0 {
			row = 0
		}
		if row >= innerH {
			row = innerH - 1
		}
		grid[row][col] = '●'
	}

	// Draw axis
	axisStyle := lipgloss.NewStyle().Foreground(ts.LabelColor)
	lineStyle := lipgloss.NewStyle().Foreground(ts.LineColor)

	var rows []string

	// Title
	if ts.Title != "" {
		rows = append(rows, lipgloss.NewStyle().Foreground(ts.LineColor).Bold(true).Render(ts.Title))
	}

	// Value axis label (max)
	rows = append(rows, axisStyle.Render(fmt.Sprintf("%.1f ┐", maxV)))

	for rowIdx, row := range grid {
		var sb strings.Builder
		// Y-axis
		if rowIdx == innerH/2 {
			label := fmt.Sprintf("%.1f │", (maxV+minV)/2)
			sb.WriteString(axisStyle.Render(label))
		} else {
			sb.WriteString(axisStyle.Render(strings.Repeat(" ", len(fmt.Sprintf("%.1f │", maxV)))))
		}
		for _, ch := range row {
			if ch == '●' {
				sb.WriteString(lineStyle.Render("●"))
			} else {
				sb.WriteString(axisStyle.Render("·"))
			}
		}
		rows = append(rows, sb.String())
	}

	// Min label
	rows = append(rows, axisStyle.Render(fmt.Sprintf("%.1f ┘", minV)))

	// Time axis
	if len(ts.Points) > 0 {
		first := ts.Points[0].Time.Format("15:04")
		last := ts.Points[len(ts.Points)-1].Time.Format("15:04")
		pad := ts.Width - len(first) - len(last) - 4
		if pad < 0 {
			pad = 0
		}
		rows = append(rows, axisStyle.Render(fmt.Sprintf("    %s%s%s", first, strings.Repeat(" ", pad), last)))
	}

	return strings.Join(rows, "\n")
}

func minFloat(vs []float64) float64 {
	if len(vs) == 0 {
		return 0
	}
	m := vs[0]
	for _, v := range vs[1:] {
		if v < m {
			m = v
		}
	}
	return m
}
