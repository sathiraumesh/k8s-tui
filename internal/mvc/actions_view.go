package mvc

import (
	"fmt"
)

// Drawing the pages that open on top of the list (actions.go): the action
// list, the confirm page with the flags, and a page of text. They are drawn
// in the same panel as the list, so they get the same scrolling, cut-off
// rows and borders.
//
//	┌─ Label worker1 ───────────────────────────────────────────────┐
//	│ set labels (key=value) or remove them (key-), several with …  │
//	│                                                               │
//	│ labels: env=prod█                                             │
//	│                                                               │
//	│ [ ] --overwrite                                               │
//	│ [ ] --dry-run=server                                          │
//	│                                                               │
//	│ This changes the cluster. y run • n back                      │
//	└─ $ kubectl label node worker1 env=prod ───────────────────────┘
//
// The text field is only there for an action that needs something typed.

// panelContent is what the panel shows: the tab's rows, or the open page.
type panelContent struct {
	rows   []string
	cursor int // the selected row, or -1 for none (a page of text)
	offset int // the first row on screen
}

// content returns what the panel shows now.
func (m Model) content() panelContent {
	switch m.overlay {
	case overlayActions:
		return panelContent{rows: m.actionRows(), cursor: m.overlayCursor}
	case overlayConfirm:
		rows, cursor := m.confirmRows()
		return panelContent{rows: rows, cursor: cursor}
	case overlayText:
		return panelContent{rows: m.text, cursor: -1, offset: m.textOffset}
	}

	if m.err != nil {
		return panelContent{rows: []string{"error: " + m.err.Error()}, cursor: m.cursor}
	}
	rows := m.rows()
	if len(rows) == 0 && m.searchText() != "" {
		return panelContent{rows: []string{"nothing matches \"" + m.searchText() + "\""}, cursor: -1}
	}
	return panelContent{rows: rows, cursor: m.cursor, offset: m.offset}
}

// cursorShown reports whether the selected row is underlined: on a page
// always, on the list only while the cursor is there.
func (m Model) cursorShown() bool {
	return m.overlay != overlayNone || m.focus == focusList
}

// actionRows lists the actions, one per row, columns lined up:
//
//	d   Describe   everything about the node, like kubectl describe
//	c   Cordon     stop new pods from being placed on the node
func (m Model) actionRows() []string {
	cells := make([][]string, len(m.actions()))
	for i, a := range m.actions() {
		cells[i] = []string{a.key, a.name, a.about}
	}
	return alignColumns(cells)
}

// confirmRows is the confirm page: what the action does, its text field,
// its flags as tick boxes, and how to go on. cursor is the row of the text
// field or flag the cursor is on, or -1 if the action has neither.
func (m Model) confirmRows() (rows []string, cursor int) {
	a := m.chosen()
	rows = []string{a.about, ""}

	cursor = -1
	if a.input != "" {
		caret := ""
		if m.typing() {
			caret = "█" // where the next letter goes
		}
		cursor = len(rows)
		rows = append(rows, a.input+": "+m.input+caret, "")
	}

	if len(m.flags) > 0 {
		if !m.typing() {
			cursor = len(rows) + m.overlayCursor - m.fieldRows()
		}
		for _, f := range m.flags {
			box := "[ ]"
			if f.on {
				box = "[x]"
			}
			row := box + " " + f.name
			if f.off != "" {
				row += "   (else " + f.off + ")"
			}
			rows = append(rows, row)
		}
		rows = append(rows, "")
	}

	if a.changes {
		rows = append(rows, "This changes the cluster. y run • n back")
	} else {
		rows = append(rows, "enter run • esc back")
	}
	return rows, cursor
}

// overlayTitle is written into the panel's top border while a page is open,
// in place of the buttons, e.g. "Cordon worker1".
func (m Model) overlayTitle() string {
	switch m.overlay {
	case overlayActions:
		if m.tabActions {
			return m.activeTab().ownTitle + " in " + m.target // e.g. "Run a pod in default"
		}
		return "Actions: " + m.target
	case overlayConfirm, overlayText:
		if m.tabActions {
			return m.chosen().name + " in " + m.target
		}
		return m.chosen().name + " " + m.target
	}
	return ""
}

// helpText is the line under the panel: the notice if there is one, or the
// keys that work where the cursor is.
func (m Model) helpText() string {
	if m.notice != "" {
		return m.notice
	}
	if m.searching {
		return "type to search • backspace delete • enter/↓ done • esc clear"
	}
	switch m.overlay {
	case overlayActions:
		keys := ""
		for _, a := range m.actions() {
			keys += fmt.Sprintf(" • %s %s", a.key, a.name)
		}
		return "↑↓ move • enter pick" + keys + " • esc back"
	case overlayConfirm:
		if m.typing() {
			return "type the " + m.chosen().input + " • backspace delete • enter/↓ flags • esc back"
		}
		if m.chosen().changes {
			return "↑↓ move • space tick • y run • n back"
		}
		return "↑↓ move • space tick • enter run • esc back"
	case overlayText:
		return "↑↓ pgup pgdn scroll • esc back"
	}
	help := "tab focus • ←→ tab • ↑↓ move"
	if len(m.actions()) > 0 {
		help += " • enter actions"
	}
	if t := m.activeTab(); len(t.own) > 0 {
		help += " • " + t.ownButton // e.g. "+ run": its key, then what it does
	}
	return help + " • w wide • r refresh • q quit"
}
