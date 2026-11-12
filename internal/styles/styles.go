// Package styles holds the colors and Lip Gloss styles used to draw the TUI.
// A style is a formatting recipe; apply it with style.Render("text").
package styles

import "github.com/charmbracelet/lipgloss"

// Color palette.
// lipgloss.Color is a string type, so colors can be constants.
const (
	ColorBackground lipgloss.Color = "#1F1F1F" // editor background
	ColorForeground lipgloss.Color = "#CCCCCC" // editor text
	ColorBlue       lipgloss.Color = "#0078D4" // VS Code accent blue (buttons, focus)
	ColorWhite      lipgloss.Color = "#FFFFFF" // text on blue
	ColorGray       lipgloss.Color = "#9D9D9D" // secondary / description text
	ColorSelection  lipgloss.Color = "#04395E" // selected row in lists (Explorer)
	ColorBorder     lipgloss.Color = "#2B2B2B" // panel and tab borders

	// Role names: styles use these instead of the raw colors,
	// so the theme can be changed by editing just these lines.
	Accent   = ColorBlue       // highlight: active tab, borders
	Muted    = ColorGray       // fade back: inactive tabs, help text
	OnAccent = ColorWhite      // text drawn on top of Accent
	Shade    = ColorSelection  // background of the selected row
	Text     = ColorForeground // normal text
	Bg       = ColorBackground // screen background
)
