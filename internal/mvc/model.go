package mvc

import (
	"time"

	k8s "k8s-tui/internal/kubernetes"

	tea "github.com/charmbracelet/bubbletea"
)

// focus says which part of the screen the cursor is on.
type focus int

const (
	focusTabs    focus = iota // cursor is on the tab bar
	focusButtons              // cursor is on the buttons in the panel's top border
	focusList                 // cursor is on the list in the panel
)

// Model is everything the app remembers between key presses.
// Update changes it; View reads it to draw the screen.
type Model struct {
	// tabs are the tabs in the tab bar, left to right (see tabs.go).
	tabs []tab

	// items holds the rows to list for each tab, keyed by tab name.
	// e.g. items["Nodes"] = ["controlplane  Ready  v1.37.2", ...]
	// Every tab starts as ["loading..."] until its loader sends its rows.
	items map[string][]string

	// active is the index of the selected tab in tabs: 0 is the first.
	// Changed by ← / → (on the tabs or the list) and shift+tab.
	active int

	// cursor is the index of the highlighted row in the panel's list.
	// Changed by ↑ / ↓, and reset to 0 when switching tabs.
	cursor int

	// offset is the index of the first row shown in the panel: how far the
	// list is scrolled down. Update keeps the cursor's row on screen (see
	// scroll.go). Reset to 0 when switching tabs.
	offset int

	// width and height are the terminal size in cells (columns and rows).
	// Bubble Tea sends them in a tea.WindowSizeMsg at start and on every resize.
	// They are 0 until that first message arrives, so View shows "loading...".
	width  int
	height int

	// focus is where the cursor is: the tab bar, the buttons or the list.
	// Starts as focusTabs. ↓ goes tabs → buttons → list, ↑ goes back up
	// (from the list only at its top row), and tab steps through all three.
	focus focus

	// button is the index of the button the cursor is on, while focus is
	// focusButtons. Changed by ← / →.
	button int

	// client talks to the cluster. nil in tests, so nothing is loaded there.
	client *k8s.Client

	// err is the last error from the cluster, shown in the panel. nil when all is fine.
	err error

	// refreshEvery is how often the active tab reloads: one of refreshRates.
	// 0 means auto refresh is off. Stepped by the refresh button or KeyRefreshRate.
	refreshEvery time.Duration

	// tickGen is the generation of the running timer (see refreshTickMsg).
	tickGen int

	// updated is when each tab's data was last loaded, keyed by tab name.
	// A tab that was never loaded has no entry.
	updated map[string]time.Time

	// wide is whether a tab shows its wide columns (like `kubectl get -o wide`),
	// keyed by tab name. Each tab has its own setting; a missing entry is off.
	// Toggled by the wide button or KeyWide.
	wide map[string]bool
}

// New returns a Model with its starting state: the tabs of allTabs, each
// showing "loading..." until its rows arrive.
// client may be nil (in tests); then nothing is loaded.
func New(client *k8s.Client) Model {
	tabs := allTabs()
	items := make(map[string][]string, len(tabs))
	for _, t := range tabs {
		items[t.name] = []string{"loading..."}
	}
	return Model{
		client:       client,
		tabs:         tabs,
		items:        items,
		refreshEvery: defaultRefresh,
		updated:      map[string]time.Time{},
		wide:         map[string]bool{},
	}
}

// activeWide is whether the active tab shows its wide columns.
func (m Model) activeWide() bool {
	return m.wide[m.activeName()]
}

// Init runs once at start: load the active tab now, and start the refresh timer.
func (m Model) Init() tea.Cmd {
	if m.client == nil {
		return nil // tests: no cluster, no timer
	}
	return tea.Batch(m.loadActive(), m.startTimer())
}
