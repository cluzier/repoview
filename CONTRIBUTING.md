# Contributing to repoview

Thanks for your interest in contributing! This document explains the codebase, how to set up a dev environment, and the conventions to follow when opening a pull request.

---

## Table of Contents

- [Getting started](#getting-started)
- [Project structure](#project-structure)
- [How the code flows](#how-the-code-flows)
- [Common contribution tasks](#common-contribution-tasks)
  - [Changing colors or styles](#changing-colors-or-styles)
  - [Adding a new tab](#adding-a-new-tab)
  - [Extending analysis data](#extending-analysis-data)
  - [Adding a new keybinding](#adding-a-new-keybinding)
- [Pull request guidelines](#pull-request-guidelines)

---

## Getting started

**Prerequisites:** Go 1.24+, `git` on your PATH.

```bash
git clone https://github.com/connerluzier/repoview.git
cd repoview

# build and run
go build -o repoview .
./repoview

# run directly without building
go run .
```

There is no test suite yet — if you'd like to add one, that's very welcome. Until then, test your changes manually by pointing repoview at a variety of repos (small, large, remote URL, local path, repos with no tags, empty repos, etc.).

---

## Project structure

```
repoview/
├── main.go                        # Entry point — version flag + Bubble Tea bootstrap
└── internal/
    ├── git_analysis/
    │   └── analysis.go            # Repository data: commits, churn, activity, contributors
    ├── metrics/
    │   └── metrics.go             # Risk scoring and TODO/FIXME/HACK/XXX scanner
    ├── utils/
    │   └── utils.go               # Shared formatting helpers (HumanBytes, TimeAgo, Truncate, Heatmap)
    └── tui/
        ├── styles.go              # Every color and lipgloss style — the single place to retheme
        ├── helpers.go             # Blob animation, path utilities, cloneRepo, runAnalysis
        ├── model.go               # Model struct, New(), Init(), Update(), all state helpers
        ├── view.go                # View(), screen renderers (input/loading/viewer/main), newTable()
        └── tabs.go                # One render function per tab (Overview, Churn, Activity, Todos, Stale)
```

### Package responsibilities at a glance

| Package | What it owns |
|---|---|
| `git_analysis` | All data collection. Uses go-git for root detection and `exec git` for log/churn (faster than go-git for large histories). Returns plain structs — no UI concerns. |
| `metrics` | Derived data on top of `git_analysis` output: risk scores and the TODO scanner. |
| `utils` | Pure formatting functions with no side effects. |
| `tui` | Everything the user sees. Follows the [Bubble Tea](https://github.com/charmbracelet/bubbletea) model/view/update pattern. |

---

## How the code flows

```
main.go
  └─ tui.New()              creates the initial Model (model.go)
       └─ tea.NewProgram()  starts the event loop

User types a path → Enter
  └─ updateInput()          validates input, dispatches cloneRepo or runAnalysis command (helpers.go)
       └─ runAnalysis()     calls git_analysis.Analyze() + metrics.ScanTodos() in a goroutine
            └─ AnalysisDoneMsg  → Update() stores result in Model, switches state to stateMain

Each keypress in stateMain
  └─ updateMain()           handles navigation, search, file open, clipboard copy

Every frame
  └─ View()  (view.go)
       ├─ viewMain()        renders header + tabs + body + status bar
       └─ renderXxx()       one of the five tab renderers in tabs.go
```

Analysis itself runs four goroutines in parallel (overview stats, file churn, daily activity, contributors) and waits on a `sync.WaitGroup` — see `git_analysis.Analyze()`.

---

## Common contribution tasks

### Changing colors or styles

Open `internal/tui/styles.go`. Every color in the app is an `AdaptiveColor` with separate light and dark values. Every reusable lipgloss style is a package-level var. Changing a color here propagates everywhere automatically.

```go
// Example: make the accent color green instead of blue
colorBlue = lipgloss.AdaptiveColor{Light: "#1fb009", Dark: "#3fd020"}
```

For badge styles (Todos tab) look for `styleBadgeTodo`, `styleBadgeFixme`, `styleBadgeHack`. For table cell styles look for `tableCell`, `tableHeader`, `tableSelected`.

### Adding a new tab

1. **Add the tab constant** in `internal/tui/model.go`:
   ```go
   const (
       TabOverview Tab = iota
       TabChurn
       TabActivity
       TabTodos
       TabStale
       TabMyNew    // ← add here, before tabCount
       tabCount
   )
   ```

2. **Add the tab name** in the `tabNames` array directly below the constants:
   ```go
   var tabNames = [tabCount]string{
       "   Overview   ", "   Churn   ", "   Activity   ",
       "   Todos   ", "   Stale   ", "   My New   ",
   }
   ```

3. **Wire up the renderer** in `internal/tui/view.go` inside `viewMain()`:
   ```go
   case TabMyNew:
       body = m.renderMyNew()
   ```

4. **Write the renderer** as a new method on `Model` in `internal/tui/tabs.go`:
   ```go
   func (m Model) renderMyNew() string {
       // use m.result.* or m.todos for data
       // use m.newTable() for a consistent table
       // use styleAccent, styleDim, etc. for text
   }
   ```

5. **Add search support** (optional) — if the tab has a filterable list, add a case to `searchBarHeight()`, `visibleRows()`, and a `filteredMyNew()` helper in `model.go`, following the same pattern as `filteredChurns`.

6. **If you need new data**, add fields to `AnalysisResult` in `git_analysis/analysis.go` and populate them in `Analyze()`.

### Extending analysis data

`git_analysis.Analyze()` runs four goroutines. To add a new metric:

1. Add a field to `AnalysisResult` (or an existing struct like `RepoStats`).
2. Add a `wg.Add(1)` goroutine that calls a new `computeXxx(rootPath)` function.
3. Lock `mu`, write the result, unlock.

The function should use `runGit(rootPath, ...)` for anything involving commit history (fast), and go-git's `PlainOpenWithOptions` only when you need structured object access.

### Adding a new keybinding

All keyboard handling for the main view lives in `updateMain()` in `internal/tui/model.go`. Add a new `case` to the `switch msg.String()` block. Use `msg.String()` values like `"ctrl+s"`, `"e"`, `"shift+tab"`, etc.

Update the status bar hint string in `renderStatusBar()` in `internal/tui/view.go` so users can discover the key.

---

## Pull request guidelines

- **Keep PRs focused** — one feature or fix per PR makes review much easier.
- **Match the existing style** — no external linter config is enforced, but the code follows standard `gofmt` formatting. Run `gofmt -w .` before committing.
- **No new dependencies without discussion** — open an issue first if you want to add a module.
- **Update the README** if you add a tab, a keybinding, or change any user-visible behavior.
- **Test manually** on at least a local repo and ideally a remote URL before opening a PR.

If you're unsure about scope or approach, open an issue to discuss it before writing the code — that saves everyone time.
