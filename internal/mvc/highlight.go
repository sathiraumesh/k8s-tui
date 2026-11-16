package mvc

import (
	"regexp"
	"strings"

	"k8s-tui/internal/mvc/commands"
	"k8s-tui/internal/styles"

	"github.com/charmbracelet/lipgloss"
)

// Colors on the pages of text and the line under the panel, the same as
// for the status columns (status.go): red for what is broken, yellow for
// warnings and what is on its way, green for what is fine.
//
//	Describe   the events of type Warning, yellow; states and reasons by
//	           what they say: Running green, Pending yellow, CrashLoopBackOff red
//	Logs       a line by its level: error / fatal / panic red, warn yellow;
//	           the heading of each pod and container in the accent color
//	a change   the lines it adds (+) green, removes (-) red: a dry run, Label
//	Show       not colored: labels and annotations say nothing good or bad
//
// The line under the panel is red when an action failed, green when it
// worked, and the list's "error: …" row red.

// textKind is what a page of text shows, so its lines are colored the
// right way.
type textKind int

const (
	textPlain    textKind = iota // e.g. Show: not colored
	textDescribe                 // kubectl describe
	textLogs                     // kubectl logs
	textChange                   // what a change does, + and - lines
)

// The styles of the colors.
var (
	okText   = lipgloss.NewStyle().Foreground(styles.StatusOK)
	warnText = lipgloss.NewStyle().Foreground(styles.StatusWarn)
	badText  = lipgloss.NewStyle().Foreground(styles.StatusBad)
	headText = lipgloss.NewStyle().Foreground(styles.Accent).Bold(true)
)

// The words of a describe text that say how something is. They are
// matched whole, with their case: "Failed" but not "failed" in a sentence.
var (
	describeBad  = regexp.MustCompile(`\b(CrashLoopBackOff|ImagePullBackOff|ErrImagePull|InvalidImageName|CreateContainerConfigError|CreateContainerError|RunContainerError|OOMKilled|Error|Failed[A-Za-z]*|BackOff|Unhealthy|Evicted|NotReady|ContainersNotReady|ProgressDeadlineExceeded|ReplicaFailure|MinimumReplicasUnavailable)\b`)
	describeWarn = regexp.MustCompile(`\b(Warning|Pending|Waiting|ContainerCreating|PodInitializing|Terminating|Terminated|SchedulingDisabled|Unknown|Unschedulable)\b`)
	describeOK   = regexp.MustCompile(`\b(Running|Completed|Succeeded|MinimumReplicasAvailable|NewReplicaSetAvailable)\b`)
)

// The levels of a log line, in any case: "ERROR", "level=error",
// "[warn]", and klog's "E1116 12:00:00" and "W1116 …".
var (
	logBad  = regexp.MustCompile(`(?i)\b(error|err|fatal|panic|critical|crit|exception|failed|failure)\b|^[EF]\d{4} `)
	logWarn = regexp.MustCompile(`(?i)\b(warn|warning)\b|^W\d{4} `)
)

// highlighter returns what colors the lines of a page of kind, or nil
// for one that isn't colored.
func highlighter(kind textKind) func(line string) string {
	switch kind {
	case textDescribe:
		return describeLine
	case textLogs:
		return logLine
	case textChange:
		return changeLine
	}
	return nil
}

// describeLine colors a line of a describe text. An event of type Warning
// is yellow as a whole:
//
//	Warning  BackOff  2m (x10 over 5m)  kubelet  Back-off restarting failed container
//
// elsewhere only the words that say how something is.
func describeLine(line string) string {
	if f := strings.Fields(line); len(f) > 1 && f[0] == "Warning" {
		return warnText.Render(line)
	}
	line = colorWords(line, describeBad, badText)
	line = colorWords(line, describeWarn, warnText)
	return colorWords(line, describeOK, okText)
}

// colorWords colors every match of re in line with style. A word that is
// already colored (inside an escape sequence's text) is left as it is.
func colorWords(line string, re *regexp.Regexp, style lipgloss.Style) string {
	var b strings.Builder
	last := 0
	for _, m := range re.FindAllStringIndex(line, -1) {
		if colored(line, m[0]) {
			continue
		}
		b.WriteString(line[last:m[0]])
		b.WriteString(style.Render(line[m[0]:m[1]]))
		last = m[1]
	}
	b.WriteString(line[last:])
	return b.String()
}

// colored reports whether byte i of line is inside text an earlier
// colorWords colored: after a color's escape code, before its reset.
func colored(line string, i int) bool {
	open := strings.LastIndex(line[:i], "\x1b[")
	return open >= 0 && !strings.HasPrefix(line[open:], "\x1b[0m")
}

// logLine colors a line of logs by its level, and the headings DeploymentLogs
// and PodLogs put above each container's logs.
func logLine(line string) string {
	switch {
	case strings.HasPrefix(line, "==> ") && strings.HasSuffix(line, " <=="):
		return headText.Render(line)
	case logBad.MatchString(line):
		return badText.Render(line)
	case logWarn.MatchString(line):
		return warnText.Render(line)
	}
	return line
}

// changeLine colors a line of what a change does (changeText): what it
// adds green, what it removes red.
func changeLine(line string) string {
	switch {
	case strings.HasPrefix(line, "+ "):
		return okText.Render(line)
	case strings.HasPrefix(line, "- "):
		return badText.Render(line)
	}
	return line
}

// noticeStyle is the style of the line under the panel for notice: red
// if an action failed, green if it worked, else the help line's.
func noticeStyle(notice string) lipgloss.Style {
	switch {
	case strings.Contains(notice, " failed: "):
		return badText
	case strings.HasSuffix(notice, ": done"), strings.Contains(notice, ": dry run passed"):
		return okText
	}
	return Help
}

// textKindOf is the kind of the page msg opens.
func textKindOf(msg commands.ActionDoneMsg) textKind {
	switch {
	case msg.Changed:
		return textChange // a dry run: what it would change
	case msg.AtEnd:
		return textLogs
	case msg.Action == "Describe":
		return textDescribe
	}
	return textPlain
}
