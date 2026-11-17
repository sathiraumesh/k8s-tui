package commands

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"slices"
	"strings"
	"time"

	k8s "k8s-tui/internal/kubernetes"

	tea "github.com/charmbracelet/bubbletea"
)

// Package commands has what each tab can load and do: its rows from the
// cluster (LoadNodes, LoadPods, …) and the actions on them (NodeActions,
// PodActions, …). A command is slow work, an API call: Bubble Tea runs it
// in the background so the screen never freezes, then sends the message it
// returns (ItemsLoadedMsg or ActionDoneMsg) to the model's Update.
//
// The pages that show actions, and the keys that pick and run them, are in
// package mvc.

// APITimeout is how long one API call may take before it gives up.
const APITimeout = 10 * time.Second

// Names of the tabs. Rows, update times and wide settings are kept per tab
// under these names.
const (
	TabNodes       = "Nodes"
	TabPods        = "Pods"
	TabDeployments = "Deployments"
	TabServices    = "Services"
	TabMore        = "More" // every resource type the cluster serves, like `kubectl api-resources`
)

// AllNamespaces is the namespace setting for every namespace, like
// `kubectl get pods --all-namespaces`.
const AllNamespaces = ""

// Flag is one kubectl option of an action, shown as a tick box. It is
// written as kubectl takes it, e.g. "--dry-run=server", and added to the
// command when ticked.
type Flag struct {
	Name string
	On   bool // ticked; the action's list gives the default

	// Off is written in the command instead while the flag isn't ticked,
	// for a choice of two, e.g. "--image=busybox" for "--image=ubuntu".
	// "" for nothing.
	Off string
}

// String is how the flag is written in the command now: its name if it is
// ticked, else its off form.
func (f Flag) String() string {
	if f.On {
		return f.Name
	}
	return f.Off
}

// Action is something that can be done to one row.
type Action struct {
	Key   string // picks it in the action list, e.g. "c"
	Name  string // e.g. "Cordon"
	About string // one line: what it does

	// Changes is whether it Changes the cluster. Such an action always asks
	// first, and only y runs it. One that only reads runs with enter.
	Changes bool

	Flags []Flag

	// Input names the text field of an action that needs something typed,
	// e.g. "labels" for Label, where "env=prod" is typed. "" for none.
	// inputDefault is what is in the field to begin with, to be changed.
	Input        string
	InputDefault string

	// Containers, if set, lists the containers the action can be for, e.g.
	// a pod's, and makes it ask which: with more than one, a picker opens
	// first (mvc/containers.go); the choice reaches Run as its input.
	//
	// An action that also has a text field (Input) starts it with the
	// choice: "app=" for app, "*=" for all of them, e.g. for Set image.
	//
	// OneContainer is for an action that can't be for all of them, such as
	// a shell: the picker then offers only the containers that are running
	// (Containers is called with running), and with one container there is
	// no input.
	Containers   ContainerLister
	OneContainer bool

	// InputArgs is how what was typed is written in the command, e.g.
	// "db postgres" as "db --image=postgres". nil writes it as typed.
	InputArgs func(input string) string

	// Command is the kubectl Command, with %s where the row's name goes,
	// e.g. "kubectl cordon %s". What was typed goes where {input} is, or
	// after the Command if there is no {input}; the ticked flags go last.
	Command string

	// Run does the action to target with what was typed and the given
	// flags. It sends an ActionDoneMsg when it is done.
	Run func(c *k8s.Client, target, input string, flags []Flag) tea.Cmd
}

// CommandFor is the kubectl command for running a on target with input
// and flags, e.g. "kubectl label node worker1 env=prod --overwrite".
func (a Action) CommandFor(target, input string, flags []Flag) string {
	if input = strings.TrimSpace(input); input != "" && a.InputArgs != nil {
		input = a.InputArgs(input)
	}
	cmd := a.Command
	if strings.Contains(cmd, "%s") { // some commands have no target: kubectl create namespace NAME
		cmd = fmt.Sprintf(cmd, target)
	}
	if strings.Contains(cmd, InputHere) {
		cmd, input = strings.ReplaceAll(cmd, InputHere, input), ""
	}
	parts := strings.Fields(cmd) // no double space where {input} was empty
	if input != "" {
		parts = append(parts, input)
	}
	for _, f := range flags {
		if s := f.String(); s != "" {
			parts = append(parts, s)
		}
	}
	return strings.Join(parts, " ")
}

// InputHere marks where in an action's command what was typed goes.
const InputHere = "{input}"

// Ticked reports whether the flag called name is Ticked.
func Ticked(flags []Flag, name string) bool {
	for _, f := range flags {
		if f.Name == name && f.On {
			return true
		}
	}
	return false
}

// FlagDryRun is the flag of a change that lets the cluster check it without
// saving it. change reports a run with it ticked as a dry run.
const FlagDryRun = "--dry-run=server"

// Read returns the command for an action that only reads: it runs get and
// sends its text, to be shown on a page.
func Read(name, target string, get func(context.Context) (string, error)) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), APITimeout)
		defer cancel()
		text, err := get(ctx)
		return ActionDoneMsg{Action: name, Target: target, Text: text, Err: err}
	}
}

// ReadToEnd is read for a text whose end matters most, such as logs: the
// page opens scrolled to its last line.
func ReadToEnd(name, target string, get func(context.Context) (string, error)) tea.Cmd {
	cmd := Read(name, target, get)
	return func() tea.Msg {
		msg := cmd().(ActionDoneMsg)
		msg.AtEnd = true
		return msg
	}
}

// Change returns the command for an action that changes the cluster: it
// runs do and reports whether it worked. do may also return a text of what
// changed (see changeText); after a dry run it is shown on a page.
func Change(name, target string, flags []Flag, do func(context.Context) (string, error)) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), APITimeout)
		defer cancel()
		text, err := do(ctx)
		return ActionDoneMsg{Action: name, Target: target, Changed: true, DryRun: Ticked(flags, FlagDryRun), Text: text, Err: err}
	}
}

// InTerminal returns the command for an action that needs the terminal to
// itself, such as a shell or an editor: the app steps aside while cmd runs
// and comes back when it ends. err is from making cmd, e.g. kubectl isn't
// installed. closed is what the line under the panel says afterwards.
//
// shell says cmd is a shell: it ends with the code of the last thing typed
// in it, which isn't a failure of the shell, so only failing to start is.
// Anything else that ends with an error code has failed.
func InTerminal(name, target, closed string, shell bool, cmd *exec.Cmd, err error) tea.Cmd {
	done := func(err error) tea.Msg {
		return ActionDoneMsg{Action: name, Target: target, Terminal: true, Closed: closed, Err: err}
	}
	if err != nil {
		return func() tea.Msg { return done(err) }
	}
	return tea.ExecProcess(cmd, func(err error) tea.Msg { return done(TerminalError(shell, err)) })
}

// TerminalError is how a command run in the terminal ended, as inTerminal
// reports it: nil for a shell's exit code, an error for anything else's.
func TerminalError(shell bool, err error) error {
	var exit *exec.ExitError
	switch {
	case !errors.As(err, &exit):
		return err // nil, or it couldn't start
	case shell:
		return nil
	}
	return fmt.Errorf("kubectl ended with an error (%w)", err)
}

// Section is one titled list of a page of text, e.g. a node's labels.
type Section struct {
	Title string
	Items []string
}

// SectionsText is a page of titled lists, e.g. the Show page of a node:
//
//	Labels:
//
//	  env=prod
//	  kubernetes.io/os=linux
//
//	Taints:
//
//	  (none)
func SectionsText(sections ...Section) string {
	var lines []string
	for _, s := range sections {
		lines = append(lines, s.Title+":", "")
		if len(s.Items) == 0 {
			lines = append(lines, "  (none)")
		}
		for _, item := range s.Items {
			lines = append(lines, "  "+item)
		}
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

// ChangeText shows what a change does to a list, e.g. a node's labels:
// every item after it, "+" for the new ones, then "-" for the removed ones.
//
//	Labels after the change:
//
//	+ env=prod
//	  kubernetes.io/os=linux
//	- old=x
func ChangeText(title string, before, after []string) string {
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

// ItemsLoadedMsg is sent to Update when a tab's data has been loaded.
// Every tab uses this same message, so Update needs only one case for all of them.
type ItemsLoadedMsg struct {
	Namespace string // the namespace the rows are from, "" for all (namespaced tabs only)

	Tab  string    // which tab the rows are for, e.g. "Nodes"
	Rows []string  // one line of text per item
	Err  error     // set if the API call failed
	At   time.Time // when the data was fetched; shown as "updated 12:04:05"

	// Wide is whether the rows have the Wide columns. If the tab's Wide
	// setting changed while the load was on its way, Update drops the rows:
	// they have the wrong columns, and a load with the right ones follows.
	Wide bool
}

// ActionDoneMsg is sent when an action has run.
type ActionDoneMsg struct {
	Action string // its name, e.g. "Cordon"
	Target string // the row it was run on, e.g. "worker1"
	Err    error  // set if it failed

	Text    string // what a read action returned (the Describe text), or what a dry run would change
	AtEnd   bool   // the text page opens scrolled to its end (logs)
	Changed bool   // it was a change: the tab is reloaded to show it
	DryRun  bool   // it was a change run with --dry-run=server: nothing was saved
	// Terminal is whether it ran kubectl in the Terminal (a shell, an
	// editor) that has now closed, and closed what to say then.
	Terminal bool
	Closed   string
}

// ContainerLister lists the containers of the thing NAMESPACE/NAME, e.g.
// (*k8s.Client).PodContainers. With running, only those a shell can be
// opened in.
type ContainerLister func(c *k8s.Client, ctx context.Context, namespace, name string, running bool) ([]string, error)

// RowName is the name of the thing a row shows: its first column.
func RowName(row string) string {
	if fields := strings.Fields(row); len(fields) > 0 {
		return fields[0]
	}
	return ""
}
