// Package tui implements the Bubble Tea terminal UI for repoview.
package tui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/atotto/clipboard"
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
	TabStale
	tabCount
)

var tabNames = [tabCount]string{"  Overview  ", "  Hotspots  ", "  Churn  ", "  Activity  ", "  Todos  ", "  Stale  "}

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

type editorClosedMsg struct{ err error }
type flashClearMsg struct{}

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

	searchMode  bool
	searchQuery string
	searchInput textinput.Model
	flashMsg    string
}

func New() Model {
	ti := textinput.New()
	ti.Placeholder = "~/projects/myrepo  or  https://github.com/owner/repo"
	ti.PlaceholderStyle = lipgloss.NewStyle().Foreground(cMuted)
	ti.TextStyle = lipgloss.NewStyle().Foreground(colorWhite)
	ti.Cursor.Style = lipgloss.NewStyle().Foreground(cPrimary)
	ti.Width = 60
	ti.Focus()

	si := textinput.New()
	si.PlaceholderStyle = lipgloss.NewStyle().Foreground(cMuted)
	si.TextStyle = lipgloss.NewStyle().Foreground(colorWhite)
	si.Cursor.Style = lipgloss.NewStyle().Foreground(cPrimary)
	si.Width = 40

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(cPrimary)

	return Model{
		state:       stateInput,
		input:       ti,
		spinner:     sp,
		searchInput: si,
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

	case editorClosedMsg:
		// no-op: just resume the TUI after the editor exits

	case flashClearMsg:
		m.flashMsg = ""

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
	// If in search mode, handle search keys first
	if m.searchMode {
		switch msg.Type {
		case tea.KeyEsc:
			m.searchMode = false
			m.searchQuery = ""
			m.searchInput.SetValue("")
			m.cursor = 0
			m.scrollOffset = 0
			return m, nil
		case tea.KeyEnter:
			m.searchMode = false
			return m, nil
		default:
			var cmd tea.Cmd
			m.searchInput, cmd = m.searchInput.Update(msg)
			m.searchQuery = m.searchInput.Value()
			m.cursor = 0
			m.scrollOffset = 0
			return m, cmd
		}
	}

	switch msg.String() {
	case "q", "ctrl+c":
		if m.tmpDir != "" {
			os.RemoveAll(m.tmpDir)
		}
		return m, tea.Quit

	case "backspace", "esc":
		// If a filter is active, clear it first
		if m.searchQuery != "" {
			m.searchQuery = ""
			m.searchInput.SetValue("")
			m.cursor = 0
			m.scrollOffset = 0
			return m, nil
		}
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

	case "/":
		// Enter search mode on applicable tabs
		switch m.activeTab {
		case TabHotspots, TabChurn, TabTodos, TabStale:
			m.searchMode = true
			m.searchInput.SetValue(m.searchQuery)
			m.searchInput.Focus()
			return m, nil
		}

	case "r":
		return m.Update(RefreshMsg{})

	case "left", "h":
		if m.activeTab > 0 {
			m.activeTab--
			m.cursor = 0
			m.scrollOffset = 0
			m.searchMode = false
			m.searchQuery = ""
			m.searchInput.SetValue("")
		}

	case "right", "l":
		if m.activeTab < tabCount-1 {
			m.activeTab++
			m.cursor = 0
			m.scrollOffset = 0
			m.searchMode = false
			m.searchQuery = ""
			m.searchInput.SetValue("")
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
		m.searchMode = false
		m.searchQuery = ""
		m.searchInput.SetValue("")

	case "enter", "o":
		// Open current file in $EDITOR
		path := m.currentFilePath()
		if path == "" {
			return m, nil
		}
		fullPath := filepath.Join(m.repoPath, path)
		editor := os.Getenv("EDITOR")
		if editor == "" {
			editor = os.Getenv("VISUAL")
		}
		if editor == "" {
			editor = "vi"
		}
		var cmd *exec.Cmd
		line := m.currentFileLine()
		if line > 0 {
			cmd = exec.Command(editor, fmt.Sprintf("+%d", line), fullPath)
		} else {
			cmd = exec.Command(editor, fullPath)
		}
		return m, tea.ExecProcess(cmd, func(err error) tea.Msg {
			return editorClosedMsg{err: err}
		})

	case "y":
		// Copy file path to clipboard
		path := m.currentFilePath()
		if path == "" {
			return m, nil
		}
		fullPath := filepath.Join(m.repoPath, path)
		if err := clipboard.WriteAll(fullPath); err == nil {
			m.flashMsg = "📋 Copied: " + fullPath
		} else {
			m.flashMsg = "✖ Clipboard error: " + err.Error()
		}
		return m, tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
			return flashClearMsg{}
		})
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
		return len(m.filteredHotspots())
	case TabChurn:
		return len(m.filteredChurns())
	case TabActivity:
		return len(m.result.ContributorActivity)
	case TabTodos:
		return len(m.filteredTodos())
	case TabStale:
		return len(m.filteredStale())
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

// searchBarHeight returns 1 if the search bar is visible, 0 otherwise.
func (m Model) searchBarHeight() int {
	switch m.activeTab {
	case TabHotspots, TabChurn, TabTodos, TabStale:
		if m.searchMode || m.searchQuery != "" {
			return 1
		}
	}
	return 0
}

// filteredHotspots returns the top 20 risks filtered by searchQuery.
func (m Model) filteredHotspots() []metrics.RiskEntry {
	top := m.risks
	if len(top) > 20 {
		top = top[:20]
	}
	if m.searchQuery == "" {
		return top
	}
	q := strings.ToLower(m.searchQuery)
	var out []metrics.RiskEntry
	for _, r := range top {
		if strings.Contains(strings.ToLower(r.Path), q) {
			out = append(out, r)
		}
	}
	return out
}

// filteredChurns returns the top 10 churns filtered by searchQuery.
func (m Model) filteredChurns() []git_analysis.FileChurn {
	top := m.result.FileChurns
	if len(top) > 10 {
		top = top[:10]
	}
	if m.searchQuery == "" {
		return top
	}
	q := strings.ToLower(m.searchQuery)
	var out []git_analysis.FileChurn
	for _, f := range top {
		if strings.Contains(strings.ToLower(f.Path), q) {
			out = append(out, f)
		}
	}
	return out
}

// filteredTodos returns todo items filtered by searchQuery.
func (m Model) filteredTodos() []metrics.TodoItem {
	items := m.todos.Items
	if m.searchQuery == "" {
		return items
	}
	q := strings.ToLower(m.searchQuery)
	var out []metrics.TodoItem
	for _, item := range items {
		if strings.Contains(strings.ToLower(item.File), q) {
			out = append(out, item)
		}
	}
	return out
}

// filteredStale returns stale files filtered by searchQuery.
func (m Model) filteredStale() []git_analysis.FileChurn {
	items := m.result.StaleFiles
	if m.searchQuery == "" {
		return items
	}
	q := strings.ToLower(m.searchQuery)
	var out []git_analysis.FileChurn
	for _, f := range items {
		if strings.Contains(strings.ToLower(f.Path), q) {
			out = append(out, f)
		}
	}
	return out
}

// currentFilePath returns the relative file path for the selected item on the current tab.
func (m Model) currentFilePath() string {
	switch m.activeTab {
	case TabHotspots:
		items := m.filteredHotspots()
		if m.cursor < len(items) {
			return items[m.cursor].Path
		}
	case TabChurn:
		items := m.filteredChurns()
		if m.cursor < len(items) {
			return items[m.cursor].Path
		}
	case TabStale:
		items := m.filteredStale()
		if m.cursor < len(items) {
			return items[m.cursor].Path
		}
	case TabTodos:
		items := m.filteredTodos()
		if m.cursor < len(items) {
			return items[m.cursor].File
		}
	}
	return ""
}

// currentFileLine returns the line number for the selected item (only non-zero on TabTodos).
func (m Model) currentFileLine() int {
	if m.activeTab != TabTodos {
		return 0
	}
	items := m.filteredTodos()
	if m.cursor < len(items) {
		return items[m.cursor].Line
	}
	return 0
}

// renderSearchBar renders the search bar.
func (m Model) renderSearchBar() string {
	prefix := styleAccent.Render("🔍 ")
	hint := styleDim.Render("  Esc clear · Enter confirm")
	return prefix + m.searchInput.View() + hint
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
		case TabStale:
			body = m.renderStale()
		}
	}

	// Pin the body to a fixed height so the status bar never drifts
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

	var middleText string
	if m.flashMsg != "" {
		middleText = "  " + m.flashMsg
	} else {
		base := "  " + tabLabel + "   ←/→ tabs  ↑/↓ scroll  r refresh  Esc back  q quit"
		switch m.activeTab {
		case TabHotspots, TabChurn, TabTodos, TabStale:
			middleText = base + "   / filter  o open  y copy"
		default:
			middleText = base
		}
	}
	keys := lipgloss.NewStyle().Foreground(colorText).Background(colorSurface).Bold(false).Render(middleText)
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
	top := m.filteredHotspots()
	if len(top) == 0 {
		return styleDim.Render("\n  No results match your filter.")
	}
	maxScore := top[0].Score
	barWidth := 20

	var sb strings.Builder
	sb.WriteString("\n")
	sb.WriteString(styleLabel.Render(fmt.Sprintf("  %-50s  %8s  %7s  %7s  %s\n",
		"File", "Score", "Commits", "Authors", "Risk")))
	sb.WriteString(styleDim.Render("  " + strings.Repeat("─", m.panelWidth()-4) + "\n"))

	// Reserve space for the detail panel (separator + 3 content lines + footnote)
	visibleRows := m.bodyHeight() - 8 - m.searchBarHeight()
	if visibleRows < 3 {
		visibleRows = 3
	}

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

	// ── Detail panel for selected entry ───────────────────────────────────────
	sb.WriteString(styleDim.Render("\n  " + strings.Repeat("─", m.panelWidth()-4) + "\n"))
	if m.cursor < len(top) {
		r := top[m.cursor]
		sb.WriteString(m.renderHotspotDetail(r, maxScore))
	}
	sb.WriteString(styleDim.Render("  score = commits × authors" + "   * recently modified (×1.2)\n"))
	return sb.String()
}

func (m Model) renderHotspotDetail(r metrics.RiskEntry, maxScore float64) string {
	var sb strings.Builder

	ratio := r.Score / maxScore
	var level, icon string
	var levelStyle lipgloss.Style
	var desc, advice string

	switch {
	case ratio >= 0.75:
		level, icon = "CRITICAL", "🔴"
		levelStyle = styleDanger
		desc = fmt.Sprintf("One of the riskiest files in the repo — %d commits across %d author(s).", r.CommitCount, r.Authors)
		advice = "Strong candidate for refactoring, ownership clarification, or additional test coverage."
	case ratio >= 0.4:
		level, icon = "HIGH RISK", "🟡"
		levelStyle = styleWarning
		desc = fmt.Sprintf("Elevated churn — %d commits from %d author(s). Prone to merge conflicts.", r.CommitCount, r.Authors)
		advice = "Consider splitting responsibilities or adding guards against regression."
	default:
		level, icon = "STABLE", "🟢"
		levelStyle = styleSuccess
		desc = fmt.Sprintf("Low churn — %d commits from %d author(s). Relatively safe to modify.", r.CommitCount, r.Authors)
		advice = "No immediate concern. Keep an eye on it if contributor count grows."
	}

	bonus := ""
	if r.RecentBonus {
		bonus = styleWarning.Render("  ⚡ recently modified")
	}

	pw := m.panelWidth()
	fileLabel := styleAccent.Render(utils.Truncate(r.Path, pw-30))
	scoreLabel := levelStyle.Render(fmt.Sprintf("%.1f", r.Score)) + "  " + icon + "  " + levelStyle.Bold(true).Render(level) + bonus

	sb.WriteString(fmt.Sprintf("  %s   %s\n", fileLabel, scoreLabel))
	sb.WriteString(fmt.Sprintf("  %s\n", styleDim.Render(desc+"  "+advice)))
	return sb.String()
}

// ── Churn ────────────────────────────────────────────────────────────────────

func (m Model) renderChurn() string {
	if len(m.result.FileChurns) == 0 {
		return styleDim.Render("\n  No data available.")
	}
	top := m.filteredChurns()
	if len(top) == 0 {
		return styleDim.Render("\n  No results match your filter.")
	}
	maxCommits := top[0].CommitCount
	barWidth := 25

	var sb strings.Builder
	sb.WriteString("\n")
	sb.WriteString(styleLabel.Render(fmt.Sprintf("  %-50s  %7s  %7s  %-12s  %s\n",
		"File", "Commits", "Authors", "Last Modified", "Churn")))
	sb.WriteString(styleDim.Render("  " + strings.Repeat("─", m.panelWidth()-4) + "\n"))

	visibleRows := m.bodyHeight() - 4 - m.searchBarHeight()
	if visibleRows < 3 {
		visibleRows = 3
	}

	for i, f := range top {
		if i < m.scrollOffset || i >= m.scrollOffset+visibleRows {
			continue
		}
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
	sb.WriteString(stylePrimary.Render("  Commit Calendar") + "\n\n")

	daily := m.result.DailyActivity

	// Build date → count map
	dayMap := make(map[string]int)
	maxCount := 0
	for _, d := range daily {
		key := d.Date.Format("2006-01-02")
		dayMap[key] = d.Count
		if d.Count > maxCount {
			maxCount = d.Count
		}
	}

	now := time.Now()
	todayWeekday := int(now.Weekday()) // 0 = Sunday

	const cellWidth = 2  // each cell: "█ " or "░ "
	const labelWidth = 4 // day label + space + indent
	numWeeks := (m.panelWidth() - labelWidth) / cellWidth
	if numWeeks > 52 {
		numWeeks = 52
	}
	if numWeeks < 4 {
		numWeeks = 4
	}

	// Sunday that opens the oldest visible week
	currentWeekSunday := now.AddDate(0, 0, -todayWeekday)
	startSunday := currentWeekSunday.AddDate(0, 0, -(numWeeks-1)*7)

	// ── Month labels ──────────────────────────────────────────────────────────
	monthBuf := []byte(strings.Repeat(" ", numWeeks*cellWidth))
	prevMonth := time.Month(-1)
	for w := 0; w < numWeeks; w++ {
		weekStart := startSunday.AddDate(0, 0, w*7)
		if weekStart.Month() != prevMonth {
			label := []byte(weekStart.Format("Jan"))
			pos := w * cellWidth
			end := pos + len(label)
			if end > len(monthBuf) {
				end = len(monthBuf)
			}
			copy(monthBuf[pos:end], label)
			prevMonth = weekStart.Month()
		}
	}
	sb.WriteString("    " + styleLabel.Render(string(monthBuf)) + "\n")

	// ── 7-row × numWeeks-col calendar grid ────────────────────────────────────
	// Show label on Mon (1), Wed (3), Fri (5); blank on others to match GitHub.
	dayAbbrev := [7]string{"S", " ", "T", " ", "T", " ", "S"}
	for row := 0; row < 7; row++ {
		sb.WriteString("  ")
		sb.WriteString(styleLabel.Render(dayAbbrev[row]) + " ")
		for w := 0; w < numWeeks; w++ {
			date := startSunday.AddDate(0, 0, w*7+row)
			if date.After(now) {
				sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#21262d")).Render("░ "))
				continue
			}
			key := date.Format("2006-01-02")
			sb.WriteString(calendarCell(dayMap[key], maxCount))
		}
		sb.WriteString("\n")
	}

	// ── Legend ────────────────────────────────────────────────────────────────
	sb.WriteString("\n  ")
	sb.WriteString(styleLabel.Render("Less "))
	for _, v := range []int{0, 1, 3, 6, 10} {
		sb.WriteString(calendarCell(v, 10))
	}
	sb.WriteString(styleLabel.Render(" More"))
	sb.WriteString("\n")

	// ── Contributor leaderboard ───────────────────────────────────────────────
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
	visibleRows := m.bodyHeight() - 14
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

// calendarCell returns a styled 2-char cell matching GitHub's contribution palette.
func calendarCell(count, max int) string {
	if count == 0 || max == 0 {
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#21262d")).Render("░ ")
	}
	ratio := float64(count) / float64(max)
	var color string
	switch {
	case ratio <= 0.25:
		color = "#0e4429"
	case ratio <= 0.50:
		color = "#006d32"
	case ratio <= 0.75:
		color = "#26a641"
	default:
		color = "#39d353"
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Render("█ ")
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

	items := m.filteredTodos()
	if len(items) == 0 {
		sb.WriteString(styleDim.Render("  No results match your filter.\n"))
		return sb.String()
	}

	sb.WriteString(styleLabel.Render(fmt.Sprintf("  %-5s  %-6s  %-45s  %s\n", "Line", "Kind", "File", "Text")))
	sb.WriteString(styleDim.Render("  " + strings.Repeat("─", m.panelWidth()-4) + "\n"))

	visibleRows := m.bodyHeight() - m.searchBarHeight()
	if visibleRows < 5 {
		visibleRows = 5
	}
	for i, item := range items {
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

// ── Stale Files ───────────────────────────────────────────────────────────────

func (m Model) renderStale() string {
	if len(m.result.StaleFiles) == 0 {
		return styleDim.Render("\n  No data available.")
	}
	items := m.filteredStale()
	if len(items) == 0 {
		return styleDim.Render("\n  No results match your filter.")
	}

	now := time.Now()
	var sb strings.Builder
	sb.WriteString("\n")
	sb.WriteString(styleLabel.Render(fmt.Sprintf("  %-50s  %-14s  %7s  %s\n",
		"File", "Last Modified", "Commits", "Dormant")))
	sb.WriteString(styleDim.Render("  " + strings.Repeat("─", m.panelWidth()-4) + "\n"))

	visibleRows := m.bodyHeight() - 4 - m.searchBarHeight()
	if visibleRows < 3 {
		visibleRows = 3
	}

	for i, f := range items {
		if i < m.scrollOffset || i >= m.scrollOffset+visibleRows {
			continue
		}
		days := int(now.Sub(f.LastModified).Hours() / 24)
		var dormantStyle lipgloss.Style
		switch {
		case days > 365:
			dormantStyle = styleDanger
		case days > 180:
			dormantStyle = styleWarning
		default:
			dormantStyle = styleSuccess
		}
		prefix := "  "
		if i == m.cursor {
			prefix = styleAccent.Render("▶ ")
		}
		row := fmt.Sprintf("%s%-50s  %-14s  %7d  %s",
			prefix,
			utils.Truncate(f.Path, 50),
			f.LastModified.Format("2006-01-02"),
			f.CommitCount,
			dormantStyle.Render(fmt.Sprintf("%d days", days)),
		)
		if i == m.cursor {
			sb.WriteString(styleSelected.Render(row) + "\n")
		} else {
			sb.WriteString(row + "\n")
		}
	}
	sb.WriteString(styleDim.Render("\n  Files sorted by oldest last-modified — potential dead code.\n"))
	return sb.String()
}
