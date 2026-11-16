package commands

import (
	"strconv"
	"strings"
)

// The status column of a tab, colored by how things are: green when fine,
// yellow when on the way there or partly there, red when broken. Which
// column it is and how its values read is given here per tab; the colors
// are drawn by the view (mvc/view.go).

// Level is how a status reads.
type Level int

const (
	LevelNone Level = iota // not a status, or nothing to say: not colored
	LevelOK                // fine: Running, Ready, Completed, 3/3
	LevelWarn              // on the way, or partly: Pending, Terminating, 1/3
	LevelBad               // broken: CrashLoopBackOff, Error, a NotReady node, 0/3
)

// StatusColumn says which column of a tab's rows is its status, and how
// to read it.
type StatusColumn struct {
	// Column is the column's index, counted from 0, in a row without the
	// NAMESPACE column; while it is shown, the status is one further on.
	// The columns before it must have no spaces in them.
	Column int

	// Level reads one value of the column.
	Level func(value string) Level
}

// PodStatus is the Pods tab's STATUS column: NAME READY STATUS.
var PodStatus = StatusColumn{Column: 2, Level: PodStatusLevel}

// NodeStatus is the Nodes tab's STATUS column: NAME STATUS.
var NodeStatus = StatusColumn{Column: 1, Level: NodeStatusLevel}

// DeploymentReady is the Deployments tab's READY column: NAME READY.
var DeploymentReady = StatusColumn{Column: 1, Level: ReadyLevel}

// PodStatusLevel reads a pod's STATUS, as kubectl writes it.
func PodStatusLevel(status string) Level {
	switch status {
	case "Running", "Completed", "Succeeded":
		return LevelOK
	case "Pending", "ContainerCreating", "PodInitializing", "Terminating", "NotReady", "Unknown":
		return LevelWarn
	case "":
		return LevelNone
	}
	if rest, ok := strings.CutPrefix(status, "Init:"); ok {
		if strings.Contains(rest, "/") {
			return LevelWarn // Init:1/2: the init containers are running
		}
		return LevelBad // Init:Error, Init:CrashLoopBackOff, …
	}
	// The rest are why a container isn't running: CrashLoopBackOff,
	// ImagePullBackOff, ErrImagePull, Error, OOMKilled, Evicted,
	// CreateContainerConfigError, ExitCode:1, Signal:9, …
	return LevelBad
}

// NodeStatusLevel reads a node's STATUS: Ready is fine, but a cordoned
// node (Ready,SchedulingDisabled) takes no new pods.
func NodeStatusLevel(status string) Level {
	switch status {
	case "Ready":
		return LevelOK
	case "Ready,SchedulingDisabled", "Unknown":
		return LevelWarn
	case "NotReady", "NotReady,SchedulingDisabled":
		return LevelBad
	}
	return LevelNone
}

// ReadyLevel reads a READY count, e.g. "2/3": all ready is fine, some is
// partly there, none of several is broken. Scaled to 0/0 there is nothing
// to say.
func ReadyLevel(ready string) Level {
	have, want, ok := strings.Cut(ready, "/")
	h, err1 := strconv.Atoi(have)
	w, err2 := strconv.Atoi(want)
	switch {
	case !ok || err1 != nil || err2 != nil || w == 0:
		return LevelNone
	case h >= w:
		return LevelOK
	case h == 0:
		return LevelBad
	}
	return LevelWarn
}

// FieldSpan is where the n-th (from 0) field of row is, the fields being
// separated by spaces: the bytes [start, end). ok is false if row has
// fewer fields.
func FieldSpan(row string, n int) (start, end int, ok bool) {
	i := 0
	for field := 0; ; field++ {
		for i < len(row) && row[i] == ' ' {
			i++
		}
		if i == len(row) {
			return 0, 0, false
		}
		start = i
		for i < len(row) && row[i] != ' ' {
			i++
		}
		if field == n {
			return start, i, true
		}
	}
}
