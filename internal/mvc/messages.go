package mvc

import "time"

// Messages are what commands send back to Update when their work is done.
// Each one gets its own case in Update. Put every message type here so
// there is one place to see everything Update can receive.

// apiTimeout is how long one API call may take before it gives up.
const apiTimeout = 10 * time.Second

// refreshRates are the auto refresh rates the user can pick from with KeyRefreshRate.
var refreshRates = []time.Duration{2 * time.Second, 5 * time.Second, 10 * time.Second}

// defaultRefresh is how often the active tab reloads from the cluster at start.
const defaultRefresh = 5 * time.Second

// itemsLoadedMsg is sent to Update when a tab's data has been loaded.
// Every tab uses this same message, so Update needs only one case for all of them.
type itemsLoadedMsg struct {
	tab  string    // which tab the rows are for, e.g. "Nodes"
	rows []string  // one line of text per item
	err  error     // set if the API call failed
	at   time.Time // when the data was fetched; shown as "updated 12:04:05"
}

// refreshTickMsg is sent every refreshEvery. Update reloads the active tab
// and starts the next tick, so the ticks keep going while auto refresh is on.
//
// gen is the timer's generation. Turning auto refresh on or changing the rate
// starts a new timer with a new gen; ticks from an older timer still arrive
// once, and Update drops them so two timers never run at the same time.
type refreshTickMsg struct {
	gen int
}
