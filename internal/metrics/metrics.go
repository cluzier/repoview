// Package metrics computes risk scores and scans for code TODOs.
package metrics

import (
	"bufio"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/connerluzier/repoview/internal/git_analysis"
)

// RiskEntry represents a file with a computed risk score.
type RiskEntry struct {
	Path        string
	CommitCount int
	Authors     int
	Score       float64
	RecentBonus bool
}

// TodoItem represents a single TODO/FIXME/HACK/XXX found in source.
type TodoItem struct {
	File    string
	Line    int
	Kind    string // TODO, FIXME, HACK, XXX
	Text    string
}

// TodoSummary groups todos by file.
type TodoSummary struct {
	Items      []TodoItem
	CountByKind map[string]int
	TotalCount  int
}

var todoKeywords = []string{"TODO", "FIXME", "HACK", "XXX"}

// ComputeRiskScores applies the heuristic risk formula and returns top entries sorted by score.
func ComputeRiskScores(churns []git_analysis.FileChurn) []RiskEntry {
	now := time.Now()
	recent := now.AddDate(0, 0, -7)

	entries := make([]RiskEntry, 0, len(churns))
	for _, f := range churns {
		score := float64(f.CommitCount)*0.6 + float64(f.UniqueAuthors)*0.4
		recentBonus := false
		if f.LastModified.After(recent) {
			score *= 1.2
			recentBonus = true
		}
		score = math.Round(score*100) / 100
		entries = append(entries, RiskEntry{
			Path:        f.Path,
			CommitCount: f.CommitCount,
			Authors:     f.UniqueAuthors,
			Score:       score,
			RecentBonus: recentBonus,
		})
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Score > entries[j].Score
	})
	return entries
}

// ScanTodos walks repoPath and returns all TODO/FIXME/HACK/XXX items.
func ScanTodos(repoPath string) TodoSummary {
	var items []TodoItem
	countByKind := make(map[string]int)

	// Common text file extensions to scan
	textExts := map[string]bool{
		".go": true, ".js": true, ".ts": true, ".tsx": true, ".jsx": true,
		".py": true, ".rb": true, ".java": true, ".c": true, ".cpp": true,
		".h": true, ".hpp": true, ".cs": true, ".rs": true, ".swift": true,
		".kt": true, ".sh": true, ".bash": true, ".zsh": true, ".fish": true,
		".md": true, ".txt": true, ".yaml": true, ".yml": true, ".toml": true,
		".json": true, ".html": true, ".css": true, ".scss": true, ".vue": true,
		".php": true, ".lua": true, ".r": true, ".scala": true, ".ex": true,
		".exs": true, ".erl": true, ".hs": true, ".ml": true, ".clj": true,
	}

	_ = filepath.WalkDir(repoPath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		// Skip hidden directories (including .git)
		if d.IsDir() {
			name := d.Name()
			if strings.HasPrefix(name, ".") || name == "node_modules" || name == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if !textExts[ext] {
			return nil
		}

		scanFile(path, repoPath, &items, countByKind)
		return nil
	})

	// Sort by file then line
	sort.Slice(items, func(i, j int) bool {
		if items[i].File != items[j].File {
			return items[i].File < items[j].File
		}
		return items[i].Line < items[j].Line
	})

	return TodoSummary{
		Items:       items,
		CountByKind: countByKind,
		TotalCount:  len(items),
	}
}

func scanFile(path, repoPath string, items *[]TodoItem, countByKind map[string]int) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	rel, _ := filepath.Rel(repoPath, path)

	scanner := bufio.NewScanner(f)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		upper := strings.ToUpper(line)
		for _, kw := range todoKeywords {
			if idx := strings.Index(upper, kw); idx >= 0 {
				// Extract surrounding text
				text := strings.TrimSpace(line[idx:])
				if len(text) > 100 {
					text = text[:100] + "…"
				}
				*items = append(*items, TodoItem{
					File: rel,
					Line: lineNum,
					Kind: kw,
					Text: text,
				})
				countByKind[kw]++
				break // only one match per line
			}
		}
	}
}
