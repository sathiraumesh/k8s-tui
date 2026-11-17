package mvc

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// Selecting text with the mouse, as in the terminal: press, drag, let go.
// The text dragged over is shown reversed and copied (clipboard.go) when the
// button is let go; it stays shown until the next key or click. A press and
// release without dragging is a click (mouse.go).
//
//	press ──drag──▶ selection grows ──release──▶ copied, shown
//	   └──release──▶ a click
//
// It selects like a terminal does, line by line: from where the drag
// started to the end of that line, every line between, and the last line
// up to where it ended. The text is what is on the screen, borders and all.

// point is a cell of the screen: its column and line.
type point struct{ x, y int }

// selection is the text being, or last, selected: from from to to (either
// way round).
type selection struct {
	from, to point
}

// ordered is the selection with from first, in reading order.
func (s selection) ordered() (a, b point) {
	a, b = s.from, s.to
	if b.y < a.y || (b.y == a.y && b.x < a.x) {
		a, b = b, a
	}
	return a, b
}

// span is the columns [start, end) of line y that s covers, of a screen
// width wide, or ok false if it doesn't cover the line.
func (s selection) span(y, width int) (start, end int, ok bool) {
	a, b := s.ordered()
	if y < a.y || y > b.y {
		return 0, 0, false
	}
	start, end = 0, width
	if y == a.y {
		start = a.x
	}
	if y == b.y {
		end = b.x + 1
	}
	return start, min(end, width), start < end
}

// selectedStyle draws the selected text: reversed, like a terminal does.
var selectedStyle = lipgloss.NewStyle().Reverse(true)

// highlight draws s onto the screen's lines.
func (s selection) highlight(lines []string, width int) []string {
	for y := range lines {
		start, end, ok := s.span(y, width)
		if !ok {
			continue
		}
		line := padTo(lines[y], width)
		lines[y] = ansi.Cut(line, 0, start) + selectedStyle.Render(ansi.Strip(ansi.Cut(line, start, end))) + ansi.Cut(line, end, width)
	}
	return lines
}

// text is what s covers of the screen's lines, without colors, each line
// without the spaces at its end.
func (s selection) text(lines []string, width int) string {
	var out []string
	for y := range lines {
		start, end, ok := s.span(y, width)
		if !ok {
			continue
		}
		out = append(out, strings.TrimRight(ansi.Strip(ansi.Cut(padTo(lines[y], width), start, end)), " "))
	}
	return strings.Join(out, "\n")
}

// padTo pads line with spaces to width cells, so a part of it past its end
// can be cut out and drawn over.
func padTo(line string, width int) string {
	if w := ansi.StringWidth(line); w < width {
		return line + strings.Repeat(" ", width-w)
	}
	return line
}
