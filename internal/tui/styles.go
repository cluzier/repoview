// Package tui implements the Bubble Tea terminal UI for repoview.
package tui

import "github.com/charmbracelet/lipgloss"

// ── Palette ───────────────────────────────────────────────────────────────────
// All colors are adaptive so the UI looks correct on both light and dark
// terminal backgrounds. Edit these to retheme the entire app at once.

var (
	// Semantic colors — all values pass WCAG AA (4.5:1) on their respective
	// terminal backgrounds. Light values target white (#ffffff); dark values
	// target a typical dark terminal (~#1e1e1e).
	colorBlue    = lipgloss.AdaptiveColor{Light: "#0969da", Dark: "#4493f8"}
	colorRed     = lipgloss.AdaptiveColor{Light: "#cf222e", Dark: "#f85149"}
	colorYellow  = lipgloss.AdaptiveColor{Light: "#9a6700", Dark: "#e3b341"}
	colorGreen   = lipgloss.AdaptiveColor{Light: "#1a7f37", Dark: "#3fb950"}
	colorGray    = lipgloss.AdaptiveColor{Light: "#636363", Dark: "#8b949e"}
	colorFg      = lipgloss.AdaptiveColor{Light: "#1f2328", Dark: "#e6edf3"}
	colorSubtle  = lipgloss.AdaptiveColor{Light: "#57606a", Dark: "#848d97"}
	colorText    = lipgloss.AdaptiveColor{Light: "#24292f", Dark: "#adbac7"}
	colorSurface = lipgloss.AdaptiveColor{Light: "#f6f8fa", Dark: "#1f2328"}

	// Calendar heat-map — adapted from GitHub's contribution graph palette.
	// Levels 0-1 are intentionally subtle (visualization gradient, not body text).
	calendarEmpty  = lipgloss.AdaptiveColor{Light: "#c8d0d9", Dark: "#484f58"}
	calendarLevels = [4]lipgloss.AdaptiveColor{
		{Light: "#7bc96f", Dark: "#196127"},
		{Light: "#40c463", Dark: "#2da44e"},
		{Light: "#30a14e", Dark: "#3fb950"},
		{Light: "#216e39", Dark: "#56d364"},
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

	// ── Todo badge styles — used by renderTodos() ─────────────────────────────

	// Badge text: white on light-mode badge bg (dark colors), dark on dark-mode badge bg (bright colors).
	badgeFg = lipgloss.AdaptiveColor{Light: "#ffffff", Dark: "#1f2328"}

	styleBadgeTodo  = lipgloss.NewStyle().Foreground(badgeFg).Background(colorGray).Bold(true).Padding(0, 1)
	styleBadgeFixme = lipgloss.NewStyle().Foreground(badgeFg).Background(colorRed).Bold(true).Padding(0, 1)
	styleBadgeHack  = lipgloss.NewStyle().Foreground(badgeFg).Background(colorYellow).Bold(true).Padding(0, 1)
)

// banner is the ASCII art shown on the input screen.
var banner = `
 ██████╗ ███████╗██████╗  ██████╗ ██╗   ██╗██╗███████╗██╗    ██╗
 ██╔══██╗██╔════╝██╔══██╗██╔═══██╗██║   ██║██║██╔════╝██║    ██║
 ██████╔╝█████╗  ██████╔╝██║   ██║██║   ██║██║█████╗  ██║ █╗ ██║
 ██╔══██╗██╔══╝  ██╔═══╝ ██║   ██║╚██╗ ██╔╝██║██╔══╝  ██║███╗██║
 ██║  ██║███████╗██║     ╚██████╔╝ ╚████╔╝ ██║███████╗╚███╔███╔╝
 ╚═╝  ╚═╝╚══════╝╚═╝      ╚═════╝   ╚═══╝  ╚═╝╚══════╝ ╚══╝╚══╝ `
