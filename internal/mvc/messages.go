package mvc

import "time"

// Messages are what commands send back to Update when their work is done.
// Each one gets its own case in Update. Put every message type here so
// there is one place to see everything Update can receive.

// apiTimeout is how long one API call may take before it gives up.
const apiTimeout = 10 * time.Second

// itemsLoadedMsg is sent to Update when a tab's data has been loaded.
// Every tab uses this same message, so Update needs only one case for all of them.
type itemsLoadedMsg struct {
	tab  string   // which tab the rows are for, e.g. "Nodes"
	rows []string // one line of text per item
	err  error    // set if the API call failed
}
