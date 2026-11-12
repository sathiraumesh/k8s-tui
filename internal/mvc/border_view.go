package mvc

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Lip Gloss can't put text into a border, so the panel is drawn with a
// plain border and then one of its border lines is replaced by one with
// text set into it, on the left and on the right:
//
//	┌─[wide: off]───[refresh: 5s]──────────────── updated 14:30:05 ─┐   top
//	│ ...                                                           │
//	└─ $ kubectl get nodes ───────────────────────────── 4-9 of 30 ─┘   bottom
//
// The top border holds the buttons (secondary_bar_view.go); the bottom one
// the kubectl command and the scroll position (scroll_view.go).

// edge is which border line of the panel to write into.
type edge int

const (
	edgeTop edge = iota
	edgeBottom
)

// withBorderText redraws the top or bottom border line of box with left set
// in after its left corner and right before its right corner, joined by
// border pieces so the line is exactly width wide:
//
//	┌─<left>──────────────<right>─┐
//
// left and right are drawn as they are given (already styled); right may
// be "". If right doesn't fit next to left, it is left out; if left doesn't
// fit either, box is returned as it is, with its plain border.
func withBorderText(box string, e edge, left, right string, width int) string {
	b := lipgloss.NormalBorder()
	leftCorner, rightCorner, piece := b.TopLeft, b.TopRight, b.Top
	if e == edgeBottom {
		leftCorner, rightCorner, piece = b.BottomLeft, b.BottomRight, b.Bottom
	}

	// The right part ends with one border piece before the corner: "<right>─┐".
	tail := ""
	if right != "" {
		tail = right + borderLine.Render(piece)
	}
	fill := width - 3 - lipgloss.Width(left) - lipgloss.Width(tail) // 3 = "┌─" and "┐"
	if fill < 1 && tail != "" {
		tail = "" // no room for the right part: keep just the left
		fill = width - 3 - lipgloss.Width(left)
	}
	if fill < 1 {
		return box
	}
	line := borderLine.Render(leftCorner+piece) + left +
		borderLine.Render(strings.Repeat(piece, fill)) + tail + borderLine.Render(rightCorner)

	if e == edgeTop {
		_, rest, ok := strings.Cut(box, "\n")
		if !ok {
			return box
		}
		return line + "\n" + rest
	}
	i := strings.LastIndex(box, "\n")
	if i < 0 {
		return box
	}
	return box[:i+1] + line
}

// borderLabel is text set into a border with a space on each side, in the
// given style, e.g. " updated 14:30:05 ".
func borderLabel(style lipgloss.Style, text string) string {
	if text == "" {
		return ""
	}
	return style.Render(" " + text + " ")
}
