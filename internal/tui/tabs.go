package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/cluzier/repoview/internal/utils"
)

// ── Overview ──────────────────────────────────────────────────────────────────

func (m Model) renderOverview() string {
	s := m.result.Stats
	pw := m.panelWidth()

	kv := func(icon, label, value string) string {
		return "    " + styleDim.Render(icon) +
			"   " + styleLabel.Width(24).Render(label) +
			styleValue.Render(value)
	}

	lines := []string{
		"",
		"",
		kv("📁", "Repository", s.RepoName),
		kv("📍", "Path", utils.Truncate(s.RepoPath, pw-34)),
		"",
		"",
		kv("📝", "Total Commits", fmt.Sprintf("%d", s.TotalCommits)),
		kv("👥", "Contributors", fmt.Sprintf("%d", s.TotalContributors)),
		kv("🌿", "Branches", fmt.Sprintf("%d", s.TotalBranches)),
		kv("🏷 ", "Tags", fmt.Sprintf("%d", s.TotalTags)),
		kv("💾", "Approx. Size", utils.HumanBytes(s.RepoSizeBytes)),
	}

	if len(s.Tags) > 0 {
		const maxTags = 5
		shown := s.Tags
		if len(shown) > maxTags {
			shown = shown[:maxTags]
		}
		tagLine := strings.Join(shown, "  ·  ")
		if len(s.Tags) > maxTags {
			tagLine += fmt.Sprintf("  +%d more", len(s.Tags)-maxTags)
		}
		lines = append(lines, "    "+styleDim.Render("          ")+"   "+styleDim.Render(tagLine))
	}

	if s.LatestCommit != nil {
		lc := s.LatestCommit
		divider := "    " + styleDim.Render(strings.Repeat("─", pw-8))
		lines = append(lines, "", "", divider, "",
			kv("🔖", "Hash", lc.Hash),
			kv("✍️ ", "Author", lc.Author),
			kv("🕐", "When", utils.TimeAgo(lc.When)),
			kv("💬", "Message", utils.Truncate(lc.Message, pw-34)),
		)
	}
	return strings.Join(lines, "\n")
}

// ── Branches ──────────────────────────────────────────────────────────────────

func (m Model) renderBranches() string {
	branches := m.result.BranchActivity
	if len(branches) == 0 {
		return styleDim.Render("\n  No branch data available.")
	}

	startIdx, endIdx := m.page.GetSliceBounds(len(branches))
	selectedInView := m.cursor - startIdx

	rows := make([][]string, 0, endIdx-startIdx)
	for i := startIdx; i < endIdx; i++ {
		b := branches[i]

		name := b.Name
		if b.IsCurrent {
			name = styleAccent.Render("* " + name)
		} else {
			name = "  " + name
		}

		status := "  "
		if b.IsActive {
			status = styleSuccess.Render("● active")
		}

		last := utils.TimeAgo(b.LastCommit)
		rows = append(rows, []string{name, b.AuthorName, last, b.ShortHash, status})
	}

	table := m.newTable(selectedInView, []string{"Branch", "Last Author", "Last Commit", "Hash", "Status"}, rows)
	return "\n\n" + table + "\n\n" + styleLabel.Render(m.page.View())
}

// ── Churn ─────────────────────────────────────────────────────────────────────

func (m Model) renderChurn() string {
	if len(m.result.FileChurns) == 0 {
		return styleDim.Render("\n  No data available.")
	}
	top := m.filteredChurns()
	if len(top) == 0 {
		return styleDim.Render("\n  No results match your filter.")
	}
	maxCommits := top[0].CommitCount

	startIdx, endIdx := m.page.GetSliceBounds(len(top))
	selectedInView := m.cursor - startIdx

	rows := make([][]string, 0, endIdx-startIdx)
	for i := startIdx; i < endIdx; i++ {
		f := top[i]
		bar := utils.Heatmap(f.CommitCount, maxCommits, 25)
		rows = append(rows, []string{
			f.Path,
			fmt.Sprintf("%d", f.CommitCount),
			fmt.Sprintf("%d", f.UniqueAuthors),
			utils.TimeAgo(f.LastModified),
			bar,
		})
	}

	table := m.newTable(selectedInView, []string{"File", "Commits", "Authors", "Last Modified", "Churn"}, rows)
	return "\n\n" + table + "\n\n" + styleLabel.Render(m.page.View())
}

// ── Activity ──────────────────────────────────────────────────────────────────

func (m Model) renderActivity() string {
	var sb strings.Builder
	sb.WriteString("\n")
	sb.WriteString(styleAccent.Render("  Commit Calendar") + "\n\n")

	daily := m.result.DailyActivity
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
	todayWeekday := int(now.Weekday())

	const cellWidth = 2
	const labelWidth = 4
	numWeeks := (m.panelWidth() - labelWidth) / cellWidth
	if numWeeks > 52 {
		numWeeks = 52
	}
	if numWeeks < 4 {
		numWeeks = 4
	}

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

	// ── 7-row × numWeeks-col grid ─────────────────────────────────────────────
	dayAbbrev := [7]string{"S", " ", "T", " ", "T", " ", "S"}
	for row := 0; row < 7; row++ {
		sb.WriteString("  ")
		sb.WriteString(styleLabel.Render(dayAbbrev[row]) + " ")
		for w := 0; w < numWeeks; w++ {
			date := startSunday.AddDate(0, 0, w*7+row)
			if date.After(now) {
				sb.WriteString(lipgloss.NewStyle().Foreground(calendarEmpty).Render("░ "))
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
	sb.WriteString("\n\n")
	sb.WriteString(styleAccent.Render("  Contributors") + "\n\n")

	total := 0
	for _, c := range contribs {
		total += c.Count
	}

	startIdx, endIdx := m.page.GetSliceBounds(len(contribs))
	selectedInView := m.cursor - startIdx

	rows := make([][]string, 0, endIdx-startIdx)
	for i := startIdx; i < endIdx; i++ {
		c := contribs[i]
		pct := 0.0
		if total > 0 {
			pct = float64(c.Count) / float64(total) * 100
		}
		bar := utils.Heatmap(c.Count, contribs[0].Count, 20)
		rows = append(rows, []string{
			c.Name,
			fmt.Sprintf("%d", c.Count),
			fmt.Sprintf("%.1f%%", pct),
			bar,
		})
	}

	sb.WriteString(m.newTable(selectedInView, []string{"Name", "Commits", "Share", "Bar"}, rows))
	sb.WriteString("\n\n" + styleLabel.Render(m.page.View()))
	return sb.String()
}

// calendarCell returns a styled 2-char cell using the adaptive heat-map palette.
func calendarCell(count, max int) string {
	if count == 0 || max == 0 {
		return lipgloss.NewStyle().Foreground(calendarEmpty).Render("░ ")
	}
	ratio := float64(count) / float64(max)
	var idx int
	switch {
	case ratio <= 0.25:
		idx = 0
	case ratio <= 0.50:
		idx = 1
	case ratio <= 0.75:
		idx = 2
	default:
		idx = 3
	}
	return lipgloss.NewStyle().Foreground(calendarLevels[idx]).Render("█ ")
}

// ── Todos ─────────────────────────────────────────────────────────────────────

func (m Model) renderTodos() string {
	summary := m.todos
	var sb strings.Builder
	sb.WriteString("\n\n")

	// Badge summary row
	sb.WriteString("    ")
	for _, kw := range []string{"TODO", "FIXME", "HACK", "XXX"} {
		count := summary.CountByKind[kw]
		var style lipgloss.Style
		switch kw {
		case "FIXME":
			style = styleBadgeFixme
		case "HACK":
			style = styleBadgeHack
		default:
			style = styleBadgeTodo
		}
		sb.WriteString(style.Render(fmt.Sprintf("%s %d", kw, count)) + "   ")
	}
	sb.WriteString(styleValue.Render(fmt.Sprintf("Total: %d", summary.TotalCount)))
	sb.WriteString("\n\n\n")

	if summary.TotalCount == 0 {
		sb.WriteString(styleSuccess.Render("  ✓  No TODOs found — clean codebase!\n"))
		return sb.String()
	}

	items := m.filteredTodos()
	if len(items) == 0 {
		sb.WriteString(styleDim.Render("  No results match your filter.\n"))
		return sb.String()
	}

	startIdx, endIdx := m.page.GetSliceBounds(len(items))
	selectedInView := m.cursor - startIdx

	rows := make([][]string, 0, endIdx-startIdx)
	for i := startIdx; i < endIdx; i++ {
		item := items[i]
		rows = append(rows, []string{
			fmt.Sprintf("%d", item.Line),
			item.Kind,
			item.File,
			utils.Truncate(item.Text, m.panelWidth()-80),
		})
	}

	sb.WriteString(m.newTable(selectedInView, []string{"Line", "Kind", "File", "Text"}, rows))
	sb.WriteString("\n\n" + styleLabel.Render(m.page.View()))
	return sb.String()
}

// ── Stale files ───────────────────────────────────────────────────────────────

func (m Model) renderStale() string {
	if len(m.result.StaleFiles) == 0 {
		return styleDim.Render("\n  No data available.")
	}
	items := m.filteredStale()
	if len(items) == 0 {
		return styleDim.Render("\n  No results match your filter.")
	}

	now := time.Now()
	startIdx, endIdx := m.page.GetSliceBounds(len(items))
	selectedInView := m.cursor - startIdx

	rows := make([][]string, 0, endIdx-startIdx)
	for i := startIdx; i < endIdx; i++ {
		f := items[i]
		days := int(now.Sub(f.LastModified).Hours() / 24)
		var dormant string
		switch {
		case days > 365:
			dormant = styleDanger.Render(fmt.Sprintf("%d days", days))
		case days > 180:
			dormant = styleWarning.Render(fmt.Sprintf("%d days", days))
		default:
			dormant = styleSuccess.Render(fmt.Sprintf("%d days", days))
		}
		rows = append(rows, []string{
			f.Path,
			f.LastModified.Format("2006-01-02"),
			fmt.Sprintf("%d", f.CommitCount),
			dormant,
		})
	}

	var sb strings.Builder
	sb.WriteString("\n\n")
	sb.WriteString(m.newTable(selectedInView, []string{"File", "Last Modified", "Commits", "Dormant"}, rows))
	sb.WriteString(styleDim.Render("\n\n  Files sorted by oldest last-modified — potential dead code.\n"))
	sb.WriteString("  " + styleLabel.Render(m.page.View()))
	return sb.String()
}
