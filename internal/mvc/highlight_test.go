package mvc

import (
	"errors"
	"strings"
	"testing"

	"k8s-tui/internal/mvc/commands"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// colorOf is the color the text in line is drawn in: "ok", "warn", "bad",
// "head", or "" if it isn't colored. line is what a highlighter returned.
func colorOf(t *testing.T, line, text string) string {
	t.Helper()
	i := strings.Index(line, text)
	if i < 0 {
		t.Fatalf("%q is not in %q", text, line)
	}
	open := strings.LastIndex(line[:i], "\x1b[")
	if open < 0 || strings.HasPrefix(line[open:], "\x1b[0m") {
		return ""
	}
	code := line[open:]
	for name, c := range map[string]lipgloss.Color{"ok": statusColors[commands.LevelOK], "warn": statusColors[commands.LevelWarn], "bad": statusColors[commands.LevelBad]} {
		if strings.HasPrefix(code[2:], rgb(c)) || strings.Contains(code[:strings.Index(code, "m")], rgb(c)) {
			return name
		}
	}
	return "head"
}

func TestDescribeColors(t *testing.T) {
	withColors(t)
	for _, tc := range []struct{ line, text, want string }{
		{"    State:          Waiting", "Waiting", "warn"},
		{"      Reason:       CrashLoopBackOff", "CrashLoopBackOff", "bad"},
		{"    State:          Running", "Running", "ok"},
		{"      Reason:       Error", "Error", "bad"},
		{"  Warning  BackOff    2m (x10 over 5m)  kubelet  Back-off restarting failed container", "kubelet", "warn"},
		{"  Normal   Pulled     3m                kubelet  Successfully pulled image", "Pulled", ""},
		{"  Progressing    True    NewReplicaSetAvailable", "NewReplicaSetAvailable", "ok"},
		{"  Available      False   MinimumReplicasUnavailable", "MinimumReplicasUnavailable", "bad"},
		{"Name:         web", "web", ""},
	} {
		if got := colorOf(t, describeLine(tc.line), tc.text); got != tc.want {
			t.Errorf("%q: %q is %q, want %q", tc.line, tc.text, got, tc.want)
		}
		if ansi.Strip(describeLine(tc.line)) != tc.line {
			t.Errorf("%q: the text changed: %q", tc.line, ansi.Strip(describeLine(tc.line)))
		}
	}
}

// A line with two words of different colors gets both.
func TestDescribeTwoWords(t *testing.T) {
	withColors(t)
	line := describeLine("  Last State: Terminated  Reason: OOMKilled")
	if colorOf(t, line, "Terminated") != "warn" || colorOf(t, line, "OOMKilled") != "bad" {
		t.Errorf("line = %q", line)
	}
}

func TestLogColors(t *testing.T) {
	withColors(t)
	for _, tc := range []struct{ line, want string }{
		{"[ERROR] plugin/kubernetes: failed to list", "bad"},
		{`time="..." level=error msg="boom"`, "bad"},
		{"E1116 12:00:00.000000       1 reflector.go:1] watch failed", "bad"},
		{"panic: runtime error: index out of range", "bad"},
		{"[WARNING] no upstream", "warn"},
		{"W1116 12:00:00.000000       1 warnings.go:70] deprecated", "warn"},
		{"==> web-1/app <==", "head"},
		{"GET /healthz 200", ""},
		{"Terror of the deep", ""}, // "error" inside a word is not a level
	} {
		if got := colorOf(t, logLine(tc.line), tc.line[:3]); got != tc.want {
			t.Errorf("%q: %q, want %q", tc.line, got, tc.want)
		}
	}
}

func TestChangeColors(t *testing.T) {
	withColors(t)
	if colorOf(t, changeLine("+ env=prod"), "env") != "ok" || colorOf(t, changeLine("- old=x"), "old") != "bad" || colorOf(t, changeLine("  kept=y"), "kept") != "" {
		t.Error("+ green, - red, the rest plain")
	}
}

func TestNoticeColors(t *testing.T) {
	for notice, want := range map[string]lipgloss.Style{
		"Cordon worker2 failed: forbidden":                badText,
		"Cordon worker2: done":                            okText,
		"Cordon worker2: dry run passed, nothing changed": okText,
		"running: kubectl cordon worker2":                 Help,
	} {
		if got := noticeStyle(notice); got.GetForeground() != want.GetForeground() {
			t.Errorf("%q: %v, want %v", notice, got.GetForeground(), want.GetForeground())
		}
	}
}

// The page a Describe opens is colored as a describe text, Logs as logs,
// a dry run as a change, Show not at all.
func TestTextKinds(t *testing.T) {
	for msg, want := range map[*commands.ActionDoneMsg]textKind{
		{Action: "Describe"}:                           textDescribe,
		{Action: "Logs", AtEnd: true}:                  textLogs,
		{Action: "Label", Changed: true, DryRun: true}: textChange,
		{Action: "Show"}:                               textPlain,
	} {
		if got := textKindOf(*msg); got != want {
			t.Errorf("%+v: kind %d, want %d", *msg, got, want)
		}
	}
}

// On screen: the Describe page shows its reason red.
func TestDescribePageColored(t *testing.T) {
	withColors(t)
	m, _ := nodesOnScreen(t)
	m = press(t, m, enterKey)
	m = press(t, m, commands.ActionDoneMsg{Action: "Describe", Target: "worker2", Text: "Name: worker2\n  Reason: CrashLoopBackOff"})
	if line := lineWith(t, m, "Reason:"); !strings.Contains(line, rgb(statusColors[commands.LevelBad])) {
		t.Errorf("reason not red: %q", line)
	}
}

// The line under the panel is red after a failure.
func TestFailedNoticeRed(t *testing.T) {
	withColors(t)
	m, _ := nodesOnScreen(t)
	m = press(t, m, commands.ActionDoneMsg{Action: "Cordon", Target: "worker2", Changed: true, Err: errors.New("forbidden")})
	if line := lineWith(t, m, "failed:"); !strings.Contains(line, rgb(statusColors[commands.LevelBad])) {
		t.Errorf("notice not red: %q", line)
	}
}
