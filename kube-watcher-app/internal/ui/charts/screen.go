package chartsscreen

import (
	"context"
	"fmt"
	"strings"
	"time"

	"kube-watcher-app/internal/charts"
	"kube-watcher-app/internal/data"
	promclient "kube-watcher-app/internal/data/prometheus"
	"kube-watcher-app/internal/theme"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Source selects the active data source.
type Source int

const (
	SourceBadgerDB Source = iota
	SourcePrometheus
	SourceKubernetes
)

var sourceLabels = []string{"BadgerDB", "Prometheus", "K8s API"}

// PrometheusDataMsg carries fetched Prometheus series.
type PrometheusDataMsg struct{ Series []data.MetricSeries }

// Screen shows multi-source SRE charts.
type Screen struct {
	source    Source
	incidents []data.Incident
	promData  []data.MetricSeries
	promCli   *promclient.Client
	viewport  viewport.Model
	width     int
	height    int
}

// New creates a Charts screen.
func New(width, height int) *Screen {
	return &Screen{
		promCli:  promclient.New(),
		viewport: viewport.New(width-2, height-6),
		width:    width,
		height:   height,
	}
}

// SetHistory supplies BadgerDB incident data.
func (s *Screen) SetHistory(incidents []data.Incident) {
	s.incidents = incidents
	if s.source == SourceBadgerDB {
		s.refresh()
	}
}

func (s Screen) Init() tea.Cmd { return nil }

func (s *Screen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "left", "h":
			if s.source > 0 {
				s.source--
				s.refresh()
				if s.source == SourcePrometheus {
					return s, s.fetchPrometheus()
				}
			}
			return s, nil
		case "right", "l":
			if int(s.source) < len(sourceLabels)-1 {
				s.source++
				s.refresh()
				if s.source == SourcePrometheus {
					return s, s.fetchPrometheus()
				}
			}
			return s, nil
		case "r":
			if s.source == SourcePrometheus {
				return s, s.fetchPrometheus()
			}
			s.refresh()
			return s, nil
		}
	case PrometheusDataMsg:
		s.promData = msg.Series
		s.refresh()
	}

	var cmd tea.Cmd
	s.viewport, cmd = s.viewport.Update(msg)
	return s, cmd
}

func (s *Screen) fetchPrometheus() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		// Default query: container restart count
		series, err := s.promCli.QueryRange(ctx,
			`kube_pod_container_status_restarts_total`,
			time.Now().Add(-24*time.Hour), time.Now(), 30*time.Minute)
		if err != nil {
			return PrometheusDataMsg{Series: nil}
		}
		return PrometheusDataMsg{Series: series}
	}
}

func (s *Screen) refresh() {
	s.viewport.SetContent(s.renderContent())
}

func (s Screen) View() string {
	// Source tabs
	var tabs []string
	for i, label := range sourceLabels {
		var tab string
		if Source(i) == s.source {
			tab = lipgloss.NewStyle().
				Foreground(theme.TextInverse).
				Background(theme.AccentPrimary).
				Bold(true).
				Padding(0, 2).
				Render(label)
		} else {
			tab = lipgloss.NewStyle().
				Foreground(theme.TextSecondary).
				Background(theme.BgElevated).
				Padding(0, 2).
				Render(label)
		}
		tabs = append(tabs, tab)
	}
	tabBar := lipgloss.JoinHorizontal(lipgloss.Top, tabs...)
	hint := lipgloss.NewStyle().Foreground(theme.TextMuted).Render("  ←/→ switch source  r=refresh")
	header := lipgloss.JoinHorizontal(lipgloss.Top, tabBar, "  ", hint)

	return theme.PanelStyle().Width(s.width).Height(s.height - 2).
		Render(lipgloss.JoinVertical(lipgloss.Left,
			header,
			lipgloss.NewStyle().Foreground(theme.BorderSubtle).Render(strings.Repeat("─", s.width-4)),
			s.viewport.View(),
		))
}

func (s *Screen) Resize(width, height int) {
	s.width = width
	s.height = height
	s.viewport = viewport.New(width-2, height-6)
	s.refresh()
}

func (s Screen) HelpBindings() []key.Binding {
	return []key.Binding{
		key.NewBinding(key.WithKeys("left", "h"), key.WithHelp("←/h", "prev source")),
		key.NewBinding(key.WithKeys("right", "l"), key.WithHelp("→/l", "next source")),
		key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh")),
	}
}

func (s Screen) Title() string {
	return fmt.Sprintf("Charts · %s", sourceLabels[s.source])
}

func (s Screen) renderContent() string {
	contentW := s.width - 6
	if contentW < 20 {
		return ""
	}

	switch s.source {
	case SourceBadgerDB:
		return s.renderBadgerCharts(contentW)
	case SourcePrometheus:
		return s.renderPrometheusCharts(contentW)
	case SourceKubernetes:
		return s.renderK8sCharts(contentW)
	}
	return ""
}

func (s Screen) renderBadgerCharts(w int) string {
	if len(s.incidents) == 0 {
		return lipgloss.NewStyle().Foreground(theme.TextMuted).Italic(true).Padding(2, 2).
			Render("No history data. Ensure kube-watcher MCP is running.")
	}

	var sb strings.Builder
	sb.WriteString(lipgloss.NewStyle().Foreground(theme.AccentBright).Bold(true).Render("Incident Frequency (24h buckets)") + "\n\n")

	// Bucket incidents by hour
	buckets := make([]float64, 24)
	for _, inc := range s.incidents {
		h := time.Since(inc.Timestamp).Hours()
		idx := int(h)
		if idx >= 0 && idx < 24 {
			buckets[23-idx] += float64(inc.Occurrences)
		}
	}
	hist := charts.Histogram{
		Buckets:        buckets,
		MaxHeight:      10,
		Width:          w,
		BarColor:       theme.AccentPrimary,
		LabelColor:     theme.TextSecondary,
		BucketDuration: time.Hour,
	}
	sb.WriteString(hist.Render())
	sb.WriteString("\n\n")

	// Top pods by occurrence sparklines
	sb.WriteString(lipgloss.NewStyle().Foreground(theme.AccentBright).Bold(true).Render("Top Pods — Occurrence Trend") + "\n\n")
	shown := 0
	for _, inc := range s.incidents {
		if shown >= 8 {
			break
		}
		if len(inc.History) == 0 {
			continue
		}
		spark := charts.SparklineColored(inc.History, 20, theme.ColorInfo, theme.ColorCritical)
		label := fmt.Sprintf("%-24s", inc.Namespace+"/"+inc.Name)
		if len(label) > 24 {
			label = label[:23] + "…"
		}
		sb.WriteString(lipgloss.NewStyle().Foreground(theme.TextSecondary).Render(label))
		sb.WriteString("  " + spark + "\n")
		shown++
	}

	return sb.String()
}

func (s Screen) renderPrometheusCharts(w int) string {
	if len(s.promData) == 0 {
		return lipgloss.NewStyle().Foreground(theme.TextMuted).Italic(true).Padding(2, 2).
			Render("Fetching Prometheus data… (press r to retry)")
	}

	var sb strings.Builder
	sb.WriteString(lipgloss.NewStyle().Foreground(theme.AccentBright).Bold(true).Render("Container Restart Counts") + "\n\n")

	bc := charts.BarChart{
		MaxWidth:   w,
		BarColor:   theme.AccentPrimary,
		LabelColor: theme.TextSecondary,
		ValueColor: theme.TextPrimary,
	}
	for i, series := range s.promData {
		if i >= 10 {
			break
		}
		if len(series.Points) == 0 {
			continue
		}
		name := series.Labels["pod"]
		if name == "" {
			name = series.Labels["container"]
		}
		if name == "" {
			continue
		}
		last := series.Points[len(series.Points)-1].Value
		bc.Bars = append(bc.Bars, charts.Bar{Label: name, Value: last})
	}
	if len(bc.Bars) == 0 {
		sb.WriteString(lipgloss.NewStyle().Foreground(theme.TextMuted).Render("  No series with data.\n"))
	} else {
		sb.WriteString(bc.Render())
	}

	// Time series for first metric
	if len(s.promData) > 0 && len(s.promData[0].Points) > 1 {
		sb.WriteString("\n\n")
		sb.WriteString(lipgloss.NewStyle().Foreground(theme.AccentBright).Bold(true).Render("Trend — "+firstLabel(s.promData[0].Labels)) + "\n\n")
		pts := make([]charts.TimePoint, len(s.promData[0].Points))
		for i, p := range s.promData[0].Points {
			pts[i] = charts.TimePoint{Time: p.Timestamp, Value: p.Value}
		}
		ts := charts.TimeSeries{
			Points:     pts,
			Width:      w,
			Height:     10,
			LineColor:  theme.AccentBright,
			LabelColor: theme.TextSecondary,
		}
		sb.WriteString(ts.Render())
	}

	return sb.String()
}

func (s Screen) renderK8sCharts(w int) string {
	return lipgloss.NewStyle().Foreground(theme.TextMuted).Italic(true).Padding(2, 2).
		Render("K8s API metrics: run 'view status' from the kube-watcher CLI to fetch live data.\n\nComing soon: automatic metrics-server polling.")
}

func firstLabel(labels map[string]string) string {
	for _, k := range []string{"pod", "container", "namespace", "__name__"} {
		if v, ok := labels[k]; ok {
			return v
		}
	}
	for _, v := range labels {
		return v
	}
	return "metric"
}
