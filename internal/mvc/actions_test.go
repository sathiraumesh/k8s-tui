package mvc

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	k8s "k8s-tui/internal/kubernetes"

	tea "github.com/charmbracelet/bubbletea"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

var escKey = tea.KeyMsg{Type: tea.KeyEsc}

// key is the key press for one letter, e.g. key("c").
func key(s string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

// nodesOnScreen is a model on a fake cluster with nodes worker1 and worker2,
// their rows loaded, on a 100×24 screen, with the cursor on worker2.
func nodesOnScreen(t *testing.T) (Model, *fake.Clientset) {
	t.Helper()
	cs := fake.NewClientset(readyNode("worker1"), readyNode("worker2"))
	m := sized(t, New(k8s.NewClientFromClientset(cs)), 100, 24)
	m = press(t, m, itemsLoadedMsg{tab: tabNodes, rows: []string{
		"worker1   Ready   v1.37.2",
		"worker2   Ready   v1.37.2",
	}, at: time.Now()})
	return press(t, m, down, down, down), cs // tabs → buttons → list row 0 → row 1
}

// run runs cmd and passes its message to Update, as Bubble Tea would.
func run(t *testing.T, m Model, cmd tea.Cmd) Model {
	t.Helper()
	if cmd == nil {
		t.Fatal("no command to run")
	}
	return press(t, m, cmd())
}

// ---------- opening the action list ----------

func TestEnterOpensActionsForRow(t *testing.T) {
	m, _ := nodesOnScreen(t)
	m = press(t, m, enterKey)
	if m.overlay != overlayActions || m.target != "worker2" {
		t.Errorf("overlay = %d, target = %q, want the actions for worker2", m.overlay, m.target)
	}
}

func TestEnterDoesNothingWhileLoading(t *testing.T) {
	m := press(t, sized(t, New(nil), 100, 24), down, down) // Nodes still "loading..."
	if m = press(t, m, enterKey); m.overlay != overlayNone {
		t.Error("actions opened on the loading... row")
	}
}

func TestEnterDoesNothingOnTabWithoutActions(t *testing.T) {
	m, _ := nodesOnScreen(t)
	m.active = 1 // Pods: no actions
	m = press(t, m, itemsLoadedMsg{tab: tabPods, rows: []string{"nginx"}, at: time.Now()})
	if m = press(t, m, enterKey); m.overlay != overlayNone {
		t.Error("actions opened on a tab that has none")
	}
}

func TestEnterOnTabsDoesNotOpenActions(t *testing.T) {
	m, _ := nodesOnScreen(t)
	m.focus = focusTabs
	if m = press(t, m, enterKey); m.overlay != overlayNone {
		t.Error("actions opened from the tab bar")
	}
}

func TestRowName(t *testing.T) {
	for row, want := range map[string]string{
		"worker1   Ready   v1.37.2": "worker1",
		"  spaced  x":               "spaced",
		"":                          "",
	} {
		if got := rowName(row); got != want {
			t.Errorf("rowName(%q) = %q, want %q", row, got, want)
		}
	}
}

// ---------- the action list ----------

func TestActionListMovesAndCloses(t *testing.T) {
	m, _ := nodesOnScreen(t)
	m = press(t, m, enterKey, down, down, down, down)
	if m.overlayCursor != len(nodeActions())-1 {
		t.Errorf("cursor = %d, want the last action", m.overlayCursor)
	}
	m = press(t, m, up)
	if m.overlayCursor != len(nodeActions())-2 {
		t.Errorf("cursor = %d after ↑", m.overlayCursor)
	}
	if m = press(t, m, escKey); m.overlay != overlayNone || m.focus != focusList {
		t.Errorf("esc: overlay = %d, focus = %d, want back on the list", m.overlay, m.focus)
	}
}

// Keys that aren't the page's own do nothing there: no tab switch, no wide.
func TestActionListIgnoresOtherKeys(t *testing.T) {
	m, _ := nodesOnScreen(t)
	m = press(t, m, enterKey, right, wideKey, rateKey)
	if m.overlay != overlayActions || m.active != 0 || m.activeWide() || m.refreshEvery != refreshNormal {
		t.Errorf("a key outside the page changed something: %+v", m)
	}
}

func TestActionKeyPicks(t *testing.T) {
	m, _ := nodesOnScreen(t)
	m = press(t, m, enterKey, key("c"))
	if m.overlay != overlayConfirm || m.chosen().name != "Cordon" {
		t.Errorf("overlay = %d, chosen = %q, want the Cordon confirm page", m.overlay, m.chosen().name)
	}
}

// ---------- the confirm page ----------

func TestSpaceTicksFlag(t *testing.T) {
	m, _ := nodesOnScreen(t)
	m = press(t, m, enterKey, key("c"), spaceKey)
	if !ticked(m.flags, flagDryRun) {
		t.Fatal("--dry-run=server not ticked after space")
	}
	if m.kubectlCommand() != "kubectl cordon worker2 --dry-run=server" {
		t.Errorf("command = %q, want the dry-run flag added", m.kubectlCommand())
	}
	// Ticking must not change the action's defaults.
	if nodeActions()[1].flags[0].on {
		t.Error("ticking changed the action table's default")
	}
}

// A change needs y: enter does nothing.
func TestEnterDoesNotRunChange(t *testing.T) {
	m, cs := nodesOnScreen(t)
	m = press(t, m, enterKey, key("c"))
	next, cmd := m.Update(enterKey)
	if cmd != nil || next.(Model).overlay != overlayConfirm {
		t.Error("enter ran a change, want only y to")
	}
	if n, _ := cs.CoreV1().Nodes().Get(context.Background(), "worker2", metav1.GetOptions{}); n.Spec.Unschedulable {
		t.Error("worker2 was cordoned without y")
	}
}

func TestNGoesBackToActions(t *testing.T) {
	m, _ := nodesOnScreen(t)
	m = press(t, m, enterKey, key("c"), key("n"))
	if m.overlay != overlayActions || m.overlayCursor != 1 {
		t.Errorf("overlay = %d, cursor = %d, want the action list on Cordon", m.overlay, m.overlayCursor)
	}
}

// ---------- running ----------

// y runs Cordon on the cluster, then the list reloads.
func TestCordonRuns(t *testing.T) {
	m, cs := nodesOnScreen(t)
	m = press(t, m, enterKey, key("c"))
	next, cmd := m.Update(key("y"))
	m = next.(Model)
	if m.overlay != overlayNone || !strings.HasPrefix(m.notice, "running: kubectl cordon worker2") {
		t.Errorf("while running: overlay = %d, notice = %q", m.overlay, m.notice)
	}

	next, reload := m.Update(cmd())
	m = next.(Model)
	if m.notice != "Cordon worker2: done" {
		t.Errorf("notice = %q, want Cordon worker2: done", m.notice)
	}
	if reload == nil {
		t.Error("no reload after a change")
	}
	n, _ := cs.CoreV1().Nodes().Get(context.Background(), "worker2", metav1.GetOptions{})
	if !n.Spec.Unschedulable {
		t.Error("worker2 is not cordoned on the cluster")
	}
}

func TestUncordonRuns(t *testing.T) {
	m, cs := nodesOnScreen(t)
	n, _ := cs.CoreV1().Nodes().Get(context.Background(), "worker2", metav1.GetOptions{})
	n.Spec.Unschedulable = true
	cs.CoreV1().Nodes().Update(context.Background(), n, metav1.UpdateOptions{})

	m = press(t, m, enterKey, key("u"))
	_, cmd := m.Update(key("y"))
	m = run(t, m, cmd)
	if m.notice != "Uncordon worker2: done" {
		t.Errorf("notice = %q", m.notice)
	}
	if n, _ := cs.CoreV1().Nodes().Get(context.Background(), "worker2", metav1.GetOptions{}); n.Spec.Unschedulable {
		t.Error("worker2 is still cordoned")
	}
}

func TestDryRunNotice(t *testing.T) {
	m, _ := nodesOnScreen(t)
	m = press(t, m, actionDoneMsg{action: "Cordon", target: "worker2", changed: true, dryRun: true})
	if m.notice != "Cordon worker2: dry run passed, nothing changed" {
		t.Errorf("notice = %q", m.notice)
	}
}

func TestFailedActionNotice(t *testing.T) {
	m, _ := nodesOnScreen(t)
	m = press(t, m, actionDoneMsg{action: "Cordon", target: "worker2", changed: true, err: errors.New("forbidden")})
	if m.notice != "Cordon worker2 failed: forbidden" {
		t.Errorf("notice = %q", m.notice)
	}
}

// The notice goes away with the next key press.
func TestNoticeClearedByKey(t *testing.T) {
	m, _ := nodesOnScreen(t)
	m.notice = "Cordon worker2: done"
	if m = press(t, m, down); m.notice != "" {
		t.Errorf("notice = %q after a key, want it cleared", m.notice)
	}
}

// ---------- Describe: a read, with a page of text ----------

func TestDescribeShowsText(t *testing.T) {
	m, _ := nodesOnScreen(t)
	m = press(t, m, enterKey, key("d"))
	if m.overlay != overlayConfirm {
		t.Fatalf("overlay = %d, want the confirm page (Describe has a flag)", m.overlay)
	}
	_, cmd := m.Update(enterKey) // a read runs with enter
	m = run(t, m, cmd)
	if m.overlay != overlayText {
		t.Fatalf("overlay = %d, want the text page", m.overlay)
	}
	if !strings.Contains(strings.Join(m.text, "\n"), "worker2") {
		t.Errorf("text = %q, want the describe output for worker2", m.text)
	}
	if m = press(t, m, escKey); m.overlay != overlayActions {
		t.Errorf("esc on the text page: overlay = %d, want the action list", m.overlay)
	}
}

func TestTextPageScrolls(t *testing.T) {
	m, _ := nodesOnScreen(t)
	m = press(t, m, enterKey, key("d"))
	lines := make([]string, 100)
	for i := range lines {
		lines[i] = "line"
	}
	m = press(t, m, actionDoneMsg{action: "Describe", target: "worker2", text: strings.Join(lines, "\n")})
	m = press(t, m, down, down, pgDown)
	if want := 2 + m.listHeight(); m.textOffset != want {
		t.Errorf("textOffset = %d, want %d", m.textOffset, want)
	}
	for range 20 {
		m = press(t, m, pgDown)
	}
	if want := 100 - m.listHeight(); m.textOffset != want {
		t.Errorf("textOffset = %d at the end, want %d", m.textOffset, want)
	}
}

// ---------- on screen ----------

func topAndBottom(t *testing.T, m Model) (top, bottom string) {
	t.Helper()
	lines := strings.Split(m.View(), "\n")
	return lines[1+panelMarginY], lines[len(lines)-2-panelMarginY]
}

func TestViewActionList(t *testing.T) {
	m, _ := nodesOnScreen(t)
	m = press(t, m, enterKey, down) // on Cordon
	out := m.View()
	for _, want := range []string{"Describe", "Cordon", "Uncordon", "stop new pods"} {
		if !strings.Contains(out, want) {
			t.Errorf("action list is missing %q", want)
		}
	}
	top, bottom := topAndBottom(t, m)
	if !strings.Contains(top, "Actions: worker2") || strings.Contains(top, "refresh") {
		t.Errorf("top border = %q, want the title instead of the buttons", top)
	}
	if !strings.Contains(bottom, "$ kubectl cordon worker2") {
		t.Errorf("bottom border = %q, want the selected action's command", bottom)
	}
}

func TestViewConfirmPage(t *testing.T) {
	m, _ := nodesOnScreen(t)
	m = press(t, m, enterKey, key("c"))
	out := m.View()
	if !strings.Contains(out, "[ ] --dry-run=server") || !strings.Contains(out, "y run") {
		t.Errorf("confirm page is missing the tick box or y run:\n%s", out)
	}
	m = press(t, m, spaceKey)
	if !strings.Contains(m.View(), "[x] --dry-run=server") {
		t.Error("tick box not ticked after space")
	}
	if top, _ := topAndBottom(t, m); !strings.Contains(top, "Cordon worker2") {
		t.Errorf("top border = %q, want Cordon worker2", top)
	}
}

func TestHelpLineFollowsPage(t *testing.T) {
	m, _ := nodesOnScreen(t)
	if h := m.helpText(); !strings.Contains(h, "enter actions") {
		t.Errorf("list help = %q", h)
	}
	m = press(t, m, enterKey)
	if h := m.helpText(); !strings.Contains(h, "c Cordon") || !strings.Contains(h, "esc back") {
		t.Errorf("action list help = %q", h)
	}
	m = press(t, m, key("c"))
	if h := m.helpText(); !strings.Contains(h, "y run") {
		t.Errorf("confirm help = %q", h)
	}
}

// Every node action has a key, a name, a kubectl command and a run.
func TestNodeActionsComplete(t *testing.T) {
	seen := map[string]bool{}
	for _, a := range nodeActions() {
		if a.key == "" || a.name == "" || !strings.HasPrefix(a.command, "kubectl ") || a.run == nil {
			t.Errorf("incomplete action %+v", a)
		}
		if seen[a.key] {
			t.Errorf("key %q used twice", a.key)
		}
		seen[a.key] = true
	}
}
