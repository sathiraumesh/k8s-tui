package mvc

import (
	"strings"
	"time"

	k8s "k8s-tui/internal/kubernetes"

	tea "github.com/charmbracelet/bubbletea"
)

// Every tab is described once, in allTabs: its name, how its rows are
// loaded, and whether the refresh timer reloads it. The rest of the app
// works from this table, so adding a tab, or giving a tab real data from
// the cluster, is a change to one line here.

// Names of the tabs. Rows, update times and wide settings are kept per tab
// under these names.
const (
	tabNodes       = "Nodes"
	tabPods        = "Pods"
	tabDeployments = "Deployments"
	tabServices    = "Services"
	tabMore        = "More" // every resource type the cluster serves, like `kubectl api-resources`
)

// loader returns the command that loads a tab's rows. It sends an
// itemsLoadedMsg for the tab, with the rows (with or without the wide
// columns) and when they were loaded.
type loader func(c *k8s.Client, wide bool) tea.Cmd

// tab describes one tab.
type tab struct {
	name string
	load loader

	// command is the kubectl command that lists what the tab shows, e.g.
	// "kubectl get nodes". It is shown on the panel's bottom border, with
	// " -o wide" added while the tab shows its wide columns.
	command string

	// actions are what can be done to a row of the tab: enter on a row
	// opens them (actions.go). nil if the tab has none.
	actions []action

	// own are actions of the tab itself, not of a row, e.g. running a new
	// pod: the tab's button (ownButton, e.g. "+ run") or KeyOwn opens them.
	// ownTitle heads their list ("Run a pod"), and ownTarget is what they
	// are for (the namespace, "default").
	own       []action
	ownButton string
	ownTitle  string
	ownTarget string

	// autoRefresh is whether the refresh timer reloads the tab. A tab that
	// isn't auto refreshed is loaded the first time it is opened, and again
	// when its wide setting changes; it has no refresh button.
	autoRefresh bool
}

// allTabs returns the tabs, left to right.
func allTabs() []tab {
	return []tab{
		{
			name:        tabNodes,
			load:        loadNodes,
			command:     "kubectl get nodes",
			actions:     nodeActions(),
			autoRefresh: true,
		},
		{
			name:        tabPods,
			load:        loadPods,
			command:     "kubectl get pods --all-namespaces",
			own:         runPodActions(),
			ownButton:   "+ run",
			ownTitle:    "Run a pod",
			ownTarget:   runNamespace,
			autoRefresh: true,
		},
		{
			name:        tabDeployments,
			load:        placeholder(tabDeployments, "nginx", "api"),
			command:     "kubectl get deployments --all-namespaces",
			autoRefresh: true,
		},
		{
			name:        tabServices,
			load:        placeholder(tabServices, "kubernetes", "nginx-svc", "redis"),
			command:     "kubectl get services --all-namespaces",
			autoRefresh: true,
		},
		{
			name:        tabMore,
			load:        loadResources,
			command:     "kubectl api-resources",
			autoRefresh: false,
		},
	}
}

// kubectlCommand is the kubectl command behind what the active tab shows,
// e.g. "kubectl get nodes -o wide" while Nodes shows its wide columns.
//
// While a page is open it is the action's command instead: the selected
// one in the action list, the chosen one with its ticked flags after that.
func (m Model) kubectlCommand() string {
	switch m.overlay {
	case overlayActions:
		a := m.actions()[m.overlayCursor]
		return a.commandFor(m.target, a.inputDefault, a.flags)
	case overlayConfirm, overlayText:
		return m.chosen().commandFor(m.target, m.input, m.flags)
	}
	cmd := m.activeTab().command
	if cmd != "" && m.activeWide() {
		cmd += " -o wide"
	}
	if s := m.searchText(); cmd != "" && s != "" {
		cmd += " | grep -i " + shellQuote(s) // what the search does, in the shell
	}
	return cmd
}

// shellQuote writes s so the shell takes it as one word: as it is if it
// is plain, else in single quotes.
func shellQuote(s string) string {
	if strings.Trim(s, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-_./:=") == "" {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// placeholder is the loader for a tab that isn't loaded from the cluster
// yet: it sends the same fixed rows each time. Because it goes through the
// same message as a real loader, the tab behaves like every other one: it
// gets an update time, auto refresh and the wide setting. To load the tab
// from the cluster, replace placeholder(...) with a real loader in allTabs.
func placeholder(name string, rows ...string) loader {
	return func(_ *k8s.Client, wide bool) tea.Cmd {
		return func() tea.Msg {
			return itemsLoadedMsg{tab: name, rows: rows, at: time.Now(), wide: wide}
		}
	}
}

// activeTab is the tab that is showing.
func (m Model) activeTab() tab {
	return m.tabs[m.active]
}

// activeName is the name of the tab that is showing.
func (m Model) activeName() string {
	return m.activeTab().name
}
