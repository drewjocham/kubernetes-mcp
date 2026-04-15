package charts

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

type Histogram struct {
	Buckets        []float64
	MaxHeight      int // terminal lines available
	Width          int
	BarColor       lipgloss.Color
	LabelColor     lipgloss.Color
	BucketDuration time.Duration
}

func (h Histogram) Render() string {
	if len(h.Buckets) == 0 || h.MaxHeight < 2 {
		return ""
	}

	maxVal := maxFloat(h.Buckets)
	bucketW := h.Width / len(h.Buckets)
	if bucketW < 1 {
		bucketW = 1
	}

	grid := make([][]bool, h.MaxHeight)
	for i := range grid {
		grid[i] = make([]bool, len(h.Buckets))
	}

	for col, v := range h.Buckets {
		barH := 0
		if maxVal > 0 {
			barH = int((v / maxVal) * float64(h.MaxHeight))
		}
		for row := 0; row < barH; row++ {
			grid[h.MaxHeight-1-row][col] = true
		}
	}

	barRune := "█"
	emptyRune := " "

	var rows []string
	for _, row := range grid {
		var sb strings.Builder
		for _, filled := range row {
			cell := emptyRune
			if filled {
				cell = barRune
			}
			bar := lipgloss.NewStyle().Foreground(h.BarColor).Render(cell)
			sb.WriteString(strings.Repeat(bar, bucketW))
		}
		rows = append(rows, sb.String())
	}

	// Label row
	var labelRow strings.Builder
	labelStyle := lipgloss.NewStyle().Foreground(h.LabelColor)
	for i := range h.Buckets {
		label := ""
		if h.BucketDuration > 0 {
			label = fmt.Sprintf("-%dh", (len(h.Buckets)-i)*int(h.BucketDuration.Hours()))
		} else {
			label = fmt.Sprintf("%d", i)
		}
		if len(label) > bucketW {
			label = label[:bucketW]
		}
		labelRow.WriteString(labelStyle.Render(fmt.Sprintf("%-*s", bucketW, label)))
	}
	rows = append(rows, labelRow.String())

	return strings.Join(rows, "\n")
}
