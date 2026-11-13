package mvc

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"slices"
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
//	                                         (type, tick flags)
//	                                                               │
//	                         a change: back to the list, reloaded ◀┤
//	                         a dry run: what it would change      ◀┤
//	                         a read: a page of text (Describe)    ◀┤
//	                         a shell: kubectl has the terminal,
//	                                  back to the list when it ends ◀┘
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

	// off is written in the command instead while the flag isn't ticked,
	// for a choice of two, e.g. "--image=busybox" for "--image=ubuntu".
	// "" for nothing.
	off string
}

// String is how the flag is written in the command now: its name if it is
// ticked, else its off form.
func (f flag) String() string {
	if f.on {
		return f.name
	}
	return f.off
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

	// input names the text field of an action that needs something typed,
	// e.g. "labels" for Label, where "env=prod" is typed. "" for none.
	input string

	// command is the kubectl command, with %s where the row's name goes,
	// e.g. "kubectl cordon %s". What was typed and the ticked flags are
	// added after it.
	command string

	// run does the action to target with what was typed and the given
	// flags. It sends an actionDoneMsg when it is done.
	run func(c *k8s.Client, target, input string, flags []flag) tea.Cmd
}

// commandFor is the kubectl command for running a on target with input
// and flags, e.g. "kubectl label node worker1 env=prod --overwrite".
func (a action) commandFor(target, input string, flags []flag) string {
	parts := []string{fmt.Sprintf(a.command, target)}
	if input = strings.TrimSpace(input); input != "" {
		parts = append(parts, input)
	}
	for _, f := range flags {
		if s := f.String(); s != "" {
			parts = append(parts, s)
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
// runs do and reports whether it worked. do may also return a text of what
// changed (see changeText); after a dry run it is shown on a page.
func change(name, target string, flags []flag, do func(context.Context) (string, error)) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), apiTimeout)
		defer cancel()
		text, err := do(ctx)
		return actionDoneMsg{action: name, target: target, changed: true, dryRun: ticked(flags, flagDryRun), text: text, err: err}
	}
}

// inTerminal returns the command for an action that needs the terminal to
// itself, such as a shell: the app steps aside while cmd runs and comes
// back when it ends. err is from making cmd, e.g. kubectl isn't installed.
func inTerminal(name, target string, cmd *exec.Cmd, err error) tea.Cmd {
	if err != nil {
		return func() tea.Msg { return actionDoneMsg{action: name, target: target, shell: true, err: err} }
	}
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		// A shell ends with the code of the last thing typed in it, which
		// isn't a failure of the shell: only failing to start is.
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			err = nil
		}
		return actionDoneMsg{action: name, target: target, shell: true, err: err}
	})
}

// changeText shows what a change does to a list, e.g. a node's labels:
// every item after it, "+" for the new ones, then "-" for the removed ones.
//
//	Labels after the change:
//
//	+ env=prod
//	  kubernetes.io/os=linux
//	- old=x
func changeText(title string, before, after []string) string {
	lines := []string{title + " after the change:", ""}
	for _, s := range after {
		mark := "  "
		if !slices.Contains(before, s) {
			mark = "+ "
		}
		lines = append(lines, mark+s)
	}
	for _, s := range before {
		if !slices.Contains(after, s) {
			lines = append(lines, "- "+s)
		}
	}
	if len(before) == 0 && len(after) == 0 {
		lines = append(lines, "(none)")
	}
	return strings.Join(lines, "\n")
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
	return len(m.actions()) > 0 && loaded && m.err == nil && len(m.rows()) > 0
}

// openActions opens the action list for the row the cursor is on.
func (m Model) openActions() Model {
	m.target = rowName(m.rows()[m.cursor])
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
	m.input = ""
	if !a.changes && len(a.flags) == 0 && a.input == "" {
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
	if a.input != "" && strings.TrimSpace(m.input) == "" {
		m.notice = "type the " + a.input + " first"
		m.overlayCursor = 0 // on the text field
		return m, nil
	}
	m.notice = "running: " + a.commandFor(m.target, m.input, m.flags)
	if a.changes {
		m.overlay = overlayNone // back to the list while it runs
	}
	if m.client == nil {
		return m, nil // tests without a cluster
	}
	return m, a.run(m.client, m.target, m.input, m.flags)
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
	case msg.shell:
		// Like kubectl, the debug pod is left behind.
		m.notice = msg.action + " on " + msg.target + " closed; its debug pod is still there, delete it when you are done"
		return m, m.loadActive()
	case msg.changed && msg.dryRun:
		m.notice = what + ": dry run passed, nothing changed"
		if msg.text != "" {
			m = m.showText(msg.text) // what it would change
		}
		return m, nil // nothing changed, so nothing to reload
	case msg.changed:
		m.notice = what + ": done"
		return m, m.loadActive()
	}
	m.notice = ""
	return m.showText(msg.text), nil
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
	if m.chosen().input != "" {
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
			m.overlayCursor = min(m.overlayCursor+1, max(m.fieldRows()+len(m.flags)-1, 0))
		case KeySpace:
			if i := m.overlayCursor - m.fieldRows(); i >= 0 && i < len(m.flags) {
				m.flags[i].on = !m.flags[i].on
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
