package tui

import "github.com/charmbracelet/bubbles/key"

// mainKeyMap implements key.Map for the bubbles/help component.
type mainKeyMap struct {
	showFileOps bool
}

var (
	keyUp      = key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up"))
	keyDown    = key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down"))
	keyLeft    = key.NewBinding(key.WithKeys("left", "h"), key.WithHelp("←/h", "prev tab"))
	keyRight   = key.NewBinding(key.WithKeys("right", "l"), key.WithHelp("→/l", "next tab"))
	keyRefresh = key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh"))
	keyFilter  = key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "filter"))
	keyOpen    = key.NewBinding(key.WithKeys("enter", "o"), key.WithHelp("o", "open"))
	keyCopy    = key.NewBinding(key.WithKeys("y"), key.WithHelp("y", "copy path"))
	keyBack    = key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back"))
	keyQuit    = key.NewBinding(key.WithKeys("q"), key.WithHelp("q", "quit"))
)

func (k mainKeyMap) ShortHelp() []key.Binding {
	if k.showFileOps {
		return []key.Binding{keyUp, keyDown, keyLeft, keyRight, keyFilter, keyOpen, keyCopy, keyQuit}
	}
	return []key.Binding{keyUp, keyDown, keyLeft, keyRight, keyRefresh, keyBack, keyQuit}
}

func (k mainKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{keyUp, keyDown, keyLeft, keyRight},
		{keyFilter, keyOpen, keyCopy, keyRefresh},
		{keyBack, keyQuit},
	}
}
