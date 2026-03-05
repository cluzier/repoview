package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/paginator"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/cluzier/repoview/internal/git_analysis"
	"github.com/cluzier/repoview/internal/metrics"
)

// ── App states ────────────────────────────────────────────────────────────────

type appState int

const (
	stateInput   appState = iota
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
	cursor    int
	width     int
	height    int
	page      paginator.Model

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
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(colorBlue)

	pg := paginator.New()
	pg.Type = paginator.Arabic
	pg.ArabicFormat = "  page %d / %d"
	pg.PerPage = 10 // recalculated on first render

	return Model{
		state:       stateInput,
		input:       ti,
		spinner:     sp,
		searchInput: si,
		page:        pg,
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
		if m.state == stateViewer {
			m.viewer.Width = msg.Width
			m.viewer.Height = msg.Height - 3
		}
		if m.state == stateMain {
			m.clampScroll()
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
		m.cursor = 0
		m.page.Page = 0
		m.clampScroll()

	case flashClearMsg:
		m.flashMsg = ""

	case RefreshMsg:
		m.loading = true
		m.cursor = 0
		m.page.Page = 0
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
			m.cursor = 0
			m.page.Page = 0
			return m, nil
		case tea.KeyEnter:
			m.searchMode = false
			return m, nil
		default:
			var cmd tea.Cmd
			m.searchInput, cmd = m.searchInput.Update(msg)
			m.searchQuery = m.searchInput.Value()
			m.cursor = 0
			m.page.Page = 0
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
			m.cursor = 0
			m.page.Page = 0
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
		m.page.Page = 0

	case "G":
		m.cursor = m.listLen() - 1
		m.clampCursor()
		m.clampScroll()

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
		vp := viewport.New(m.width, m.height-3)
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

// resetSearch clears search state and resets the cursor/page, used on tab change.
func (m *Model) resetSearch() {
	m.cursor = 0
	m.page.Page = 0
	m.searchMode = false
	m.searchQuery = ""
	m.searchInput.SetValue("")
	m.clampScroll()
}

func (m *Model) listLen() int {
	switch m.activeTab {
	case TabBranches:
		return len(m.result.BranchActivity)
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
	perPage := m.visibleRows()
	if perPage < 1 {
		perPage = 1
	}
	m.page.PerPage = perPage
	m.page.SetTotalPages(m.listLen())
	m.page.Page = m.cursor / perPage
}

// visibleRows returns how many list rows fit in the body for the active tab.
func (m Model) visibleRows() int {
	var n int
	switch m.activeTab {
	case TabBranches:
		// 2 blank + top border + header + sep + bottom border + 2 blank + page = 8
		n = m.bodyHeight() - 8
	case TabChurn:
		// 2 blank + top border + header + sep + bottom border + 2 blank + page = 8
		n = m.bodyHeight() - m.searchBarHeight() - 8
	case TabActivity:
		// calendar+legend+spacing (~20) + table chrome + page ≈ 24
		n = m.bodyHeight() - 24
	case TabTodos:
		// 2 blank + badges + 3 blank + table chrome + 2 blank + page = 12
		n = m.bodyHeight() - m.searchBarHeight() - 12
	case TabStale:
		// 2 blank + table chrome + footer + page = 9
		n = m.bodyHeight() - m.searchBarHeight() - 9
	default:
		n = m.bodyHeight()
	}
	if n < 1 {
		n = 1
	}
	return n
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

// currentFileLine returns the line number for the selected item (non-zero only on TabTodos).
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
