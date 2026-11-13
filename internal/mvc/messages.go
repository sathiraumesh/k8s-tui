package mvc

import "time"

// Messages are what commands send back to Update when their work is done.
// Each one gets its own case in Update. Put every message type here so
// there is one place to see everything Update can receive.

// apiTimeout is how long one API call may take before it gives up.
const apiTimeout = 10 * time.Second

// Auto refresh rates: how often the active tab reloads from the cluster.
const (
	refreshOff    time.Duration = 0 // auto refresh is off
	refreshFast                 = 2 * time.Second
	refreshNormal               = 5 * time.Second
	refreshSlow                 = 10 * time.Second

	// defaultRefresh is the rate the app starts with.
	defaultRefresh = refreshNormal
)

// refreshRates are the choices the refresh button steps through, in order.
var refreshRates = []time.Duration{refreshOff, refreshFast, refreshNormal, refreshSlow}

// itemsLoadedMsg is sent to Update when a tab's data has been loaded.
// Every tab uses this same message, so Update needs only one case for all of them.
type itemsLoadedMsg struct {
	tab  string    // which tab the rows are for, e.g. "Nodes"
	rows []string  // one line of text per item
	err  error     // set if the API call failed
	at   time.Time // when the data was fetched; shown as "updated 12:04:05"

	// wide is whether the rows have the wide columns. If the tab's wide
	// setting changed while the load was on its way, Update drops the rows:
	// they have the wrong columns, and a load with the right ones follows.
	wide bool
}

// refreshTickMsg is sent every refreshEvery. Update reloads the active tab
// and starts the next tick, so the ticks keep going while auto refresh is on.
//
// gen is the timer's generation. Changing the rate starts a new timer with a
// new gen; ticks from an older timer still arrive
// once, and Update drops them so two timers never run at the same time.
type refreshTickMsg struct {
	gen int
}

// actionDoneMsg is sent when an action (actions.go) has run.
type actionDoneMsg struct {
	action string // its name, e.g. "Cordon"
	target string // the row it was run on, e.g. "worker1"
	err    error  // set if it failed

	text    string // what a read action returned (the Describe text), or what a dry run would change
	changed bool   // it was a change: the tab is reloaded to show it
	dryRun  bool   // it was a change run with --dry-run=server: nothing was saved
	// terminal is whether it ran kubectl in the terminal (a shell, an
	// editor) that has now closed, and closed what to say then.
	terminal bool
	closed   string
}
