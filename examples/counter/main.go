// A tiny Bubble Tea app: a counter.
// It uses the same Model / Update / View pattern as the k8s-tui app.
//
// Run it with:  go run ./examples/counter
package main

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
)

// ---------------------------------------------------------------
// 1. MODEL: the data the app remembers.
//
// Think of it as a whiteboard with one number written on it.
// In k8s-tui the model holds more (active tab, cursor, size...),
// but the idea is the same.
// ---------------------------------------------------------------
type model struct {
	count int
}

// ---------------------------------------------------------------
// 2. INIT: runs once when the app starts.
//
// Return a command to start something (like loading data),
// or nil to do nothing.
// ---------------------------------------------------------------
func (m model) Init() tea.Cmd {
	return nil
}

// ---------------------------------------------------------------
// 3. UPDATE: something happened (a key press), so change the data.
//
// Bubble Tea calls this every time an event happens.
// It gets the current model, changes it, and returns the new one.
// It never draws anything.
// ---------------------------------------------------------------
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Is this event a key press?
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "up":
			m.count++ // erase the number, write number + 1
		case "down":
			m.count-- // erase the number, write number - 1
		case "q", "ctrl+c":
			return m, tea.Quit // stop the app
		}
	}

	// Give the changed model back to Bubble Tea.
	// Bubble Tea will now call View() to redraw the screen.
	return m, nil
}

// ---------------------------------------------------------------
// 4. VIEW: turn the data into text for the screen.
//
// Bubble Tea calls this after every Update.
// It only reads the model; it never changes it.
// ---------------------------------------------------------------
func (m model) View() string {
	return fmt.Sprintf("Count: %d\n\n↑ add • ↓ subtract • q quit\n", m.count)
}

// ---------------------------------------------------------------
// The loop Bubble Tea runs for you:
//
//	start:     model{count: 0} -> View -> "Count: 0"
//	press ↑:   Update -> model{count: 1} -> View -> "Count: 1"
//	press ↑:   Update -> model{count: 2} -> View -> "Count: 2"
//	press ↓:   Update -> model{count: 1} -> View -> "Count: 1"
//	press q:   Update -> tea.Quit -> app ends
//
//	   MODEL --> VIEW --> screen
//	     ^                  |
//	     |             key press
//	     |                  v
//	     +------------- UPDATE
// ---------------------------------------------------------------
func main() {
	// Start the app with count = 0.
	if _, err := tea.NewProgram(model{count: 0}).Run(); err != nil {
		fmt.Println("error:", err)
	}
}
