package theme

import "github.com/charmbracelet/lipgloss"

func PanelStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Background(BgSurface).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(BorderSubtle).
		Padding(1, 2).
		MarginBottom(1)
}

func PanelFocusedStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Background(BgSurface).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(BorderFocus).
		Padding(0, 1)
}

func SidebarStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Background(BgAccent).
		Padding(1, 1)
}

func SidebarItemStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(TextMuted).
		Padding(0, 2).
		MarginRight(1)
}

func SidebarItemActiveStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(TextPrimary).
		Background(AccentPrimary).
		Bold(true).
		Padding(0, 2).
		MarginRight(1).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(AccentPrimary)
}

func SidebarItemHoverStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(TextPrimary).
		Background(BgElevated).
		Padding(0, 2).
		MarginRight(1)
}

func HeaderStyle(width int) lipgloss.Style {
	return lipgloss.NewStyle().
		Background(AccentSecondary).
		Foreground(TextPrimary).
		Bold(true).
		Padding(1, 2).
		Width(width).
		BorderBottom(true).
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(BorderSubtle)
}

func StatusBarStyle(width int) lipgloss.Style {
	return lipgloss.NewStyle().
		Background(BgAccent).
		Foreground(TextMuted).
		Padding(1, 2).
		Width(width).
		BorderTop(true).
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(BorderSubtle)
}

func TableHeaderStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Background(AccentSecondary).
		Foreground(TextPrimary).
		Bold(true).
		Padding(0, 1).
		BorderBottom(true).
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(BorderSubtle)
}

func TableRowStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(TextPrimary).
		Padding(0, 1)
}

func TableSelectedStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Background(AccentPrimary).
		Foreground(TextPrimary).
		Bold(true).
		Padding(0, 1)
}

func AlertBadgeStyle(severity string) lipgloss.Style {
	var color lipgloss.Color
	switch severity {
	case "critical":
		color = ColorCritical
	case "high":
		color = ColorHigh
	case "medium":
		color = ColorMedium
	case "low":
		color = ColorLow
	default:
		color = ColorInfo
	}
	return lipgloss.NewStyle().
		Foreground(color).
		Bold(true)
}

func CodeBlockStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Background(BgAccent).
		Foreground(TextSecondary).
		Padding(1, 2).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(BorderSubtle).
		MarginTop(1)
}

func ChatMessageUserStyle(width int) lipgloss.Style {
	return lipgloss.NewStyle().
		Background(AccentMuted).
		Foreground(TextPrimary).
		Padding(0, 1).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(AccentPrimary).
		Width(width - 4)
}

func ChatMessageAgentStyle(width int) lipgloss.Style {
	return lipgloss.NewStyle().
		Background(BgElevated).
		Foreground(TextPrimary).
		Padding(0, 1).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(BorderSubtle).
		Width(width - 4)
}

func ChatMessageSystemStyle(width int) lipgloss.Style {
	return lipgloss.NewStyle().
		Background(BgOverlay).
		Foreground(TextSecondary).
		Italic(true).
		Padding(0, 1).
		BorderLeft(true).
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(AccentPrimary).
		Width(width - 4)
}

func ChatInputStyle(width int) lipgloss.Style {
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(AccentPrimary).
		Padding(0, 1).
		Width(width - 4)
}

func HintCardStyle(width int) lipgloss.Style {
	return lipgloss.NewStyle().
		Background(BgElevated).
		Padding(0, 1).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(BorderSubtle).
		Width(width - 2).
		MarginBottom(1)
}

func TitleStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(AccentBright).
		Bold(true)
}

func CardTitleStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color("110"))
}

func CardValueStyle() lipgloss.Style {
	return lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("230"))
}

func SubtitleStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(TextSecondary)
}

func ConnectedStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(ColorSuccess)
}

func DisconnectedStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(ColorCritical)
}

func BadgeStyle(color lipgloss.Color) lipgloss.Style {
	return lipgloss.NewStyle().
		Background(color).
		Foreground(TextInverse).
		Padding(0, 1).
		Bold(true)
}

func DimStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(TextMuted)
}

// Modern app-like styles
func CardStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Background(BgElevated).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(BorderSubtle).
		Padding(1, 2).
		MarginBottom(1)
}

func SectionHeaderStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(AccentBright).
		Bold(true).
		BorderBottom(true).
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(BorderSubtle).
		PaddingBottom(1).
		MarginBottom(1)
}

func ButtonStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Background(AccentPrimary).
		Foreground(TextPrimary).
		Padding(0, 2).
		MarginRight(1).
		Bold(true)
}

func ButtonHoverStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Background(AccentHover).
		Foreground(TextPrimary).
		Padding(0, 2).
		MarginRight(1).
		Bold(true)
}

func InputStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Background(BgSurface).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(BorderFocus).
		Padding(0, 1)
}

func HighlightStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Background(AccentPrimary).
		Foreground(TextPrimary).
		Bold(true).
		Padding(0, 1)
}
