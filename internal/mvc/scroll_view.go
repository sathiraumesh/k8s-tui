package mvc

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Drawing the scroll position on the panel's bottom border:
//
//	└─ 4-9 of 30 ─────────────────────────────────────────────────┘
//
// It is only shown when the list has more rows than fit. The scroll logic
// is in scroll.go.

// scrollInfo describes which rows are on screen, e.g. "4-9 of 30", or ""
// when every row fits and there is nothing to scroll.
func scrollInfo(first, end, total, h int) string {
	if total <= h {
		return ""
	}
	return fmt.Sprintf("%d-%d of %d", first+1, end, total)
}

// withScrollInfo redraws the last line of box, its bottom border, with info
// set into it on the left. width is the box's full width. If info is "" or
// doesn't fit, box is returned as it is.
func withScrollInfo(box, info string, width int) string {
	if info == "" {
		return box
	}
	b := lipgloss.NormalBorder()
	label := Help.Render(" " + info + " ")
	fill := width - 3 - lipgloss.Width(label) // 3 = "└─" and "┘"
	if fill < 1 {
		return box
	}
	bottom := borderLine.Render(b.BottomLeft+b.Bottom) + label +
		borderLine.Render(strings.Repeat(b.Bottom, fill)+b.BottomRight)

	i := strings.LastIndex(box, "\n")
	if i < 0 {
		return box
	}
	return box[:i+1] + bottom
}
