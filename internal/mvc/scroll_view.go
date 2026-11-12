package mvc

import (
	"fmt"

	"k8s-tui/internal/styles"

	"github.com/charmbracelet/lipgloss"
)

// Drawing the panel's bottom border: the kubectl command behind the list on
// the left, and the scroll position on the right while the list is longer
// than the panel:
//
//	└─ $ kubectl get nodes ───────────────────────────── 4-9 of 30 ─┘
//
// The scroll logic is in scroll.go; the command comes from kubectlCommand
// (tabs.go).

// commandStyle draws the kubectl command: light text, so it stands out from
// the gray scroll position.
var commandStyle = lipgloss.NewStyle().Foreground(styles.Text)

// scrollInfo describes which rows are on screen, e.g. "4-9 of 30", or ""
// when every row fits and there is nothing to scroll.
func scrollInfo(first, end, total, h int) string {
	if total <= h {
		return ""
	}
	return fmt.Sprintf("%d-%d of %d", first+1, end, total)
}

// withCommand redraws the last line of box, its bottom border, with the
// kubectl command on the left ("$ kubectl get nodes") and info on the right
// (the scroll position; may be ""). width is the box's full width.
//
// A command too long for the border is cut off at its end. If there is no
// room for info next to it, info is left out. With no command, only info is
// shown.
func withCommand(box, command, info string, width int) string {
	left := ""
	if command != "" {
		// Room for the command: the width minus the corners, a border piece
		// and a space on each side ("└─ " and " ─┘" = 6).
		text := lipgloss.NewStyle().MaxWidth(max(width-6, 0)).Render("$ " + command)
		left = borderLabel(commandStyle, text)
	}
	if left == "" {
		return withBorderText(box, edgeBottom, borderLabel(Help, info), "", width)
	}
	return withBorderText(box, edgeBottom, left, borderLabel(Help, info), width)
}
