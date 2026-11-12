package mvc

// Key bindings.
// Each value is the string Bubble Tea returns from tea.KeyMsg.String()
// for that key press. Change a value here to rebind a key everywhere.
const (
	// Quit the app
	KeyQuit  = "q"
	KeyCtrlC = "ctrl+c"

	// Switch focus between the tab bar and the list
	KeyTab = "tab"

	// Auto refresh: turn it on/off, and cycle the rate 2s → 5s → 10s
	KeyAutoRefresh = "a"
	KeyRefreshRate = "f"

	// Next tab
	KeyRight = "right" // → arrow
	KeyL     = "l"     // vim-style right

	// Previous tab
	KeyShiftTab = "shift+tab"
	KeyLeft     = "left" // ← arrow
	KeyH        = "h"    // vim-style left

	// Move cursor down the list
	KeyDown = "down" // ↓ arrow
	KeyJ    = "j"    // vim-style down

	// Move cursor up the list
	KeyUp = "up" // ↑ arrow
	KeyK  = "k"  // vim-style up
)
