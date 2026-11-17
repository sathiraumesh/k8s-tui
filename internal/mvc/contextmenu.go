package mvc

import (
	"strings"

	"k8s-tui/internal/styles"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// The right-click menu: a small box drawn where the mouse was, over
// whatever is on screen, like a terminal's own. Its items are what makes
// sense there:
//
//	Copy           the text selected with a drag
//	Copy row       the row clicked on, as it is
//	Copy all       every row of the list or page, also those off screen
//	Copy command   the kubectl command on the bottom border
//	Back           like esc, when a page or a view of More is open
//
// A click or enter picks one; esc, a click elsewhere or another right
// click closes it. Copying is done with OSC 52 (clipboard.go).

// menuItem is one item of the menu.
type menuItem struct {
	label string
	do    func(m Model) (Model, tea.Cmd)
}

// contextMenu is an open right-click menu, its top left corner at at.
type contextMenu struct {
	at     point
	items  []menuItem
	cursor int
}

// openMenu opens the menu for a right click at (x, y).
func (m Model) openMenu(x, y int) Model {
	var items []menuItem
	if m.sel != nil {
		text := m.selectedText()
		items = append(items, menuItem{"Copy", func(m Model) (Model, tea.Cmd) {
			return m, copyToClipboard(text)
		}})
	}
	c := m.content()
	if i := m.rowAt(y); i >= 0 && i < len(c.rows) {
		row := strings.TrimRight(c.rows[i], " ")
		items = append(items, menuItem{"Copy row", func(m Model) (Model, tea.Cmd) {
			return m, copyToClipboard(row)
		}})
	}
	if len(c.rows) > 0 {
		all := strings.Join(c.rows, "\n")
		items = append(items, menuItem{"Copy all", func(m Model) (Model, tea.Cmd) {
			return m, copyToClipboard(all)
		}})
	}
	if cmd := m.kubectlCommand(); cmd != "" {
		items = append(items, menuItem{"Copy command", func(m Model) (Model, tea.Cmd) {
			return m, copyToClipboard(cmd)
		}})
	}
	if m.overlay != overlayNone || m.inKind() || m.searching {
		items = append(items, menuItem{"Back", func(m Model) (Model, tea.Cmd) {
			next, cmd := m.back()
			return next.(Model), cmd
		}})
	}
	m.menu = &contextMenu{at: point{x, y}, items: items}
	return m
}

// menuStyle is the menu's box.
var menuStyle = lipgloss.NewStyle().
	Border(lipgloss.NormalBorder()).
	BorderForeground(styles.Accent).
	Padding(0, 1)

// box draws the menu, the item under its cursor selected.
func (c contextMenu) box() string {
	width := 0
	for _, it := range c.items {
		width = max(width, lipgloss.Width(it.label))
	}
	lines := make([]string, len(c.items))
	for i, it := range c.items {
		lines[i] = it.label + strings.Repeat(" ", width-lipgloss.Width(it.label))
		if i == c.cursor {
			lines[i] = Selected.Render(lines[i])
		}
	}
	return menuStyle.Render(strings.Join(lines, "\n"))
}

// place is where the box goes on a screen w×h: at the click, moved left
// and up as far as it must to fit.
func (c contextMenu) place(w, h int) point {
	box := c.box()
	return point{
		x: max(min(c.at.x, w-lipgloss.Width(box)), 0),
		y: max(min(c.at.y, h-lipgloss.Height(box)), 0),
	}
}

// draw draws the menu over the screen's lines.
func (c contextMenu) draw(lines []string, w, h int) []string {
	p := c.place(w, h)
	for i, row := range strings.Split(c.box(), "\n") {
		y := p.y + i
		if y >= len(lines) {
			break
		}
		line := padTo(lines[y], w)
		end := p.x + lipgloss.Width(row)
		lines[y] = ansi.Cut(line, 0, p.x) + row + ansi.Cut(line, end, w)
	}
	return lines
}

// itemAt is the index of the item at (x, y) of the screen, or -1.
func (m Model) menuItemAt(x, y int) int {
	c := m.menu
	p := c.place(m.width, m.height)
	if x < p.x || x >= p.x+lipgloss.Width(c.box()) {
		return -1
	}
	i := y - p.y - 1 // the top border
	if i < 0 || i >= len(c.items) {
		return -1
	}
	return i
}

// pickMenu does item i of the menu and closes it.
func (m Model) pickMenu(i int) (Model, tea.Cmd) {
	it := m.menu.items[i]
	m.menu = nil
	m.sel = nil
	return it.do(m)
}

// onMenuKey handles a key while the menu is open: ↑ ↓ move, enter picks,
// esc (or any other key) closes it.
func (m Model) onMenuKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case KeyUp, KeyK:
		m.menu.cursor = max(m.menu.cursor-1, 0)
	case KeyDown, KeyJ:
		m.menu.cursor = min(m.menu.cursor+1, len(m.menu.items)-1)
	case KeyEnter:
		return m.pickMenu(m.menu.cursor)
	case KeyCtrlC:
		return m, tea.Quit
	default:
		m.menu = nil
	}
	return m, nil
}
