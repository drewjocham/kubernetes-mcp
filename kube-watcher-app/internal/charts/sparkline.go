package charts

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var blockRunes = []rune{' ', '▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}

func Sparkline(values []float64, width int, style lipgloss.Style) string {
	if len(values) == 0 || width <= 0 {
		return strings.Repeat(" ", width)
	}

	sampled := sample(values, width)

	maxV := maxFloat(sampled)
	var sb strings.Builder
	for _, v := range sampled {
		idx := 0
		if maxV > 0 {
			idx = int((v / maxV) * float64(len(blockRunes)-1))
		}
		if idx >= len(blockRunes) {
			idx = len(blockRunes) - 1
		}
		sb.WriteRune(blockRunes[idx])
	}
	return style.Render(sb.String())
}

func SparklineColored(values []float64, width int, lowColor, highColor lipgloss.Color) string {
	if len(values) == 0 || width <= 0 {
		return strings.Repeat(" ", width)
	}
	sampled := sample(values, width)
	maxV := maxFloat(sampled)

	var sb strings.Builder
	for _, v := range sampled {
		idx := 0
		if maxV > 0 {
			idx = int((v / maxV) * float64(len(blockRunes)-1))
		}
		if idx >= len(blockRunes) {
			idx = len(blockRunes) - 1
		}
		// Pick color based on relative height
		color := lowColor
		if maxV > 0 && v/maxV > 0.6 {
			color = highColor
		}
		char := lipgloss.NewStyle().Foreground(color).Render(string(blockRunes[idx]))
		sb.WriteString(char)
	}
	return sb.String()
}

func sample(values []float64, n int) []float64 {
	if len(values) <= n {
		return values
	}
	result := make([]float64, n)
	stride := float64(len(values)) / float64(n)
	for i := 0; i < n; i++ {
		start := int(float64(i) * stride)
		end := int(float64(i+1) * stride)
		if end > len(values) {
			end = len(values)
		}
		sum := 0.0
		for _, v := range values[start:end] {
			sum += v
		}
		result[i] = sum / float64(end-start)
	}
	return result
}

func maxFloat(vs []float64) float64 {
	if len(vs) == 0 {
		return 0
	}
	m := vs[0]
	for _, v := range vs[1:] {
		if v > m {
			m = v
		}
	}
	return m
}
