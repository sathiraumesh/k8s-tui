// Package styles holds the colors and Lip Gloss styles used to draw the TUI.
// A style is a formatting recipe; apply it with style.Render("text").
package styles

import "github.com/charmbracelet/lipgloss"

// Color palette.
// lipgloss.Color is a string type, so colors can be constants.
const (
	ColorPurple lipgloss.Color = "#5F5FFF" // accent: active tab, borders, selection
	ColorGray   lipgloss.Color = "#626262" // muted: inactive tabs, help text
	ColorCream  lipgloss.Color = "#FFFFD7" // text on the accent background

	// Role names: styles below use these instead of the raw colors,
	// so the theme can be changed by editing just these two lines.
	Accent = ColorPurple // highlight color for things that should stand out
	Muted  = ColorGray   // low-emphasis color for things that should fade back
)

// Styles are vars (not consts) because lipgloss.NewStyle() is a function call.
