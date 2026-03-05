package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/cluzier/repoview/internal/tui"
)

// version is set at build time via -X main.version=<tag> (see .goreleaser.yaml).
var version = "dev"

func main() {
	if len(os.Args) > 1 && (os.Args[1] == "--version" || os.Args[1] == "-v") {
		fmt.Println("repoview " + version)
		return
	}
	m := tui.New()
	p := tea.NewProgram(m, tea.WithAltScreen())
	final, err := p.Run()
	if fm, ok := final.(tui.Model); ok {
		fm.Cleanup()
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "error running TUI: %v\n", err)
		os.Exit(1)
	}
}
