package mvc

// Key bindings.
// Each value is the string Bubble Tea returns from tea.KeyMsg.String()
// for that key press. Change a value here to rebind a key everywhere.
const (
	// Quit the app
	KeyQuit  = "q"
	KeyCtrlC = "ctrl+c"

	// Step focus: tab bar → buttons → list → tab bar
	KeyTab = "tab"

	// Press the button the cursor is on; on the list, open the row's actions;
	// in the action list, pick one. space also ticks a flag.
	KeyEnter = "enter"
	KeySpace = " "

	// Go back from a page (action list, flags, text) to the one before
	KeyEsc = "esc"

	// Search the list of the tab on screen
	KeySearch = "/"

	// Delete the last letter typed in a text field
	KeyBackspace = "backspace"

	// Run / cancel an action that changes the cluster
	KeyYes = "y"
	KeyNo  = "n"

	// Step the refresh rate from anywhere: off → 2s → 5s → 10s → off
	KeyRefreshRate = "r"

	// Turn the active tab's wide columns on/off from anywhere
	KeyWide = "w"

	// Next tab (on the buttons: next button)
	KeyRight = "right" // → arrow
	KeyL     = "l"     // vim-style right

	// Previous tab (on the buttons: previous button)
	KeyShiftTab = "shift+tab" // always the previous tab
	KeyLeft     = "left"      // ← arrow
	KeyH        = "h"         // vim-style left

	// Move cursor down the list
	KeyDown = "down" // ↓ arrow
	KeyJ    = "j"    // vim-style down

	// Move cursor up the list
	KeyUp = "up" // ↑ arrow
	KeyK  = "k"  // vim-style up

	// Move a page (the rows that fit in the panel) down / up the list
	KeyPageDown = "pgdown"
	KeyPageUp   = "pgup"
)
