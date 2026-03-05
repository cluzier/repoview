package tui

import (
	"bufio"
	"fmt"
	"math"
	"os"
	"os/exec"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/cluzier/repoview/internal/git_analysis"
	"github.com/cluzier/repoview/internal/metrics"
)

// ── Blob animation ────────────────────────────────────────────────────────────

var blobGrad = []rune{' ', '·', '░', '▒', '▓', '█'}

func blobTick() tea.Cmd {
	return tea.Tick(55*time.Millisecond, func(time.Time) tea.Msg { return blobTickMsg{} })
}

// renderBlob draws a dithered 3-D blob at the given character dimensions.
// t is the animation time parameter (incremented each tick).
func renderBlob(t float64, w, h int) string {
	cx := float64(w-1) / 2.0
	cy := float64(h-1) / 2.0
	r := math.Min(float64(w)*0.42, float64(h)*0.88)
	var sb strings.Builder
	for row := 0; row < h; row++ {
		for col := 0; col < w; col++ {
			nx := (float64(col) - cx) / r
			ny := (float64(row) - cy) * 2.1 / r
			d := math.Sqrt(nx*nx + ny*ny)
			if d < 1e-9 {
				d = 1e-9
			}
			angle := math.Atan2(ny, nx)
			wobble := math.Sin(angle*3+t*1.8)*0.10 +
				math.Cos(angle*5-t*1.2)*0.06 +
				math.Sin(angle*7+t*2.5)*0.03
			surface := 1.0 + wobble
			inside := surface - d
			if inside <= 0 {
				sb.WriteRune(' ')
				continue
			}
			snx := nx / surface
			sny := ny / surface
			snz := math.Sqrt(math.Max(0, 1-snx*snx-sny*sny*0.25))
			snn := math.Sqrt(snx*snx + sny*sny + snz*snz)
			snx /= snn
			sny /= snn
			snz /= snn
			lx := -math.Cos(t*0.3) * 0.6
			ly := -0.5
			lz := math.Sin(t*0.3)*0.3 + 0.7
			ln := math.Sqrt(lx*lx + ly*ly + lz*lz)
			lx /= ln
			ly /= ln
			lz /= ln
			diffuse := math.Max(0, snx*lx+sny*ly+snz*lz)
			specular := math.Pow(math.Max(0, snz*lz+snx*lx), 10) * 0.5
			intensity := diffuse*0.75 + specular + 0.08
			intensity *= math.Min(1.0, inside/0.10)
			intensity = math.Max(0, math.Min(1, intensity))
			idx := int(intensity*float64(len(blobGrad)-1) + 0.5)
			if idx >= len(blobGrad) {
				idx = len(blobGrad) - 1
			}
			sb.WriteRune(blobGrad[idx])
		}
		if row < h-1 {
			sb.WriteRune('\n')
		}
	}
	return sb.String()
}

// ── Path utilities ────────────────────────────────────────────────────────────

// expandPath expands a leading ~ to the user's home directory.
func expandPath(p string) (string, error) {
	if p == "~" || strings.HasPrefix(p, "~/") || strings.HasPrefix(p, `~\`) {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		p = home + p[1:]
	}
	return p, nil
}

func isRemoteURL(s string) bool {
	return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://") || strings.HasPrefix(s, "git@")
}

// addLineNumbers prepends right-aligned line numbers to each line of content.
func addLineNumbers(content string) string {
	scanner := bufio.NewScanner(strings.NewReader(content))
	var sb strings.Builder
	lineNum := 1
	for scanner.Scan() {
		fmt.Fprintf(&sb, "%5d  %s\n", lineNum, scanner.Text())
		lineNum++
	}
	return sb.String()
}

// ── Bubble Tea commands ───────────────────────────────────────────────────────

// cloneRepo shallow-clones a remote URL into a temp directory using the system git.
func cloneRepo(url string) tea.Cmd {
	return func() tea.Msg {
		tmp, err := os.MkdirTemp("", "repoview-*")
		if err != nil {
			return cloneDoneMsg{err: err}
		}
		cmd := exec.Command("git", "clone", "--depth=200", url, tmp)
		if out, err := cmd.CombinedOutput(); err != nil {
			os.RemoveAll(tmp)
			return cloneDoneMsg{err: fmt.Errorf("%w\n%s", err, strings.TrimSpace(string(out)))}
		}
		return cloneDoneMsg{path: tmp}
	}
}

// runAnalysis performs the full repository analysis and todo scan in the background.
func runAnalysis(repoPath string) tea.Cmd {
	return func() tea.Msg {
		result := git_analysis.Analyze(repoPath)
		if result.Error != nil {
			return AnalysisDoneMsg{Result: result}
		}
		todos := metrics.ScanTodos(repoPath)
		return AnalysisDoneMsg{Result: result, Todos: todos}
	}
}
