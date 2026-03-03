<div align="center">

# 🔭 repoview

**A blazing-fast terminal UI for understanding any Git repository at a glance.**

Visualize commit history, file churn, contributor activity, risk hotspots, and TODO comments — for any local path or GitHub URL.

[![Go Version](https://img.shields.io/badge/Go-1.24+-00ADD8?logo=go&logoColor=white)](https://golang.org)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Built with Bubble Tea](https://img.shields.io/badge/Built%20with-Bubble%20Tea-ff69b4)](https://github.com/charmbracelet/bubbletea)

</div>

---

## ✨ Features

| Tab | What you get |
|-----|-------------|
| **Overview** | Repo name, path, total commits, contributors, branches, tags, size, and latest commit |
| **Hotspots** | Files ranked by risk score — commit frequency × author count, boosted for recent activity. Contextual risk description updates as you scroll. |
| **Churn** | Top files by raw commit count with last-modified timestamps |
| **Activity** | 52-week GitHub-style contribution calendar + contributor leaderboard |
| **Todos** | Scans for `TODO`, `FIXME`, `HACK`, and `XXX` across 40+ file types |
| **Stale** | Files sorted by oldest last-modified — great for spotting dead code |

> 🌐 Works with **local paths** and **remote GitHub URLs** — remote repos are cloned to a temp directory and cleaned up automatically on exit.

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

### Run without installing

```bash
go run github.com/connerluzier/repoview@latest
```

---

## 🛠 Requirements

- **Go 1.21+**
- **`git`** on your `PATH`

---

## 📖 Usage

```bash
repoview
```

On launch, enter a local path or GitHub URL:

```
~/projects/my-app
/absolute/path/to/repo
https://github.com/owner/repo
git@github.com:owner/repo.git
```

Press **Enter** — repoview clones (if remote) and analyzes the repo instantly.

---

## ⌨️ Keybindings

| Key | Action |
|-----|--------|
| `←` `→` or `Tab` | Switch tabs |
| `↑` `↓` or `k` `j` | Scroll list |
| `g` / `G` | Jump to top / bottom |
| `/` | Filter list by filename (Hotspots, Churn, Todos, Stale) |
| `o` / `Enter` | Open selected file in `$EDITOR` at the correct line |
| `y` | Copy selected file path to clipboard |
| `r` | Refresh analysis |
| `Esc` | Clear filter → back to input |
| `q` / `Ctrl+C` | Quit |

---

## 🧱 Built With

- [**Bubble Tea**](https://github.com/charmbracelet/bubbletea) — TUI framework
- [**Bubbles**](https://github.com/charmbracelet/bubbles) — UI components (text input, spinner)
- [**Lip Gloss**](https://github.com/charmbracelet/lipgloss) — terminal styling
- [**go-git**](https://github.com/go-git/go-git) — pure Go Git implementation

---

## 🤝 Contributing

Contributions, issues, and feature requests are welcome! Feel free to open an issue or submit a pull request.

---

## 📄 License

MIT © [connerluzier](https://github.com/connerluzier)
