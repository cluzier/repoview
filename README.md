# repoview

A terminal UI for analyzing Git repositories. Visualize commit history, file churn, contributor activity, risk hotspots, and TODOs for any local path or GitHub URL.

## Features

- **Overview** — repo name, path, total commits, contributors, branches, tags, size, and latest commit
- **Hotspots** — files ranked by a risk score (commit frequency × author count, boosted for recent changes)
- **Churn** — top files by raw commit count with last-modified dates
- **Activity** — 30-day commit heatmap strip + contributor leaderboard
- **Todos** — scans source files for `TODO`, `FIXME`, `HACK`, and `XXX` comments across 40+ file types

Supports **local paths** and **remote GitHub URLs** — remote repos are cloned to a temporary directory and cleaned up automatically on exit.

## Requirements

- Go 1.21+
- `git` installed and available on your `PATH`

## Install

```bash
git clone https://github.com/cluzier/repoview.git
cd repoview
go build -o repoview .
```

Or run directly without building:

```bash
go run .
```

## Usage

```bash
go run .
# or if built:
./repoview
```

On launch you'll see an input prompt. Enter either a local path or a GitHub URL:

```
~/projects/my-app
/absolute/path/to/repo
https://github.com/owner/repo
git@github.com:owner/repo.git
```

Press **Enter** to analyze. The TUI will clone (if remote) and analyze the repository, then display the results.

## Keybindings

| Key | Action |
|-----|--------|
| `←` / `→` or `Tab` | Switch tabs |
| `↑` / `↓` or `k` / `j` | Scroll list |
| `g` / `G` | Jump to top / bottom |
| `r` | Refresh analysis |
| `Esc` | Back to input screen |
| `q` / `Ctrl+C` | Quit |

## Built With

- [Bubble Tea](https://github.com/charmbracelet/bubbletea) — TUI framework
- [Bubbles](https://github.com/charmbracelet/bubbles) — UI components (text input, spinner)
- [Lip Gloss](https://github.com/charmbracelet/lipgloss) — styling
- [go-git](https://github.com/go-git/go-git) — Git library

## License

MIT
