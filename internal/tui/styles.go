// Package tui implements the Bubble Tea terminal UI for repoview.
package tui

import "github.com/charmbracelet/lipgloss"

// ── Palette ───────────────────────────────────────────────────────────────────
// All colors are adaptive so the UI looks correct on both light and dark
// terminal backgrounds. Edit these to retheme the entire app at once.

var (
	colorBlue    = lipgloss.AdaptiveColor{Light: "#347aeb", Dark: "#347aeb"}
	colorRed     = lipgloss.AdaptiveColor{Light: "#f54242", Dark: "#f54242"}
	colorYellow  = lipgloss.AdaptiveColor{Light: "#b0ad09", Dark: "#e0d44f"}
	colorGreen   = lipgloss.AdaptiveColor{Light: "#1fb009", Dark: "#3fd020"}
	colorGray    = lipgloss.AdaptiveColor{Light: "#636363", Dark: "#888888"}
	colorFg      = lipgloss.AdaptiveColor{Light: "#1a1a1a", Dark: "#FFFDF5"}
	colorSubtle  = lipgloss.AdaptiveColor{Light: "#D9DCCF", Dark: "#383838"}
	colorText    = lipgloss.AdaptiveColor{Light: "#343433", Dark: "#C1C6B2"}
	colorSurface = lipgloss.AdaptiveColor{Light: "#D9DCCF", Dark: "#353533"}

	// Calendar heat-map — mirrors GitHub's contribution graph palette.
	calendarEmpty  = lipgloss.AdaptiveColor{Light: "#ebedf0", Dark: "#21262d"}
	calendarLevels = [4]lipgloss.AdaptiveColor{
		{Light: "#9be9a8", Dark: "#0e4429"},
		{Light: "#40c463", Dark: "#006d32"},
		{Light: "#30a14e", Dark: "#26a641"},
		{Light: "#216e39", Dark: "#39d353"},
	}
)

// ── Shared styles ─────────────────────────────────────────────────────────────

var (
	styleLabel   = lipgloss.NewStyle().Foreground(colorGray)
	styleValue   = lipgloss.NewStyle().Foreground(colorFg).Bold(true)
	styleDim     = lipgloss.NewStyle().Foreground(colorSubtle)
	styleDanger  = lipgloss.NewStyle().Foreground(colorRed).Bold(true)
	styleWarning = lipgloss.NewStyle().Foreground(colorYellow)
	styleSuccess = lipgloss.NewStyle().Foreground(colorGreen)
	styleAccent  = lipgloss.NewStyle().Foreground(colorBlue).Bold(true)

	// ── Tab borders ───────────────────────────────────────────────────────────

	activeTabBorder = lipgloss.Border{
		Top: "─", Bottom: " ", Left: "│", Right: "│",
		TopLeft: "╭", TopRight: "╮", BottomLeft: "┘", BottomRight: "└",
	}
	inactiveTabBorder = lipgloss.Border{
		Top: "─", Bottom: "─", Left: "│", Right: "│",
		TopLeft: "╭", TopRight: "╮", BottomLeft: "┴", BottomRight: "┴",
	}

	styleTab = lipgloss.NewStyle().
			Border(inactiveTabBorder, true).
			BorderForeground(colorSubtle).
			Foreground(colorGray).
			Padding(0, 1)

	styleActiveTab = lipgloss.NewStyle().
			Border(activeTabBorder, true).
			BorderForeground(colorBlue).
			Foreground(colorBlue).
			Bold(true).
			Padding(0, 1)

	// ── Status bar ────────────────────────────────────────────────────────────

	statusBarBg = lipgloss.NewStyle().
			Foreground(colorText).
			Background(colorSurface)

	statusNugget = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFDF5")).
			Padding(0, 1)

	statusPill      = statusNugget.Background(lipgloss.Color("#347aeb"))
	statusRightPill = statusNugget.Background(lipgloss.Color("#6124DF"))

	// ── Table cell styles — used by newTable() ────────────────────────────────

	tableCell     = lipgloss.NewStyle().Foreground(colorText).Padding(0, 2)
	tableHeader   = lipgloss.NewStyle().Foreground(colorGray).Padding(0, 2)
	tableSelected = lipgloss.NewStyle().Foreground(colorBlue).Bold(true).Padding(0, 2)

	// ── Todo badge styles — used by renderTodos() ─────────────────────────────

	styleBadgeTodo  = lipgloss.NewStyle().Foreground(lipgloss.Color("#ffffff")).Background(colorGray).Bold(true).Padding(0, 1)
	styleBadgeFixme = lipgloss.NewStyle().Foreground(lipgloss.Color("#ffffff")).Background(colorRed).Bold(true).Padding(0, 1)
	styleBadgeHack  = lipgloss.NewStyle().Foreground(lipgloss.Color("#ffffff")).Background(colorYellow).Bold(true).Padding(0, 1)
)

// banner is the ASCII art shown on the input screen.
var banner = `
 ██████╗ ███████╗██████╗  ██████╗ ██╗   ██╗██╗███████╗██╗    ██╗
 ██╔══██╗██╔════╝██╔══██╗██╔═══██╗██║   ██║██║██╔════╝██║    ██║
 ██████╔╝█████╗  ██████╔╝██║   ██║██║   ██║██║█████╗  ██║ █╗ ██║
 ██╔══██╗██╔══╝  ██╔═══╝ ██║   ██║╚██╗ ██╔╝██║██╔══╝  ██║███╗██║
 ██║  ██║███████╗██║     ╚██████╔╝ ╚████╔╝ ██║███████╗╚███╔███╔╝
 ╚═╝  ╚═╝╚══════╝╚═╝      ╚═════╝   ╚═══╝  ╚═╝╚══════╝ ╚══╝╚══╝ `
