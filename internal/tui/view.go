package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
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
	titleStyle := func() lipgloss.Style {
		b := lipgloss.RoundedBorder()
		b.Right = "├"
		return lipgloss.NewStyle().BorderStyle(b).Padding(0, 1).Foreground(colorBlue).Bold(true)
	}()
	infoStyle := func() lipgloss.Style {
		b := lipgloss.RoundedBorder()
		b.Left = "┤"
		return lipgloss.NewStyle().BorderStyle(b).Padding(0, 1).Foreground(colorGray)
	}()

	title := titleStyle.Render(m.viewerTitle)
	headerLine := strings.Repeat("─", max(0, m.width-lipgloss.Width(title)))
	header := lipgloss.JoinHorizontal(lipgloss.Center, title, styleLabel.Render(headerLine))

	pct := infoStyle.Render(fmt.Sprintf("%3.f%%", m.viewer.ScrollPercent()*100))
	hints := styleDim.Render("↑/↓  PgUp/PgDn  q/Esc close")
	footerLine := strings.Repeat("─", max(0, m.width-lipgloss.Width(pct)-lipgloss.Width(hints)-2))
	footer := lipgloss.JoinHorizontal(lipgloss.Center,
		"  "+hints,
		styleLabel.Render(footerLine),
		pct,
	)

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
		case TabBranches:
			body = m.renderBranches()
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
	if m.flashMsg != "" {
		return statusBarBg.Width(m.panelWidth()).Padding(0, 1).Render(m.flashMsg)
	}
	showFileOps := m.activeTab == TabChurn || m.activeTab == TabTodos || m.activeTab == TabStale
	helpView := m.help.View(mainKeyMap{showFileOps: showFileOps})
	return statusBarBg.Width(m.panelWidth()).Padding(0, 1).Render(helpView)
}

func (m Model) renderSearchBar() string {
	return styleAccent.Render("🔍 ") +
		m.searchInput.View() +
		styleDim.Render("  Esc clear · Enter confirm")
}

// ── Table helper ──────────────────────────────────────────────────────────────

func (m Model) renderTable() string {
	return m.tbl.View()
}
