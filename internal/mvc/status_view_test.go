package mvc

import (
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	"k8s-tui/internal/mvc/commands"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"
)

// withColors draws in true color for the test, as on a real terminal.
func withColors(t *testing.T) {
	t.Helper()
	old := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(old) })
}

// rgb is the part of an escape code that sets the text to hex, e.g.
// "38;2;115;201;145" for "#73C991".
func rgb(hex lipgloss.Color) string {
	var r, g, b int
	fmt.Sscanf(string(hex), "#%02x%02x%02x", &r, &g, &b)
	return fmt.Sprintf("38;2;%d;%d;%d", r, g, b)
}

// lineWith is the line of the screen that has text, with its colors.
func lineWith(t *testing.T, m Model, text string) string {
	t.Helper()
	for _, line := range strings.Split(m.View(), "\n") {
		if strings.Contains(ansi.Strip(line), text) {
			return line
		}
	}
	t.Fatalf("%q is not on the screen", text)
	return ""
}

func TestPodStatusColored(t *testing.T) {
	withColors(t)
	m, _ := podsOnScreen(t)
	m = press(t, m, commands.ItemsLoadedMsg{Tab: tabPods, Rows: commandsRows(
		"default   web   1/1   Running            0   3h",
		"default   db    0/1   CrashLoopBackOff   4   3h",
		"default   job   0/1   Pending            0   3h",
	), At: time.Now()})
	for name, color := range map[string]lipgloss.Color{"web": statusColors[commands.LevelOK], "db": statusColors[commands.LevelBad], "job": statusColors[commands.LevelWarn]} {
		if line := lineWith(t, m, name+" "); !strings.Contains(line, rgb(color)) {
			t.Errorf("%s: status not in %s: %q", name, color, line)
		}
	}
}

// In one namespace the NAMESPACE column is gone: the status is one column earlier.
func TestPodStatusColoredInNamespace(t *testing.T) {
	withColors(t)
	m, _ := podsOnScreen(t)
	m.namespace = "default"
	m = press(t, m, commands.ItemsLoadedMsg{Tab: tabPods, Namespace: "default", Rows: commandsRows("web   1/1   Running   0   3h"), At: time.Now()})
	if line := lineWith(t, m, "web "); !regexp.MustCompile(rgb(statusColors[commands.LevelOK]) + `[;0-9]*mRunning`).MatchString(line) {
		t.Errorf("Running not green: %q", line)
	}
}

// The selected row keeps its full width and background with a colored status.
func TestSelectedRowWithStatus(t *testing.T) {
	withColors(t)
	m, _ := nodesOnScreen(t) // cursor on worker2, in the list
	line := lineWith(t, m, "worker2")
	plain := lineWith(t, m, "worker1")
	if ansi.StringWidth(line) != ansi.StringWidth(plain) {
		t.Errorf("selected row is %d wide, the others %d", ansi.StringWidth(line), ansi.StringWidth(plain))
	}
	if !strings.Contains(line, rgb(statusColors[commands.LevelOK])) {
		t.Errorf("selected row's status not colored: %q", line)
	}
}

// commandsRows lines up rows the way the loaders do.
func commandsRows(rows ...string) []string {
	cells := make([][]string, len(rows))
	for i, r := range rows {
		cells[i] = strings.Fields(r)
	}
	return commands.AlignColumns(cells)
}
