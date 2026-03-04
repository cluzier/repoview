package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	ltable "github.com/charmbracelet/lipgloss/table"
)

// ── Top-level view dispatch ───────────────────────────────────────────────────

func (m Model) View() string {
	if m.width == 0 {
		return ""
	}
	switch m.state {
	case stateInput:
		return m.viewInput()
	case stateLoading:
		return m.viewLoading()
	case stateViewer:
		return m.viewViewer()
	default:
		return m.viewMain()
	}
}

// ── Input screen ──────────────────────────────────────────────────────────────

func (m Model) viewInput() string {
	subtitle := styleLabel.Render("Git repository analyzer  ·  local paths & GitHub URLs")

	inputBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorBlue).
		Padding(0, 1).
		Width(64).
		Render(m.input.View())

	var errLine string
	if m.inputErr != "" {
		errLine = "\n" + styleDanger.Render("  ✖  "+m.inputErr)
	}

	content := lipgloss.JoinVertical(lipgloss.Center,
		styleAccent.Render(banner),
		"",
		subtitle,
		"",
		"",
		inputBox,
		errLine,
		"",
		styleDim.Render("Enter a local path or GitHub URL, then press Enter"),
		styleDim.Render("Ctrl+C / Esc to quit"),
	)

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
}

// ── Loading screen ────────────────────────────────────────────────────────────

func (m Model) viewLoading() string {
	blob := lipgloss.NewStyle().Foreground(colorBlue).Render(renderBlob(m.blobT, 36, 15))
	label := lipgloss.JoinHorizontal(lipgloss.Center,
		m.spinner.View()+" ",
		styleValue.Render(m.loadingMsg),
	)
	content := lipgloss.JoinVertical(lipgloss.Center, blob, "", label)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
}

// ── File viewer ───────────────────────────────────────────────────────────────

func (m Model) viewViewer() string {
	titleStyle := lipgloss.NewStyle().
		Foreground(colorFg).Bold(true).
		Background(lipgloss.Color("#0369a1")).
		Padding(0, 2).
		Width(m.width)
	header := titleStyle.Render("  " + m.viewerTitle)

	pct := int(m.viewer.ScrollPercent() * 100)
	footerStyle := lipgloss.NewStyle().
		Foreground(colorText).
		Background(lipgloss.Color("#0c1e2c")).
		Width(m.width).Padding(0, 1)
	hints := styleDim.Render("↑/↓ scroll  PgUp/PgDn  q/Esc close")
	scrollPct := styleAccent.Render(fmt.Sprintf("%d%%", pct))
	footer := footerStyle.Render(lipgloss.JoinHorizontal(lipgloss.Top,
		lipgloss.NewStyle().Width(m.width-10).Render(hints),
		scrollPct,
	))

	return lipgloss.JoinVertical(lipgloss.Left, header, m.viewer.View(), footer)
}

// ── Main view ─────────────────────────────────────────────────────────────────

func (m Model) viewMain() string {
	pw := m.panelWidth()

	header := m.renderHeader()
	tabs := m.renderTabs(pw)

	var body string
	if m.loading {
		body = lipgloss.Place(pw, m.bodyHeight(), lipgloss.Center, lipgloss.Center,
			lipgloss.JoinHorizontal(lipgloss.Center, m.spinner.View()+" ", styleValue.Render("Analyzing…")))
	} else if m.err != nil {
		body = styleDanger.Render(fmt.Sprintf("\n  ✖  %v\n\n  Make sure the path is a valid git repository.", m.err))
	} else {
		switch m.activeTab {
		case TabOverview:
			body = m.renderOverview()
		case TabChurn:
			body = m.renderChurn()
		case TabActivity:
			body = m.renderActivity()
		case TabTodos:
			body = m.renderTodos()
		case TabStale:
			body = m.renderStale()
		}
	}

	// Pin the body to a fixed height so the status bar never drifts.
	fixedBodyHeight := m.bodyHeight() - m.searchBarHeight()
	if fixedBodyHeight < 1 {
		fixedBodyHeight = 1
	}
	fixedBody := lipgloss.NewStyle().Height(fixedBodyHeight).MaxHeight(fixedBodyHeight).Render(body)
	statusBar := m.renderStatusBar()

	parts := []string{header, tabs}
	if m.searchBarHeight() == 1 {
		parts = append(parts, m.renderSearchBar())
	}
	parts = append(parts, fixedBody, statusBar)
	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

// ── Chrome renderers ──────────────────────────────────────────────────────────

func (m Model) renderHeader() string {
	name := m.result.Stats.RepoName
	if name == "" {
		name = "…"
	}
	return styleAccent.Render("⎇  repoview") +
		styleDim.Render("  /  ") +
		styleAccent.Render(name)
}

func (m Model) renderTabs(pw int) string {
	var tabs []string
	for i := Tab(0); i < tabCount; i++ {
		if i == m.activeTab {
			tabs = append(tabs, styleActiveTab.Render(tabNames[i]))
		} else {
			tabs = append(tabs, styleTab.Render(tabNames[i]))
		}
	}
	row := lipgloss.JoinHorizontal(lipgloss.Bottom, tabs...)

	gapWidth := pw - lipgloss.Width(row)
	if gapWidth < 0 {
		gapWidth = 0
	}
	gap := lipgloss.NewStyle().
		BorderStyle(inactiveTabBorder).
		BorderBottom(true).
		BorderForeground(colorSubtle).
		Render(strings.Repeat(" ", gapWidth))

	return lipgloss.JoinHorizontal(lipgloss.Bottom, row, gap)
}

func (m Model) renderStatusBar() string {
	pw := m.panelWidth()
	repoName := m.result.Stats.RepoName
	if repoName == "" {
		repoName = "no repo"
	}
	tabLabel := strings.TrimSpace(tabNames[m.activeTab])

	pill := statusPill.Render("repoview")
	right := statusRightPill.Render(repoName)
	descW := pw - lipgloss.Width(pill) - lipgloss.Width(right)
	if descW < 0 {
		descW = 0
	}

	var middleText string
	if m.flashMsg != "" {
		middleText = "  " + m.flashMsg
	} else {
		base := "  " + tabLabel + "   ←/→ tabs  ↑/↓ scroll  r refresh  Esc back  q quit"
		switch m.activeTab {
		case TabChurn, TabTodos, TabStale:
			middleText = base + "   / filter  o open  y copy"
		default:
			middleText = base
		}
	}

	desc := statusBarBg.Width(descW).Render(middleText)
	bar := lipgloss.JoinHorizontal(lipgloss.Top, pill, desc, right)
	return statusBarBg.Width(pw).Render(bar)
}

func (m Model) renderSearchBar() string {
	return styleAccent.Render("🔍 ") +
		m.searchInput.View() +
		styleDim.Render("  Esc clear · Enter confirm")
}

// ── Table helper ──────────────────────────────────────────────────────────────

// newTable builds a lipgloss table with consistent app-wide styling.
// selectedInView is the cursor's 0-based position within the visible window.
func (m Model) newTable(selectedInView int, headers []string, rows [][]string) string {
	inner := m.panelWidth() - 2 // 2 chars reserved for left/right border
	if inner < 10 {
		inner = 10
	}

	t := ltable.New().
		Border(lipgloss.NormalBorder()).
		BorderTop(false).
		BorderBottom(false).
		BorderLeft(false).
		BorderRight(false).
		BorderColumn(false).
		BorderHeader(true).
		BorderStyle(lipgloss.NewStyle().Foreground(colorSubtle)).
		Width(inner).
		StyleFunc(func(row, col int) lipgloss.Style {
			switch {
			case row == ltable.HeaderRow:
				return tableHeader
			case row == selectedInView:
				return tableSelected
			default:
				return tableCell
			}
		}).
		Headers(headers...).
		Rows(rows...)

	return lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(colorSubtle).
		Width(inner).
		Render(t.Render())
}
