package mvc

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

var searchKey = key(KeySearch)

// typeSearch presses / and types s, as a person would.
func typeSearch(t *testing.T, m Model, s string) Model {
	t.Helper()
	return typeText(t, press(t, m, searchKey), s)
}

// / starts the search from anywhere; letters then filter the list,
// without minding capitals.
func TestSearchFilters(t *testing.T) {
	m, _ := nodesOnScreen(t) // worker1, worker2
	m = typeSearch(t, m, "KER2")
	if !m.searching || m.searchText() != "KER2" {
		t.Fatalf("searching = %v, search = %q", m.searching, m.searchText())
	}
	if got := m.rows(); len(got) != 1 || rowName(got[0]) != "worker2" {
		t.Errorf("rows = %q, want only worker2", got)
	}
	if m.cursor != 0 {
		t.Errorf("cursor = %d, want back on the first row", m.cursor)
	}
}

// While typing, every letter goes to the search, even the keys q, w, r, j.
func TestSearchTypesKeyLetters(t *testing.T) {
	m, _ := nodesOnScreen(t)
	m = press(t, m, searchKey)
	next, cmd := m.Update(key("q"))
	if cmd != nil {
		t.Fatal("q quit while typing the search")
	}
	m = typeText(t, next.(Model), "wrj")
	if m.searchText() != "qwrj" || m.activeWide() || m.refreshEvery != defaultRefresh {
		t.Errorf("search = %q, wide = %v, refresh = %v", m.searchText(), m.activeWide(), m.refreshEvery)
	}
}

// enter ends typing and keeps the search; the actions are for the row in the
// searched list.
func TestSearchEnterKeepsItAndActsOnMatch(t *testing.T) {
	m, _ := nodesOnScreen(t)
	m = press(t, typeSearch(t, m, "worker2"), enterKey)
	if m.searching || m.searchText() != "worker2" {
		t.Fatalf("searching = %v, search = %q", m.searching, m.searchText())
	}
	if m = press(t, m, enterKey); m.overlay != overlayActions || m.target != "worker2" {
		t.Errorf("enter on the match: overlay = %d, target = %q", m.overlay, m.target)
	}
}

// esc clears the search, while typing and after.
func TestSearchEscClears(t *testing.T) {
	m, _ := nodesOnScreen(t)
	if m = press(t, typeSearch(t, m, "1"), escKey); m.searching || m.searchText() != "" || len(m.rows()) != 2 {
		t.Errorf("esc while typing: searching = %v, search = %q", m.searching, m.searchText())
	}
	m = press(t, typeSearch(t, m, "1"), enterKey, escKey)
	if m.searchText() != "" {
		t.Errorf("esc after: search = %q, want cleared", m.searchText())
	}
}

func TestSearchBackspace(t *testing.T) {
	m, _ := nodesOnScreen(t)
	m = press(t, typeSearch(t, m, "work2"), tea.KeyMsg{Type: tea.KeyBackspace})
	if m.searchText() != "work" || len(m.rows()) != 2 {
		t.Errorf("search = %q, rows = %d", m.searchText(), len(m.rows()))
	}
}

func TestSearchNoMatch(t *testing.T) {
	m, _ := nodesOnScreen(t)
	m = press(t, typeSearch(t, m, "zzz"), enterKey)
	if !strings.Contains(m.View(), `nothing matches "zzz"`) {
		t.Errorf("view:\n%s", m.View())
	}
	if m = press(t, m, enterKey); m.overlay != overlayNone {
		t.Error("enter opened actions with nothing matching")
	}
}

// Each tab keeps its own search.
func TestSearchPerTab(t *testing.T) {
	m, _ := nodesOnScreen(t)
	m = press(t, typeSearch(t, m, "worker1"), enterKey, right) // to Pods
	if m.searchText() != "" {
		t.Errorf("Pods search = %q, want none", m.searchText())
	}
	if m = press(t, m, left); m.searchText() != "worker1" {
		t.Errorf("Nodes search = %q after coming back, want worker1", m.searchText())
	}
}

// The search button starts it too, and shows the search, green.
func TestSearchButton(t *testing.T) {
	m, _ := nodesOnScreen(t)
	m.focus, m.button = focusButtons, 2
	m = press(t, m, enterKey)
	if !m.searching || m.focus != focusList {
		t.Fatalf("search button: searching = %v, focus = %d", m.searching, m.focus)
	}
	b := buttonNamed(t, typeText(t, m, "abc"), buttonSearch)
	if b.label != "search: abc█" || !b.on {
		t.Errorf("button = %+v while typing", b)
	}
	if b = buttonNamed(t, New(nil), buttonSearch); b.label != "/ search" || b.on {
		t.Errorf("button = %+v with no search", b)
	}
}

// A long search shows its end on the button, so the button stays small.
func TestSearchButtonLong(t *testing.T) {
	m, _ := nodesOnScreen(t)
	m = press(t, typeSearch(t, m, "abcdefghijklmnopqrstuvwxyz"), enterKey)
	b := buttonNamed(t, m, buttonSearch)
	if b.label != "search: …lmnopqrstuvwxyz" {
		t.Errorf("label = %q", b.label)
	}
}

// The bottom border shows the search as the shell would do it.
func TestSearchCommand(t *testing.T) {
	m, _ := nodesOnScreen(t)
	if m = typeSearch(t, m, "worker2"); m.kubectlCommand() != "kubectl get nodes | grep -i worker2" {
		t.Errorf("command = %q", m.kubectlCommand())
	}
	if m = typeText(t, m, " x"); m.kubectlCommand() != "kubectl get nodes | grep -i 'worker2 x'" {
		t.Errorf("command = %q", m.kubectlCommand())
	}
}

// A refresh with fewer matching rows keeps the cursor on one of them.
func TestSearchRefreshKeepsCursor(t *testing.T) {
	m, _ := nodesOnScreen(t)
	m = press(t, m, itemsLoadedMsg{tab: tabNodes, rows: []string{"a1", "a2", "a3", "b1"}, at: m.updated[tabNodes]})
	m = press(t, typeSearch(t, m, "a"), enterKey, down, down) // on a3
	m = press(t, m, itemsLoadedMsg{tab: tabNodes, rows: []string{"a1", "b1"}, at: m.updated[tabNodes]})
	if m.cursor != 0 || rowName(m.rows()[m.cursor]) != "a1" {
		t.Errorf("cursor = %d on %q", m.cursor, m.rows())
	}
}

// "loading..." isn't searched: it shows until the rows arrive.
func TestSearchBeforeLoad(t *testing.T) {
	m := typeSearch(t, sized(t, New(nil), 100, 24), "x")
	if got := m.rows(); len(got) != 1 || got[0] != "loading..." {
		t.Errorf("rows = %q", got)
	}
}
