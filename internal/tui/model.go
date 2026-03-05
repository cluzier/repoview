package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/cluzier/repoview/internal/git_analysis"
	"github.com/cluzier/repoview/internal/metrics"
	"github.com/cluzier/repoview/internal/utils"
)

// ── App states ────────────────────────────────────────────────────────────────

type appState int

const (
	stateInput appState = iota
	stateLoading
	stateMain
	stateViewer
)

// ── Tabs ──────────────────────────────────────────────────────────────────────

type Tab int

const (
	TabOverview Tab = iota
	TabBranches
	TabChurn
	TabActivity
	TabTodos
	TabStale
	tabCount
)

var tabNames = [tabCount]string{"   Overview   ", "   Branches   ", "   Churn   ", "   Activity   ", "   Todos   ", "   Stale   "}

// ── Messages ──────────────────────────────────────────────────────────────────

// AnalysisDoneMsg is sent when background analysis completes.
type AnalysisDoneMsg struct {
	Result git_analysis.AnalysisResult
	Todos  metrics.TodoSummary
}

type cloneDoneMsg struct {
	path string
	err  error
}

// RefreshMsg triggers a re-analysis of the current repository.
type RefreshMsg struct{}

type flashClearMsg struct{}
type blobTickMsg struct{}

// ── Model ─────────────────────────────────────────────────────────────────────

type Model struct {
	state      appState
	input      textinput.Model
	spinner    spinner.Model
	loadingMsg string
	tmpDir     string // temp dir for remote clones; cleaned up on exit
	inputErr   string
	blobT      float64

	repoPath  string
	activeTab Tab
	loading   bool
	err       error
	result    git_analysis.AnalysisResult
	todos     metrics.TodoSummary
	tbl       table.Model
	help      help.Model
	width     int
	height    int

	viewer      viewport.Model
	viewerTitle string

	searchMode  bool
	searchQuery string
	searchInput textinput.Model
	flashMsg    string
}

func New() Model {
	ti := textinput.New()
	ti.Placeholder = "~/projects/myrepo  or  https://github.com/owner/repo"
	ti.PlaceholderStyle = lipgloss.NewStyle().Foreground(colorGray)
	ti.TextStyle = lipgloss.NewStyle().Foreground(colorFg)
	ti.Cursor.Style = lipgloss.NewStyle().Foreground(colorBlue)
	ti.Width = 60
	ti.Focus()

	si := textinput.New()
	si.PlaceholderStyle = lipgloss.NewStyle().Foreground(colorGray)
	si.TextStyle = lipgloss.NewStyle().Foreground(colorFg)
	si.Cursor.Style = lipgloss.NewStyle().Foreground(colorBlue)
	si.Width = 40

	sp := spinner.New()
	sp.Spinner = spinner.Pulse
	sp.Style = lipgloss.NewStyle().Foreground(colorBlue)

	return Model{
		state:       stateInput,
		input:       ti,
		spinner:     sp,
		searchInput: si,
		help:        help.New(),
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
		m.help.Width = msg.Width
		if m.state == stateViewer {
			m.viewer.Width = msg.Width
			m.viewer.Height = msg.Height - 6
		}
		if m.state == stateMain {
			m.rebuildTable()
		}

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case blobTickMsg:
		if m.state == stateLoading {
			m.blobT += 0.07
			return m, blobTick()
		}

	case cloneDoneMsg:
		if msg.err != nil {
			m.state = stateInput
			m.inputErr = fmt.Sprintf("Clone failed: %v", msg.err)
			return m, nil
		}
		m.tmpDir = msg.path
		m.loadingMsg = "Analyzing repository…"
		return m, tea.Batch(runAnalysis(msg.path), blobTick())

	case AnalysisDoneMsg:
		m.loading = false
		m.state = stateMain
		m.result = msg.Result
		m.todos = msg.Todos
		m.err = msg.Result.Error
		m.rebuildTable()

	case flashClearMsg:
		m.flashMsg = ""

	case RefreshMsg:
		m.loading = true
		return m, runAnalysis(m.repoPath)

	case tea.KeyMsg:
		// ctrl+c quits from any state, always cleaning up.
		if msg.Type == tea.KeyCtrlC {
			m.Cleanup()
			return m, tea.Quit
		}
		switch m.state {
		case stateInput:
			return m.updateInput(msg)
		case stateLoading:
			return m.updateLoading(msg)
		case stateMain:
			return m.updateMain(msg)
		case stateViewer:
			return m.updateViewer(msg)
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
			return m, tea.Batch(cloneRepo(raw), blobTick())
		}
		expanded, err := expandPath(raw)
		if err != nil {
			m.inputErr = fmt.Sprintf("Invalid path: %v", err)
			m.state = stateInput
			return m, nil
		}
		abs, err := filepath.Abs(expanded)
		if err != nil {
			m.inputErr = fmt.Sprintf("Invalid path: %v", err)
			m.state = stateInput
			return m, nil
		}
		m.repoPath = abs
		m.loadingMsg = "Analyzing repository…"
		return m, tea.Batch(runAnalysis(abs), blobTick())

	case tea.KeyCtrlC, tea.KeyEsc:
		return m, tea.Quit
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

// updateLoading handles key presses while a clone or analysis is in progress.
func (m Model) updateLoading(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.Cleanup()
		return m, tea.Quit
	}
	if msg.String() == "q" {
		m.Cleanup()
		return m, tea.Quit
	}
	return m, nil
}

func (m Model) updateMain(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.searchMode {
		switch msg.Type {
		case tea.KeyEsc:
			m.searchMode = false
			m.searchQuery = ""
			m.searchInput.SetValue("")
			m.rebuildTable()
			return m, nil
		case tea.KeyEnter:
			m.searchMode = false
			return m, nil
		default:
			var cmd tea.Cmd
			m.searchInput, cmd = m.searchInput.Update(msg)
			m.searchQuery = m.searchInput.Value()
			m.rebuildTable()
			return m, cmd
		}
	}

	switch msg.String() {
	case "q":
		m.Cleanup()
		return m, tea.Quit

	case "backspace", "esc":
		if m.searchQuery != "" {
			m.searchQuery = ""
			m.searchInput.SetValue("")
			m.rebuildTable()
			return m, nil
		}
		m.Cleanup()
		m.state = stateInput
		m.activeTab = TabOverview
		m.input.SetValue("")
		m.inputErr = ""
		m.result = git_analysis.AnalysisResult{}
		m.todos = metrics.TodoSummary{}
		m.err = nil
		return m, textinput.Blink

	case "/":
		switch m.activeTab {
		case TabChurn, TabTodos, TabStale:
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
			m.resetSearch()
		}

	case "right", "l":
		if m.activeTab < tabCount-1 {
			m.activeTab++
			m.resetSearch()
		}

	case "up", "k":
		m.tbl.MoveUp(1)

	case "down", "j":
		m.tbl.MoveDown(1)

	case "g":
		m.tbl.GotoTop()

	case "G":
		m.tbl.GotoBottom()

	case "tab":
		m.activeTab = (m.activeTab + 1) % tabCount
		m.resetSearch()

	case "enter", "o":
		path := m.currentFilePath()
		if path == "" {
			return m, nil
		}
		fullPath := filepath.Join(m.repoPath, path)
		content, err := os.ReadFile(fullPath)
		if err != nil {
			m.flashMsg = "✖ Cannot read file: " + err.Error()
			return m, tea.Tick(2*time.Second, func(time.Time) tea.Msg { return flashClearMsg{} })
		}
		numbered := addLineNumbers(string(content))
		vp := viewport.New(m.width, m.height-6)
		vp.Style = lipgloss.NewStyle().Foreground(colorText)
		vp.SetContent(numbered)
		if line := m.currentFileLine(); line > 1 {
			vp.SetYOffset(line - 2)
		}
		m.viewer = vp
		m.viewerTitle = path
		m.state = stateViewer
		return m, nil

	case "y":
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

func (m Model) updateViewer(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc", "ctrl+c":
		m.state = stateMain
		return m, nil
	}
	var cmd tea.Cmd
	m.viewer, cmd = m.viewer.Update(msg)
	return m, cmd
}

// ── Model helpers ─────────────────────────────────────────────────────────────

// Cleanup removes all repoview temporary directories from the OS temp folder,
// including any orphaned from previous crashed sessions.
func (m *Model) Cleanup() {
	m.tmpDir = ""
	entries, err := os.ReadDir(os.TempDir())
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() && strings.HasPrefix(e.Name(), "repoview-") {
			os.RemoveAll(filepath.Join(os.TempDir(), e.Name()))
		}
	}
}

// resetSearch clears search state and rebuilds the table, used on tab change.
func (m *Model) resetSearch() {
	m.searchMode = false
	m.searchQuery = ""
	m.searchInput.SetValue("")
	m.rebuildTable()
}

// panelWidth is the usable content width (full terminal width, min 40).
func (m Model) panelWidth() int {
	w := m.width
	if w < 40 {
		w = 40
	}
	return w
}

// bodyHeight is the scrollable rows between the tab bar and status bar.
func (m Model) bodyHeight() int {
	// header(1) + tab bar with borders(3) + status bar(1) = 5 overhead
	h := m.height - 5
	if h < 5 {
		h = 5
	}
	return h
}

// searchBarHeight returns 1 when the search bar is visible, 0 otherwise.
func (m Model) searchBarHeight() int {
	switch m.activeTab {
	case TabChurn, TabTodos, TabStale:
		if m.searchMode || m.searchQuery != "" {
			return 1
		}
	}
	return 0
}

// ── Filtered list helpers ─────────────────────────────────────────────────────

func (m Model) filteredChurns() []git_analysis.FileChurn {
	items := m.result.FileChurns
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

// currentFilePath returns the relative path of the selected item on the active tab.
func (m Model) currentFilePath() string {
	switch m.activeTab {
	case TabChurn:
		items := m.filteredChurns()
		if m.tbl.Cursor() < len(items) {
			return items[m.tbl.Cursor()].Path
		}
	case TabStale:
		items := m.filteredStale()
		if m.tbl.Cursor() < len(items) {
			return items[m.tbl.Cursor()].Path
		}
	case TabTodos:
		items := m.filteredTodos()
		if m.tbl.Cursor() < len(items) {
			return items[m.tbl.Cursor()].File
		}
	}
	return ""
}

// currentFileLine returns the line number for the selected item (non-zero only on TabTodos).
func (m Model) currentFileLine() int {
	if m.activeTab != TabTodos {
		return 0
	}
	items := m.filteredTodos()
	if m.tbl.Cursor() < len(items) {
		return items[m.tbl.Cursor()].Line
	}
	return 0
}

// rebuildTable constructs a new bubbles/table for the active tab.
func (m *Model) rebuildTable() {
	if m.activeTab == TabOverview {
		return
	}

	pw := m.panelWidth()
	var cols []table.Column
	var rows []table.Row
	prefixLines := 0

	switch m.activeTab {
	case TabBranches:
		prefixLines = 1
		cols = []table.Column{
			{Title: "Branch", Width: pw * 30 / 100},
			{Title: "Last Author", Width: pw * 18 / 100},
			{Title: "Last Commit", Width: pw * 14 / 100},
			{Title: "Hash", Width: 10},
			{Title: "Status", Width: 8},
		}
		for _, b := range m.result.BranchActivity {
			name := "  " + b.Name
			if b.IsCurrent {
				name = styleAccent.Render("* " + b.Name)
			}
			status := ""
			if b.IsActive {
				status = styleSuccess.Render("● active")
			}
			rows = append(rows, table.Row{name, b.AuthorName, utils.TimeAgo(b.LastCommit), b.ShortHash, status})
		}

	case TabChurn:
		prefixLines = 1
		cols = []table.Column{
			{Title: "File", Width: pw * 35 / 100},
			{Title: "Commits", Width: 8},
			{Title: "Authors", Width: 8},
			{Title: "Last Modified", Width: 14},
			{Title: "Churn", Width: pw * 18 / 100},
		}
		churns := m.filteredChurns()
		maxCommits := 0
		if len(churns) > 0 {
			maxCommits = churns[0].CommitCount
		}
		for _, f := range churns {
			bar := utils.Heatmap(f.CommitCount, maxCommits, 20)
			rows = append(rows, table.Row{f.Path, fmt.Sprintf("%d", f.CommitCount), fmt.Sprintf("%d", f.UniqueAuthors), utils.TimeAgo(f.LastModified), bar})
		}

	case TabActivity:
		prefixLines = 17
		cols = []table.Column{
			{Title: "Name", Width: pw * 30 / 100},
			{Title: "Commits", Width: 8},
			{Title: "Share", Width: 8},
			{Title: "Bar", Width: pw * 25 / 100},
		}
		contribs := m.result.ContributorActivity
		total := 0
		for _, c := range contribs {
			total += c.Count
		}
		for _, c := range contribs {
			pct := 0.0
			if total > 0 {
				pct = float64(c.Count) / float64(total) * 100
			}
			bar := ""
			if len(contribs) > 0 {
				bar = utils.Heatmap(c.Count, contribs[0].Count, 20)
			}
			rows = append(rows, table.Row{c.Name, fmt.Sprintf("%d", c.Count), fmt.Sprintf("%.1f%%", pct), bar})
		}

	case TabTodos:
		prefixLines = 5
		cols = []table.Column{
			{Title: "Line", Width: 6},
			{Title: "Kind", Width: 8},
			{Title: "File", Width: pw * 28 / 100},
			{Title: "Text", Width: pw * 35 / 100},
		}
		for _, item := range m.filteredTodos() {
			rows = append(rows, table.Row{fmt.Sprintf("%d", item.Line), item.Kind, item.File, utils.Truncate(item.Text, pw-60)})
		}

	case TabStale:
		prefixLines = 1
		cols = []table.Column{
			{Title: "File", Width: pw * 38 / 100},
			{Title: "Last Modified", Width: 14},
			{Title: "Commits", Width: 8},
			{Title: "Dormant", Width: 10},
		}
		for _, f := range m.filteredStale() {
			days := int(time.Since(f.LastModified).Hours() / 24)
			var dormant string
			switch {
			case days > 365:
				dormant = styleDanger.Render(fmt.Sprintf("%d days", days))
			case days > 180:
				dormant = styleWarning.Render(fmt.Sprintf("%d days", days))
			default:
				dormant = styleSuccess.Render(fmt.Sprintf("%d days", days))
			}
			rows = append(rows, table.Row{f.Path, f.LastModified.Format("2006-01-02"), fmt.Sprintf("%d", f.CommitCount), dormant})
		}
	}

	h := (m.bodyHeight() - m.searchBarHeight()) - prefixLines - 1
	if m.activeTab == TabStale {
		h -= 3
	}
	if h < 3 {
		h = 3
	}

	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(colorSubtle).
		BorderBottom(true).
		Foreground(colorGray).
		Bold(false)
	s.Cell = lipgloss.NewStyle().Padding(0, 1).Foreground(colorText)
	s.Selected = lipgloss.NewStyle().Padding(0, 1).Foreground(colorBlue).Bold(true)

	t := table.New(
		table.WithColumns(cols),
		table.WithRows(rows),
		table.WithHeight(h),
		table.WithWidth(pw),
		table.WithFocused(true),
	)
	t.SetStyles(s)
	m.tbl = t
}
