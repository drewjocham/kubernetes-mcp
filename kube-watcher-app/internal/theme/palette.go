package theme

import "github.com/charmbracelet/lipgloss"

// Background layers — Proton/Apple dark theme
var (
	BgBase     = lipgloss.Color("#0d0d0f") // Proton dark base
	BgSurface  = lipgloss.Color("#1a1a1f") // Card backgrounds
	BgElevated = lipgloss.Color("#252529") // Elevated cards
	BgOverlay  = lipgloss.Color("#0f0f12") // Modal overlays
	BgAccent   = lipgloss.Color("#1e1e22") // Accent areas
)

// Accent — Proton purple gradient scale
var (
	AccentPrimary   = lipgloss.Color("#8b5cf6") // Proton purple
	AccentSecondary = lipgloss.Color("#7c3aed") // Darker purple for headers
	AccentMuted     = lipgloss.Color("#374151") // Muted slate
	AccentBright    = lipgloss.Color("#a78bfa") // Light purple
	AccentHover     = lipgloss.Color("#9333ea") // Hover purple
)

// Text — modern web app hierarchy
var (
	TextPrimary   = lipgloss.Color("#ffffff") // Pure white
	TextSecondary = lipgloss.Color("#e2e8f0") // Light blue-gray
	TextMuted     = lipgloss.Color("#94a3b8") // Muted blue-gray
	TextInverse   = lipgloss.Color("#0f172a") // Dark slate for contrast
	TextAccent    = lipgloss.Color("#3b82f6") // Bright blue for accents
)

// Semantic — Proton app colors
var (
	ColorCritical = lipgloss.Color("#ef4444") // Red for critical
	ColorHigh     = lipgloss.Color("#f97316") // Orange for high
	ColorMedium   = lipgloss.Color("#eab308") // Yellow for medium
	ColorLow      = lipgloss.Color("#22c55e") // Green for low
	ColorInfo     = lipgloss.Color("#8b5cf6") // Purple for info
	ColorSuccess  = lipgloss.Color("#10b981") // Green for success
)

// Borders — Proton subtle borders
var (
	BorderSubtle = lipgloss.Color("#374151") // Gray border
	BorderFocus  = AccentPrimary
	BorderActive = lipgloss.Color("#8b5cf6") // Purple border for active
	BorderHover  = lipgloss.Color("#a78bfa") // Light purple for hover
)
