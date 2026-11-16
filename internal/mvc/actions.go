package mvc

import (
	"k8s-tui/internal/mvc/commands"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// Actions: things that can be done to the row the cursor is on, such as
// cordoning a node. enter on a row opens the tab's list of actions; an
// action is then picked with enter or its key. Each tab's actions are in
// its entry in allTabs, so adding an action is one entry in a table.
//
//	list ──enter──▶ actions ──enter / key──▶ confirm ──y / enter──▶ run
//	                                         (type, tick flags)
//	                                                               │
//	                         a change: back to the list, reloaded ◀┤
//	                         a dry run: what it would change      ◀┤
//	                         a read: a page of text (Describe)    ◀┤
//	                         a shell or an editor: kubectl has the
//	                                  terminal, back to the list when it ends ◀┘
//
// How the pages are drawn is in actions_view.go.

// overlay is a page open on top of the tab's list.
type overlay int

const (
	overlayNone       overlay = iota // the tab's list
	overlayActions                   // the actions for the selected row
	overlayConfirm                   // the chosen action's flags, and y/n or enter to run it
	overlayText                      // the text a read action returned, e.g. Describe
	overlayNamespaces                // the namespaces to pick from (namespace.go)
	overlayContainers                // the containers of a pod to pick from (containers.go)
)

// ---------- the model's side ----------

// rowTarget is what the actions of row are for (see tab.rowTarget).
func (m Model) rowTarget(row string) string {
	if f := m.activeTab().rowTarget; f != nil {
		return f(row, m.tabNamespace())
	}
	return commands.RowName(row)
}

// commandTarget is the target as the kubectl commands write it, e.g.
// "web -n default" (see tab.commandTarget). The tab's own actions get
// their target as it is: it is already what their commands take.
func (m Model) commandTarget() string {
	if f := m.activeTab().commandTarget; f != nil && !m.tabActions {
		return f(m.target)
	}
	return m.target
}

// actions are the active tab's actions for a row, or while its own action
// list is open (tabActions), the tab's own actions, e.g. Run on Pods.
func (m Model) actions() []commands.Action {
	if m.tabActions {
		return m.activeTab().own
	}
	return m.activeTab().actions
}

// chosen is the action picked in the action list.
func (m Model) chosen() commands.Action {
	return m.actions()[m.chosenAction]
}

// canOpenActions reports whether enter on the list opens the action list:
// the tab has actions, and its rows have loaded without an error.
func (m Model) canOpenActions() bool {
	_, loaded := m.updated[m.activeName()]
	return len(m.actions()) > 0 && loaded && m.err == nil && len(m.rows()) > 0
}

// openActions opens the action list for the row the cursor is on.
func (m Model) openActions() Model {
	m.target = m.rowTarget(m.rows()[m.cursor])
	m.tabActions = false
	m.overlay = overlayActions
	m.overlayCursor = 0
	return m
}

// openOwnActions opens the tab's own action list, e.g. the images to run a
// pod from on Pods. Its target is the tab's ownTarget, e.g. the namespace.
func (m Model) openOwnActions() Model {
	t := m.activeTab()
	if len(t.own) == 0 {
		return m
	}
	m.target = t.ownTarget
	if t.namespaced && m.namespace != allNamespaces {
		m.target = m.namespace // e.g. run the pod in the namespace picked
	}
	m.tabActions = true
	m.overlay = overlayActions
	m.overlayCursor = 0
	m.focus = focusList
	return m
}

// pick chooses the action at index i of the action list. It opens the
// confirm page, where its flags can be ticked; an action that only reads
// and has no flags runs straight away.
func (m Model) pick(i int) (Model, tea.Cmd) {
	m.chosenAction = i
	a := m.chosen()
	m.flags = append([]commands.Flag(nil), a.Flags...) // a copy: ticking must not change the table
	m.input = a.InputDefault
	if a.Containers != nil && m.client != nil {
		return m, loadContainers(m.client, m.target, a.Containers, a.OneContainer)
	}
	return m.afterPick()
}

// afterPick goes on from the action picked: to its confirm page, or, for
// an action that only reads and has nothing to set, straight to running.
func (m Model) afterPick() (Model, tea.Cmd) {
	a := m.chosen()
	if !a.Changes && len(a.Flags) == 0 && a.Input == "" {
		return m.runChosen()
	}
	m.overlay = overlayConfirm
	m.overlayCursor = 0
	return m, nil
}

// runChosen runs the chosen action with what was typed and the ticked
// flags. An action with a text field doesn't run while it is empty.
func (m Model) runChosen() (Model, tea.Cmd) {
	a := m.chosen()
	if a.Input != "" && strings.TrimSpace(m.input) == "" {
		m.notice = "type the " + a.Input + " first"
		m.overlayCursor = 0 // on the text field
		return m, nil
	}
	m.notice = "running: " + a.CommandFor(m.commandTarget(), m.input, m.flags)
	if a.Changes {
		m.overlay = overlayNone // back to the list while it runs
		m.tabActions = false
	}
	if m.client == nil {
		return m, nil // tests without a cluster
	}
	return m, a.Run(m.client, m.target, m.input, m.flags)
}

// closeOverlay goes back to the tab's list.
func (m Model) closeOverlay() Model {
	m.overlay = overlayNone
	m.tabActions = false
	m.focus = focusList
	return m
}

// onActionDone handles the end of an action: a read opens its text, a
// change reloads the tab so the list shows it. Either way, the line under
// the panel says how it went.
func (m Model) onActionDone(msg commands.ActionDoneMsg) (Model, tea.Cmd) {
	what := msg.Action + " " + msg.Target
	switch {
	case msg.Err != nil:
		m.notice = what + " failed: " + msg.Err.Error()
		return m, nil
	case msg.Terminal:
		m = m.closeOverlay() // an action that only reads, like Edit, ran from the action list
		m.notice = msg.Closed
		return m, m.loadActive()
	case msg.Changed && msg.DryRun:
		m.notice = what + ": dry run passed, nothing changed"
		if msg.Text != "" {
			m = m.showText(msg.Text) // what it would change
			m.textKind = textKindOf(msg)
		}
		return m, nil // nothing changed, so nothing to reload
	case msg.Changed:
		m.notice = what + ": done"
		return m, m.loadActive()
	}
	m.notice = ""
	m = m.showText(msg.Text)
	m.textKind = textKindOf(msg)
	if msg.AtEnd {
		m.textOffset = max(len(m.text)-m.listHeight(), 0) // the last lines on screen
	}
	return m, nil
}

// showText opens the text page with text, scrolled to the top.
func (m Model) showText(text string) Model {
	m.overlay = overlayText
	m.text = strings.Split(strings.TrimRight(text, "\n"), "\n")
	m.textOffset = 0
	return m
}

// fieldRows is the number of rows the chosen action's text field takes in
// the confirm page's cursor: 1 if it has one, else 0. The flags come after.
func (m Model) fieldRows() int {
	if m.chosen().Input != "" {
		return 1
	}
	return 0
}

// typing reports whether the cursor is in the confirm page's text field.
// Keys then type, so q, y, n and the vim keys are letters there.
func (m Model) typing() bool {
	return m.overlay == overlayConfirm && m.fieldRows() == 1 && m.overlayCursor == 0
}

// onFieldKey handles a key while the cursor is in the text field: letters
// (and pasted text) are added, backspace deletes the last one, and enter,
// tab or ↓ go on to the flags.
func (m Model) onFieldKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyRunes, tea.KeySpace:
		m.input += string(msg.Runes)
	case tea.KeyBackspace:
		if r := []rune(m.input); len(r) > 0 {
			m.input = string(r[:len(r)-1])
		}
	case tea.KeyEnter, tea.KeyTab, tea.KeyDown:
		m.overlayCursor = min(1, len(m.flags)) // the first flag, if any
	case tea.KeyEsc:
		m.overlay = overlayActions
		m.overlayCursor = m.chosenAction
	case tea.KeyCtrlC:
		return m, tea.Quit
	}
	return m, nil
}

// onOverlayKey handles a key while a page is open on top of the list.
// Keys the page doesn't use are ignored, except quit.
func (m Model) onOverlayKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.typing() {
		return m.onFieldKey(msg)
	}
	key := msg.String()
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
				if key == a.Key {
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
			m.overlayCursor = min(m.overlayCursor+1, max(m.fieldRows()+len(m.flags)-1, 0))
		case KeySpace:
			if i := m.overlayCursor - m.fieldRows(); i >= 0 && i < len(m.flags) {
				m.flags[i].On = !m.flags[i].On
			}
		case KeyYes:
			if a.Changes {
				return m.runChosen()
			}
		case KeyEnter:
			if !a.Changes {
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

	case overlayNamespaces:
		return m.onNamespacesKey(key)

	case overlayContainers:
		return m.onContainersKey(key)
	}
	return m, nil
}
