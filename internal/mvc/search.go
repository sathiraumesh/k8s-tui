package mvc

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// Searching a tab's list. / (or the search button) starts typing; the list
// then shows only the rows that hold what is typed, without minding
// capitals. Each tab keeps its own search while the app runs.
//
//	list ──/──▶ typing ──enter / ↓──▶ list, searched
//	              │  └────esc──────▶ list, search cleared
//	              letters, backspace
//
// The cursor and scrolling work on the rows that match (rows()), so
// actions are for the row the cursor is on in the searched list.

// rows are the active tab's rows that match its search: all of them when
// there is no search, or before the tab has loaded ("loading..." isn't a
// row to search).
func (m Model) rows() []string {
	all := m.items[m.activeName()]
	search := strings.ToLower(m.searchText())
	if _, loaded := m.updated[m.activeName()]; search == "" || !loaded {
		return all
	}
	var found []string
	for _, row := range all {
		if strings.Contains(strings.ToLower(row), search) {
			found = append(found, row)
		}
	}
	return found
}

// searchText is what the active tab is searched for, or "".
func (m Model) searchText() string {
	return m.search[m.activeName()]
}

// setSearch makes text the active tab's search, and puts the cursor back
// on the first row: the rows it was on may be gone.
func (m Model) setSearch(text string) Model {
	m.search[m.activeName()] = text
	m.cursor, m.offset = 0, 0
	return m
}

// startSearch starts typing the search; what was searched before stays, to
// be changed. The cursor goes to the list, which is what is searched.
func (m Model) startSearch() Model {
	m.searching = true
	m.focus = focusList
	return m
}

// clearSearch stops typing and shows every row again.
func (m Model) clearSearch() Model {
	m.searching = false
	return m.setSearch("")
}

// onSearchKey handles a key while the search is typed: letters (and pasted
// text) are added, backspace deletes the last one, enter or ↓ end typing
// and keep the search, esc clears it.
func (m Model) onSearchKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyRunes, tea.KeySpace:
		return m.setSearch(m.searchText() + string(msg.Runes)), nil
	case tea.KeyBackspace:
		if r := []rune(m.searchText()); len(r) > 0 {
			return m.setSearch(string(r[:len(r)-1])), nil
		}
	case tea.KeyEnter, tea.KeyDown:
		m.searching = false
	case tea.KeyEsc:
		return m.clearSearch(), nil
	case tea.KeyCtrlC:
		return m, tea.Quit
	}
	return m, nil
}
