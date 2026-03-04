<div align="center">

# 🔭 repoview

**A terminal UI for understanding any Git repository at a glance.**

Visualize commit history, file churn, contributor activity, TODO comments, and stale files — for any local path or GitHub URL.

[![Go Version](https://img.shields.io/badge/Go-1.24+-00ADD8?logo=go&logoColor=white)](https://golang.org)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Built with Bubble Tea](https://img.shields.io/badge/Built%20with-Bubble%20Tea-ff69b4)](https://github.com/charmbracelet/bubbletea)

</div>

---

## ✨ Features

| Tab | What you get |
|-----|-------------|
| **Overview** | Repo name, path, total commits, contributors, branches, tags, size, and latest commit |
| **Churn** | Files ranked by raw commit count with heatmap bars, author counts, and last-modified timestamps |
| **Activity** | 52-week contribution calendar + contributor leaderboard with commit share |
| **Todos** | Scans for `TODO`, `FIXME`, `HACK`, and `XXX` across 40+ file types with badge summary |
| **Stale** | Files sorted by oldest last-modified — useful for spotting dead code |

> Works with **local paths** and **remote GitHub URLs**. Remote repos are shallow-cloned (last 200 commits) to a temp directory and cleaned up automatically on exit. Commit history, churn counts, and activity data will reflect only those commits for large repositories.

---

## 🚀 Quick Start

### Install via `go install`

```bash
go install github.com/connerluzier/repoview@latest
```

### Build from source

```bash
git clone https://github.com/connerluzier/repoview.git
cd repoview
go build -o repoview .
./repoview
```

---

## 🛠 Requirements

- **Go 1.24+**
- **`git`** on your `PATH`

---

## 📖 Usage

```bash
repoview
```

On launch, enter a local path or GitHub URL and press **Enter**:

```
~/projects/my-app
/absolute/path/to/repo
https://github.com/owner/repo
git@github.com:owner/repo.git
```

Paths starting with `~` are expanded to your home directory on all platforms.

---

## ⌨️ Keybindings

### Navigation

| Key | Action |
|-----|--------|
| `←` `→` or `Tab` | Switch tabs |
| `↑` `↓` or `k` `j` | Move cursor through the current page |
| `g` / `G` | Jump to first / last item |
| `Esc` | Clear filter → back to input screen |
| `q` / `Ctrl+C` | Quit |

### Within list tabs (Churn, Activity, Todos, Stale)

| Key | Action |
|-----|--------|
| `/` | Filter list by filename |
| `o` / `Enter` | Open selected file in the built-in viewer |
| `y` | Copy selected file path to clipboard |
| `r` | Refresh analysis |

### File viewer

| Key | Action |
|-----|--------|
| `↑` `↓` | Scroll line by line |
| `PgUp` `PgDn` | Scroll page by page |
| `q` / `Esc` | Close viewer, return to list |

---

## File Viewer

Pressing `o` or `Enter` on any file opens it in a full-screen viewport inside the TUI — no external editor required. Files are displayed with line numbers. On the **Todos** tab the viewer scrolls directly to the relevant line.

---

## Pagination

Long lists are paginated automatically based on your terminal height. The current page and total pages are shown below each table (`page 1 / 4`). Navigate between pages by moving the cursor past the top or bottom of the current page.

---

## 🧱 Built With

- [**Bubble Tea**](https://github.com/charmbracelet/bubbletea) — TUI framework
- [**Bubbles**](https://github.com/charmbracelet/bubbles) — UI components (text input, spinner, paginator, viewport)
- [**Lip Gloss**](https://github.com/charmbracelet/lipgloss) — terminal styling and layout
- [**go-git**](https://github.com/go-git/go-git) — pure Go Git implementation

---

## 🤝 Contributing

Contributions, issues, and feature requests are welcome. Feel free to open an issue or submit a pull request.

---

## 📄 License

MIT © [connerluzier](https://github.com/connerluzier)
