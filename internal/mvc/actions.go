package mvc

import (
	"context"
	"fmt"
	"strings"

	k8s "k8s-tui/internal/kubernetes"

	tea "github.com/charmbracelet/bubbletea"
)

// Actions: things that can be done to the row the cursor is on, such as
// cordoning a node. enter on a row opens the tab's list of actions; an
// action is then picked with enter or its key. Each tab's actions are in
// its entry in allTabs, so adding an action is one entry in a table.
//
//	list ──enter──▶ actions ──enter / key──▶ confirm ──y / enter──▶ run
//	                                                               │
//	                         a change: back to the list, reloaded ◀┤
//	                         a read: a page of text (Describe)    ◀┘
//
// How the pages are drawn is in actions_view.go.

// overlay is a page open on top of the tab's list.
type overlay int

const (
	overlayNone    overlay = iota // the tab's list
	overlayActions                // the actions for the selected row
	overlayConfirm                // the chosen action's flags, and y/n or enter to run it
	overlayText                   // the text a read action returned, e.g. Describe
)

// flag is one kubectl option of an action, shown as a tick box. It is
// written as kubectl takes it, e.g. "--dry-run=server", and added to the
// command when ticked.
type flag struct {
	name string
	on   bool // ticked; the action's list gives the default
}

// action is something that can be done to one row.
type action struct {
	key   string // picks it in the action list, e.g. "c"
	name  string // e.g. "Cordon"
	about string // one line: what it does

	// changes is whether it changes the cluster. Such an action always asks
	// first, and only y runs it. One that only reads runs with enter.
	changes bool

	flags []flag

	// command is the kubectl command, with %s where the row's name goes,
	// e.g. "kubectl cordon %s". Ticked flags are added after it.
	command string

	// run does the action to target with the given flags. It sends an
	// actionDoneMsg when it is done.
	run func(c *k8s.Client, target string, flags []flag) tea.Cmd
}

// commandFor is the kubectl command for running a on target with flags,
// e.g. "kubectl cordon worker1 --dry-run=server".
func (a action) commandFor(target string, flags []flag) string {
	parts := []string{fmt.Sprintf(a.command, target)}
	for _, f := range flags {
		if f.on {
			parts = append(parts, f.name)
		}
	}
	return strings.Join(parts, " ")
}

// ticked reports whether the flag called name is ticked.
func ticked(flags []flag, name string) bool {
	for _, f := range flags {
		if f.name == name && f.on {
			return true
		}
	}
	return false
}

// flagDryRun is the flag of a change that lets the cluster check it without
// saving it. change reports a run with it ticked as a dry run.
const flagDryRun = "--dry-run=server"

// read returns the command for an action that only reads: it runs get and
// sends its text, to be shown on a page.
func read(name, target string, get func(context.Context) (string, error)) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), apiTimeout)
		defer cancel()
		text, err := get(ctx)
		return actionDoneMsg{action: name, target: target, text: text, err: err}
	}
}

// change returns the command for an action that changes the cluster: it
// runs do and reports whether it worked.
func change(name, target string, flags []flag, do func(context.Context) error) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), apiTimeout)
		defer cancel()
		return actionDoneMsg{action: name, target: target, changed: true, dryRun: ticked(flags, flagDryRun), err: do(ctx)}
	}
}

// ---------- the model's side ----------

// rowName is the name of the thing a row shows: its first column.
func rowName(row string) string {
	if fields := strings.Fields(row); len(fields) > 0 {
		return fields[0]
	}
	return ""
}

// actions are the active tab's actions.
func (m Model) actions() []action {
	return m.activeTab().actions
}

// chosen is the action picked in the action list.
func (m Model) chosen() action {
	return m.actions()[m.chosenAction]
}

// canOpenActions reports whether enter on the list opens the action list:
// the tab has actions, and its rows have loaded without an error.
func (m Model) canOpenActions() bool {
	_, loaded := m.updated[m.activeName()]
	return len(m.actions()) > 0 && loaded && m.err == nil && len(m.items[m.activeName()]) > 0
}

// openActions opens the action list for the row the cursor is on.
func (m Model) openActions() Model {
	m.target = rowName(m.items[m.activeName()][m.cursor])
	m.overlay = overlayActions
	m.overlayCursor = 0
	return m
}

// pick chooses the action at index i of the action list. It opens the
// confirm page, where its flags can be ticked; an action that only reads
// and has no flags runs straight away.
func (m Model) pick(i int) (Model, tea.Cmd) {
	m.chosenAction = i
	a := m.chosen()
	m.flags = append([]flag(nil), a.flags...) // a copy: ticking must not change the table
	if !a.changes && len(a.flags) == 0 {
		return m.runChosen()
	}
	m.overlay = overlayConfirm
	m.overlayCursor = 0
	return m, nil
}

// runChosen runs the chosen action with the ticked flags.
func (m Model) runChosen() (Model, tea.Cmd) {
	a := m.chosen()
	m.notice = "running: " + a.commandFor(m.target, m.flags)
	if a.changes {
		m.overlay = overlayNone // back to the list while it runs
	}
	if m.client == nil {
		return m, nil // tests without a cluster
	}
	return m, a.run(m.client, m.target, m.flags)
}

// closeOverlay goes back to the tab's list.
func (m Model) closeOverlay() Model {
	m.overlay = overlayNone
	m.focus = focusList
	return m
}

// onActionDone handles the end of an action: a read opens its text, a
// change reloads the tab so the list shows it. Either way, the line under
// the panel says how it went.
func (m Model) onActionDone(msg actionDoneMsg) (Model, tea.Cmd) {
	what := msg.action + " " + msg.target
	switch {
	case msg.err != nil:
		m.notice = what + " failed: " + msg.err.Error()
		return m, nil
	case msg.changed:
		m.notice = what + ": done"
		if msg.dryRun {
			m.notice = what + ": dry run passed, nothing changed"
		}
		return m, m.loadActive()
	}
	m.overlay = overlayText
	m.text = strings.Split(strings.TrimRight(msg.text, "\n"), "\n")
	m.textOffset = 0
	m.notice = ""
	return m, nil
}

// onOverlayKey handles a key while a page is open on top of the list.
// Keys the page doesn't use are ignored, except quit.
func (m Model) onOverlayKey(key string) (tea.Model, tea.Cmd) {
	if key == KeyQuit || key == KeyCtrlC {
		return m, tea.Quit
	}

	switch m.overlay {
	case overlayActions:
		switch key {
		case KeyUp, KeyK:
			m.overlayCursor = max(m.overlayCursor-1, 0)
		case KeyDown, KeyJ:
			m.overlayCursor = min(m.overlayCursor+1, len(m.actions())-1)
		case KeyEnter:
			return m.pick(m.overlayCursor)
		case KeyEsc:
			return m.closeOverlay(), nil
		default:
			for i, a := range m.actions() {
				if key == a.key {
					return m.pick(i)
				}
			}
		}

	case overlayConfirm:
		a := m.chosen()
		switch key {
		case KeyUp, KeyK:
			m.overlayCursor = max(m.overlayCursor-1, 0)
		case KeyDown, KeyJ:
			m.overlayCursor = min(m.overlayCursor+1, max(len(m.flags)-1, 0))
		case KeySpace:
			if len(m.flags) > 0 {
				m.flags[m.overlayCursor].on = !m.flags[m.overlayCursor].on
			}
		case KeyYes:
			if a.changes {
				return m.runChosen()
			}
		case KeyEnter:
			if !a.changes {
				return m.runChosen() // a change needs y: enter is too easy to press
			}
		case KeyNo, KeyEsc:
			m.overlay = overlayActions // back to the list of actions
			m.overlayCursor = m.chosenAction
		}

	case overlayText:
		h := m.listHeight()
		last := max(len(m.text)-h, 0)
		switch key {
		case KeyUp, KeyK:
			m.textOffset = max(m.textOffset-1, 0)
		case KeyDown, KeyJ:
			m.textOffset = min(m.textOffset+1, last)
		case KeyPageUp:
			m.textOffset = max(m.textOffset-h, 0)
		case KeyPageDown:
			m.textOffset = min(m.textOffset+h, last)
		case KeyEsc:
			m.overlay = overlayActions
			m.overlayCursor = m.chosenAction
		}
	}
	return m, nil
}
