package mvc

import (
	"os"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/muesli/termenv"
)

// Copying to the clipboard, for a drag selection or the right-click menu
// (selection.go, contextmenu.go). It is done the terminal's way, with the
// OSC 52 escape sequence: the app asks the terminal to put the text on the
// clipboard, so no clipboard program is needed, and it works over ssh too.
// A terminal that doesn't support OSC 52 ignores it.
//
// Pasting can't be done this way (most terminals, VS Code's among them,
// don't let an app read the clipboard): the terminal's own paste, e.g.
// ctrl+shift+v, types the text into the app.

// copiedMsg says what copyToClipboard did.
type copiedMsg struct {
	chars int // how many characters were copied
}

// writeClipboard sends text to the terminal's clipboard. It is a variable
// so tests can see what is copied without a terminal.
var writeClipboard = func(text string) {
	termenv.NewOutput(os.Stdout).Copy(text)
}

// copyToClipboard returns the command that copies text.
func copyToClipboard(text string) tea.Cmd {
	return func() tea.Msg {
		writeClipboard(text)
		return copiedMsg{chars: utf8.RuneCountInString(text)}
	}
}
