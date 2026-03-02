// Package tui implements the Bubble Tea terminal UI for repoview.
package tui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/connerluzier/repoview/internal/git_analysis"
	"github.com/connerluzier/repoview/internal/metrics"
	"github.com/connerluzier/repoview/internal/utils"
)

// ── Palette ──────────────────────────────────────────────────────────────────

var (
	colorBlue   = lipgloss.AdaptiveColor{Light: "#347aeb", Dark: "#347aeb"}
	colorRed    = lipgloss.AdaptiveColor{Light: "#f54242", Dark: "#f54242"}
	colorYellow = lipgloss.AdaptiveColor{Light: "#b0ad09", Dark: "#e0d44f"}
	colorGreen  = lipgloss.AdaptiveColor{Light: "#1fb009", Dark: "#3fd020"}
	colorGray   = lipgloss.AdaptiveColor{Light: "#636363", Dark: "#888888"}
	colorWhite  = lipgloss.Color("#FFFDF5")
	colorSubtle = lipgloss.AdaptiveColor{Light: "#D9DCCF", Dark: "#383838"}
	colorText   = lipgloss.AdaptiveColor{Light: "#343433", Dark: "#C1C6B2"}
	colorSurface = lipgloss.AdaptiveColor{Light: "#D9DCCF", Dark: "#353533"}

	// keep these for backward compat with input screen
	cPrimary = lipgloss.Color("#347aeb")
	cMuted   = lipgloss.Color("#636363")
	cSubtext = lipgloss.Color("#888888")
	cBorder  = lipgloss.AdaptiveColor{Light: "#D9DCCF", Dark: "#383838"}
)

// ── Shared styles ─────────────────────────────────────────────────────────────

var (
	styleBold    = lipgloss.NewStyle().Bold(true)
	styleLabel   = lipgloss.NewStyle().Foreground(colorGray)
	styleValue   = lipgloss.NewStyle().Foreground(colorWhite).Bold(true)
	styleDim     = lipgloss.NewStyle().Foreground(colorSubtle)
	styleDanger  = lipgloss.NewStyle().Foreground(colorRed).Bold(true)
	styleWarning = lipgloss.NewStyle().Foreground(colorYellow)
	styleSuccess = lipgloss.NewStyle().Foreground(colorGreen)
	styleAccent  = lipgloss.NewStyle().Foreground(colorBlue).Bold(true)
	stylePrimary = lipgloss.NewStyle().Foreground(colorBlue).Bold(true)

	styleSelected = lipgloss.NewStyle().Foreground(colorBlue).Bold(true)

	// ── Tab borders (from charm.sh article) ──────────────────────────────────

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

	styleHelp = lipgloss.NewStyle().Foreground(colorGray).PaddingTop(1)
)

// ── Banner ───────────────────────────────────────────────────────────────────

var banner = `
 ██████╗ ███████╗██████╗  ██████╗ ██╗   ██╗██╗███████╗██╗    ██╗
 ██╔══██╗██╔════╝██╔══██╗██╔═══██╗██║   ██║██║██╔════╝██║    ██║
 ██████╔╝█████╗  ██████╔╝██║   ██║██║   ██║██║█████╗  ██║ █╗ ██║
 ██╔══██╗██╔══╝  ██╔═══╝ ██║   ██║╚██╗ ██╔╝██║██╔══╝  ██║███╗██║
 ██║  ██║███████╗██║     ╚██████╔╝ ╚████╔╝ ██║███████╗╚███╔███╔╝
 ╚═╝  ╚═╝╚══════╝╚═╝      ╚═════╝   ╚═══╝  ╚═╝╚══════╝ ╚══╝╚══╝ `

// ── App state ─────────────────────────────────────────────────────────────────

type appState int

const (
	stateInput    appState = iota
	stateLoading
	stateMain
)

// ── Tabs ──────────────────────────────────────────────────────────────────────

type Tab int

const (
	TabOverview Tab = iota
	TabHotspots
	TabChurn
	TabActivity
	TabTodos
	tabCount
)

var tabNames = [tabCount]string{"  Overview  ", "  Hotspots  ", "  Churn  ", "  Activity  ", "  Todos  "}

// ── Messages ──────────────────────────────────────────────────────────────────

type AnalysisDoneMsg struct {
	Result git_analysis.AnalysisResult
	Risks  []metrics.RiskEntry
	Todos  metrics.TodoSummary
}

type cloneDoneMsg struct {
	path string
	err  error
}

type RefreshMsg struct{}

// ── Model ─────────────────────────────────────────────────────────────────────

type Model struct {
	state      appState
	input      textinput.Model
	spinner    spinner.Model
	loadingMsg string
	tmpDir     string
	inputErr   string

	repoPath    string
	activeTab   Tab
	loading     bool
	err         error
	result      git_analysis.AnalysisResult
	risks       []metrics.RiskEntry
	todos       metrics.TodoSummary
	cursor      int
	width       int
	height      int
	scrollOffset int
}

func New() Model {
	ti := textinput.New()
	ti.Placeholder = "~/projects/myrepo  or  https://github.com/owner/repo"
	ti.PlaceholderStyle = lipgloss.NewStyle().Foreground(cMuted)
	ti.TextStyle = lipgloss.NewStyle().Foreground(colorWhite)
	ti.Cursor.Style = lipgloss.NewStyle().Foreground(cPrimary)
	ti.Width = 60
	ti.Focus()

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(cPrimary)

	return Model{
		state:   stateInput,
		input:   ti,
		spinner: sp,
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, m.spinner.Tick)
}

// ── Update ────────────────────────────────────────────────────────────────────

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case cloneDoneMsg:
		if msg.err != nil {
			m.state = stateInput
			m.inputErr = fmt.Sprintf("Clone failed: %v", msg.err)
			return m, nil
		}
		m.tmpDir = msg.path
		m.loadingMsg = "Analyzing repository…"
		return m, runAnalysis(msg.path)

	case AnalysisDoneMsg:
		m.loading = false
		m.state = stateMain
		m.result = msg.Result
		m.risks = msg.Risks
		m.todos = msg.Todos
		m.err = msg.Result.Error
		m.cursor = 0
		m.scrollOffset = 0

	case RefreshMsg:
		m.loading = true
		m.cursor = 0
		m.scrollOffset = 0
		return m, runAnalysis(m.repoPath)

	case tea.KeyMsg:
		switch m.state {
		case stateInput:
			return m.updateInput(msg)
		case stateMain:
			return m.updateMain(msg)
		}
	}

	if m.state == stateInput {
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m Model) updateInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEnter:
		raw := strings.TrimSpace(m.input.Value())
		if raw == "" {
			m.inputErr = "Please enter a path or URL."
			return m, nil
		}
		m.inputErr = ""
		m.state = stateLoading
		if isRemoteURL(raw) {
			m.loadingMsg = "Cloning repository…"
			return m, cloneRepo(raw)
		}
		abs, err := filepath.Abs(raw)
		if err != nil {
			m.inputErr = fmt.Sprintf("Invalid path: %v", err)
			m.state = stateInput
			return m, nil
		}
		m.repoPath = abs
		m.loadingMsg = "Analyzing repository…"
		return m, runAnalysis(abs)

	case tea.KeyCtrlC, tea.KeyEsc:
		return m, tea.Quit
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m Model) updateMain(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		if m.tmpDir != "" {
			os.RemoveAll(m.tmpDir)
		}
		return m, tea.Quit

	case "backspace", "esc":
		// go back to input screen
		if m.tmpDir != "" {
			os.RemoveAll(m.tmpDir)
			m.tmpDir = ""
		}
		m.state = stateInput
		m.input.SetValue("")
		m.inputErr = ""
		m.result = git_analysis.AnalysisResult{}
		m.risks = nil
		m.todos = metrics.TodoSummary{}
		m.err = nil
		return m, textinput.Blink

	case "r":
		return m.Update(RefreshMsg{})

	case "left", "h":
		if m.activeTab > 0 {
			m.activeTab--
			m.cursor = 0
			m.scrollOffset = 0
		}

	case "right", "l":
		if m.activeTab < tabCount-1 {
			m.activeTab++
			m.cursor = 0
			m.scrollOffset = 0
		}

	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
		m.clampScroll()

	case "down", "j":
		m.cursor++
		m.clampCursor()
		m.clampScroll()

	case "g":
		m.cursor = 0
		m.scrollOffset = 0

	case "G":
		m.cursor = m.listLen() - 1
		m.clampCursor()
		m.clampScroll()

	case "tab":
		m.activeTab = (m.activeTab + 1) % tabCount
		m.cursor = 0
		m.scrollOffset = 0
	}
	return m, nil
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func isRemoteURL(s string) bool {
	return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://") || strings.HasPrefix(s, "git@")
}

func cloneRepo(url string) tea.Cmd {
	return func() tea.Msg {
		tmp, err := os.MkdirTemp("", "repoview-*")
		if err != nil {
			return cloneDoneMsg{err: err}
		}
		cmd := exec.Command("git", "clone", "--depth=200", url, tmp)
		if err := cmd.Run(); err != nil {
			os.RemoveAll(tmp)
			return cloneDoneMsg{err: err}
		}
		return cloneDoneMsg{path: tmp}
	}
}

func runAnalysis(repoPath string) tea.Cmd {
	return func() tea.Msg {
		result := git_analysis.Analyze(repoPath)
		if result.Error != nil {
			return AnalysisDoneMsg{Result: result}
		}
		risks := metrics.ComputeRiskScores(result.FileChurns)
		todos := metrics.ScanTodos(repoPath)
		return AnalysisDoneMsg{Result: result, Risks: risks, Todos: todos}
	}
}

func (m *Model) listLen() int {
	switch m.activeTab {
	case TabHotspots:
		if len(m.risks) > 20 {
			return 20
		}
		return len(m.risks)
	case TabChurn:
		if len(m.result.FileChurns) > 10 {
			return 10
		}
		return len(m.result.FileChurns)
	case TabActivity:
		return len(m.result.ContributorActivity)
	case TabTodos:
		return len(m.todos.Items)
	}
	return 0
}

func (m *Model) clampCursor() {
	l := m.listLen()
	if l == 0 {
		m.cursor = 0
		return
	}
	if m.cursor >= l {
		m.cursor = l - 1
	}
}

func (m *Model) clampScroll() {
	visibleRows := m.bodyHeight()
	if m.cursor < m.scrollOffset {
		m.scrollOffset = m.cursor
	}
	if m.cursor >= m.scrollOffset+visibleRows {
		m.scrollOffset = m.cursor - visibleRows + 1
	}
}

// panelWidth is the usable content width (full terminal width).
func (m Model) panelWidth() int {
	w := m.width
	if w < 40 {
		w = 40
	}
	return w
}

// bodyHeight is the scrollable rows between the tab bar and status bar.
func (m Model) bodyHeight() int {
	// header(1) + tab bar with custom borders(3) + status bar(1) = 5 overhead
	h := m.height - 5
	if h < 5 {
		h = 5
	}
	return h
}

// ── View ──────────────────────────────────────────────────────────────────────

func (m Model) View() string {
	if m.width == 0 {
		return ""
	}
	switch m.state {
	case stateInput:
		return m.viewInput()
	case stateLoading:
		return m.viewLoading()
	default:
		return m.viewMain()
	}
}

// ── Input screen ─────────────────────────────────────────────────────────────

func (m Model) viewInput() string {
	bannerStyle := lipgloss.NewStyle().Foreground(cPrimary).Bold(true)
	subtitle := lipgloss.NewStyle().Foreground(cSubtext).Render("Git repository analyzer  ·  local paths & GitHub URLs")

	inputBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(cPrimary).
		Padding(0, 1).
		Width(64).
		Render(m.input.View())

	var errLine string
	if m.inputErr != "" {
		errLine = "\n" + styleDanger.Render("  ✖  "+m.inputErr)
	}

	hint := styleDim.Render("Enter a local path or GitHub URL, then press Enter")
	esc := styleDim.Render("Ctrl+C / Esc to quit")

	content := lipgloss.JoinVertical(lipgloss.Center,
		bannerStyle.Render(banner),
		"",
		subtitle,
		"",
		"",
		inputBox,
		errLine,
		"",
		hint,
		esc,
	)

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
}

// ── Loading screen ───────────────────────────────────────────────────────────

func (m Model) viewLoading() string {
	msg := lipgloss.JoinHorizontal(lipgloss.Center,
		m.spinner.View()+" ",
		styleValue.Render(m.loadingMsg),
	)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, msg)
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
		case TabHotspots:
			body = m.renderHotspots()
		case TabChurn:
			body = m.renderChurn()
		case TabActivity:
			body = m.renderActivity()
		case TabTodos:
			body = m.renderTodos()
		}
	}

	// Pin the body to a fixed height so the status bar never drifts
	fixedBody := lipgloss.NewStyle().Height(m.bodyHeight()).MaxHeight(m.bodyHeight()).Render(body)
	statusBar := m.renderStatusBar()

	return lipgloss.JoinVertical(lipgloss.Left, header, tabs, fixedBody, statusBar)
}

func (m Model) renderHeader() string {
	name := m.result.Stats.RepoName
	if name == "" {
		name = "…"
	}
	left := lipgloss.NewStyle().Foreground(cPrimary).Bold(true).Render("⎇  repoview")
	sep := styleDim.Render("  /  ")
	right := styleAccent.Render(name)
	return left + sep + right
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

	// fill the remaining width with a bottom-border-only gap (article technique)
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
	keys := lipgloss.NewStyle().Foreground(colorText).Background(colorSurface).Bold(false).
		Render("  " + tabLabel + "   ←/→ tabs  ↑/↓ scroll  r refresh  Esc back  q quit")
	desc := statusBarBg.Width(descW).Render(keys)

	bar := lipgloss.JoinHorizontal(lipgloss.Top, pill, desc, right)
	return statusBarBg.Width(pw).Render(bar)
}

// ── Overview ─────────────────────────────────────────────────────────────────

func (m Model) renderOverview() string {
	s := m.result.Stats
	pw := m.panelWidth()

	kv := func(icon, label, value string) string {
		ic := styleDim.Render(icon)
		lb := styleLabel.Render(utils.PadRight(label, 20))
		vl := styleValue.Render(value)
		return "  " + ic + "  " + lb + vl
	}

	lines := []string{
		"",
		kv("📁", "Repository", s.RepoName),
		kv("📍", "Path", utils.Truncate(s.RepoPath, pw-30)),
		"",
		kv("📝", "Total Commits", fmt.Sprintf("%d", s.TotalCommits)),
		kv("👥", "Contributors", fmt.Sprintf("%d", s.TotalContributors)),
		kv("🌿", "Branches", fmt.Sprintf("%d", s.TotalBranches)),
		kv("🏷 ", "Tags", fmt.Sprintf("%d", s.TotalTags)),
		kv("💾", "Approx. Size", utils.HumanBytes(s.RepoSizeBytes)),
	}

	if s.LatestCommit != nil {
		lc := s.LatestCommit
		divider := "  " + styleDim.Render(strings.Repeat("─", pw-4))
		lines = append(lines, "", divider,
			kv("🔖", "Hash", lc.Hash),
			kv("✍️ ", "Author", lc.Author),
			kv("🕐", "When", utils.TimeAgo(lc.When)),
			kv("💬", "Message", utils.Truncate(lc.Message, pw-30)),
		)
	}
	return strings.Join(lines, "\n")
}

// ── Hotspots ──────────────────────────────────────────────────────────────────

func (m Model) renderHotspots() string {
	if len(m.risks) == 0 {
		return styleDim.Render("\n  No data available.")
	}
	top := m.risks
	if len(top) > 20 {
		top = top[:20]
	}
	maxScore := top[0].Score
	barWidth := 20

	var sb strings.Builder
	sb.WriteString("\n")
	sb.WriteString(styleLabel.Render(fmt.Sprintf("  %-50s  %8s  %7s  %7s  %s\n",
		"File", "Score", "Commits", "Authors", "Risk")))
	sb.WriteString(styleDim.Render("  " + strings.Repeat("─", m.panelWidth()-4) + "\n"))

	visibleRows := m.bodyHeight()

	for i, r := range top {
		if i < m.scrollOffset || i >= m.scrollOffset+visibleRows {
			continue
		}
		bar := utils.Heatmap(int(r.Score*10), int(maxScore*10), barWidth)
		bonus := ""
		if r.RecentBonus {
			bonus = styleWarning.Render("*")
		}
		scoreStr := fmt.Sprintf("%.1f", r.Score) + bonus

		var scoreStyle lipgloss.Style
		switch {
		case r.Score >= maxScore*0.75:
			scoreStyle = styleDanger
		case r.Score >= maxScore*0.4:
			scoreStyle = styleWarning
		default:
			scoreStyle = styleSuccess
		}

		prefix := "  "
		if i == m.cursor {
			prefix = styleAccent.Render("▶ ")
		}

		filePart := utils.Truncate(r.Path, 50)
		row := fmt.Sprintf("%s%-50s  %8s  %7d  %7d  %s",
			prefix, filePart, scoreStyle.Render(scoreStr), r.CommitCount, r.Authors, bar)

		if i == m.cursor {
			sb.WriteString(styleSelected.Render(row) + "\n")
		} else {
			sb.WriteString(row + "\n")
		}
	}
	sb.WriteString(styleDim.Render("\n  * recently modified (score ×1.2)"))
	return sb.String()
}

// ── Churn ────────────────────────────────────────────────────────────────────

func (m Model) renderChurn() string {
	if len(m.result.FileChurns) == 0 {
		return styleDim.Render("\n  No data available.")
	}
	top := m.result.FileChurns
	if len(top) > 10 {
		top = top[:10]
	}
	maxCommits := top[0].CommitCount
	barWidth := 25

	var sb strings.Builder
	sb.WriteString("\n")
	sb.WriteString(styleLabel.Render(fmt.Sprintf("  %-50s  %7s  %7s  %-12s  %s\n",
		"File", "Commits", "Authors", "Last Modified", "Churn")))
	sb.WriteString(styleDim.Render("  " + strings.Repeat("─", m.panelWidth()-4) + "\n"))

	for i, f := range top {
		bar := utils.Heatmap(f.CommitCount, maxCommits, barWidth)
		prefix := "  "
		if i == m.cursor {
			prefix = styleAccent.Render("▶ ")
		}
		row := fmt.Sprintf("%s%-50s  %7d  %7d  %-12s  %s",
			prefix,
			utils.Truncate(f.Path, 50),
			f.CommitCount,
			f.UniqueAuthors,
			utils.TimeAgo(f.LastModified),
			bar,
		)
		if i == m.cursor {
			sb.WriteString(styleSelected.Render(row) + "\n")
		} else {
			sb.WriteString(row + "\n")
		}
	}
	return sb.String()
}

// ── Activity ──────────────────────────────────────────────────────────────────

func (m Model) renderActivity() string {
	var sb strings.Builder
	sb.WriteString("\n")
	sb.WriteString(stylePrimary.Render("  Commits — last 30 days") + "\n\n")

	daily := m.result.DailyActivity
	maxDay := 0
	for _, d := range daily {
		if d.Count > maxDay {
			maxDay = d.Count
		}
	}

	sb.WriteString("  ")
	for _, d := range daily {
		sb.WriteString(activityBlock(d.Count, maxDay))
	}
	sb.WriteString("\n  ")
	if len(daily) > 0 {
		sb.WriteString(styleDim.Render(daily[0].Date.Format("Jan 2")))
		sb.WriteString(strings.Repeat(" ", 28))
		sb.WriteString(styleDim.Render(daily[len(daily)-1].Date.Format("Jan 2")))
	}
	sb.WriteString("\n")

	contribs := m.result.ContributorActivity
	if len(contribs) == 0 {
		return sb.String()
	}
	sb.WriteString("\n")
	sb.WriteString(stylePrimary.Render("  Contributors") + "\n")
	sb.WriteString(styleDim.Render("  " + strings.Repeat("─", m.panelWidth()-4) + "\n"))
	sb.WriteString(styleLabel.Render(fmt.Sprintf("  %-30s  %8s  %7s  %s\n", "Name", "Commits", "Share", "Bar")))
	sb.WriteString(styleDim.Render("  " + strings.Repeat("─", m.panelWidth()-4) + "\n"))

	total := 0
	for _, c := range contribs {
		total += c.Count
	}
	visibleRows := m.bodyHeight() - 10
	if visibleRows < 5 {
		visibleRows = 5
	}
	for i, c := range contribs {
		if i < m.scrollOffset || i >= m.scrollOffset+visibleRows {
			continue
		}
		pct := 0.0
		if total > 0 {
			pct = float64(c.Count) / float64(total) * 100
		}
		bar := utils.Heatmap(c.Count, contribs[0].Count, 20)
		prefix := "  "
		if i == m.cursor {
			prefix = styleAccent.Render("▶ ")
		}
		row := fmt.Sprintf("%s%-30s  %8d  %6.1f%%  %s", prefix, utils.Truncate(c.Name, 30), c.Count, pct, bar)
		if i == m.cursor {
			sb.WriteString(styleSelected.Render(row) + "\n")
		} else {
			sb.WriteString(row + "\n")
		}
	}
	return sb.String()
}

func activityBlock(count, max int) string {
	if max == 0 {
		return styleDim.Render("░")
	}
	ratio := float64(count) / float64(max)
	switch {
	case ratio == 0:
		return styleDim.Render("░")
	case ratio < 0.25:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("25")).Render("▒")
	case ratio < 0.5:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("33")).Render("▓")
	case ratio < 0.75:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Render("█")
	default:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("75")).Render("█")
	}
}

// ── Todos ────────────────────────────────────────────────────────────────────

func (m Model) renderTodos() string {
	summary := m.todos
	var sb strings.Builder
	sb.WriteString("\n")

	sb.WriteString("  ")
	for _, kw := range []string{"TODO", "FIXME", "HACK", "XXX"} {
		count := summary.CountByKind[kw]
		var style lipgloss.Style
		switch kw {
		case "FIXME":
			style = lipgloss.NewStyle().Foreground(colorWhite).Background(lipgloss.Color("#f54242")).Bold(true).Padding(0, 1)
		case "HACK":
			style = lipgloss.NewStyle().Foreground(colorWhite).Background(lipgloss.Color("#b0ad09")).Bold(true).Padding(0, 1)
		default:
			style = lipgloss.NewStyle().Foreground(colorWhite).Background(lipgloss.Color("#636363")).Bold(true).Padding(0, 1)
		}
		sb.WriteString(style.Render(fmt.Sprintf("%s %d", kw, count)) + "  ")
	}
	sb.WriteString(styleValue.Render(fmt.Sprintf("Total: %d", summary.TotalCount)))
	sb.WriteString("\n\n")

	if summary.TotalCount == 0 {
		sb.WriteString(styleSuccess.Render("  ✓  No TODOs found — clean codebase!\n"))
		return sb.String()
	}

	sb.WriteString(styleLabel.Render(fmt.Sprintf("  %-5s  %-6s  %-45s  %s\n", "Line", "Kind", "File", "Text")))
	sb.WriteString(styleDim.Render("  " + strings.Repeat("─", m.panelWidth()-4) + "\n"))

	visibleRows := m.bodyHeight()
	if visibleRows < 5 {
		visibleRows = 5
	}
	for i, item := range summary.Items {
		if i < m.scrollOffset || i >= m.scrollOffset+visibleRows {
			continue
		}
		var kindStyle lipgloss.Style
		switch item.Kind {
		case "FIXME":
			kindStyle = styleDanger
		case "HACK", "XXX":
			kindStyle = styleWarning
		default:
			kindStyle = styleLabel
		}
		prefix := "  "
		if i == m.cursor {
			prefix = styleAccent.Render("▶ ")
		}
		row := fmt.Sprintf("%s%-5d  %s  %-45s  %s",
			prefix,
			item.Line,
			kindStyle.Render(utils.PadRight(item.Kind, 6)),
			utils.Truncate(item.File, 45),
			utils.Truncate(item.Text, m.panelWidth()-70),
		)
		if i == m.cursor {
			sb.WriteString(styleSelected.Render(row) + "\n")
		} else {
			sb.WriteString(row + "\n")
		}
	}
	return sb.String()
}
