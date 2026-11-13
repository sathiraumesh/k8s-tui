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
	m = press(t, m, enterKey)
	for range nodeActions() { // one more ↓ than needed: stops on the last
		m = press(t, m, down)
	}
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
	if nodeActions()[2].flags[0].on {
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
	if m.overlay != overlayActions || m.chosen().name != "Cordon" || m.overlayCursor != m.chosenAction {
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
	m = press(t, m, enterKey, down, down) // on Cordon
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

// ---------- actions with a text field: Label and Taint ----------

var backspaceKey = tea.KeyMsg{Type: tea.KeyBackspace}

// typeText types s into the text field one key at a time, as a person would.
func typeText(t *testing.T, m Model, s string) Model {
	t.Helper()
	for _, r := range s {
		k := key(string(r))
		if r == ' ' {
			k = spaceKey
		}
		m = press(t, m, k)
	}
	return m
}

// The cursor starts in the text field, where every letter types, even the
// ones that are keys elsewhere (q, y, n, j, k).
func TestTypingInTextField(t *testing.T) {
	m, _ := nodesOnScreen(t)
	m = press(t, m, enterKey, key("l"))
	if !m.typing() {
		t.Fatal("cursor not in the text field on the Label page")
	}
	next, cmd := m.Update(key("q"))
	if cmd != nil {
		t.Fatal("q quit while typing")
	}
	m = typeText(t, next.(Model), "ynjk env=prod")
	m = press(t, m, backspaceKey)
	if m.input != "qynjk env=pro" {
		t.Errorf("input = %q", m.input)
	}
	if m.kubectlCommand() != "kubectl label node worker2 qynjk env=pro" {
		t.Errorf("command = %q", m.kubectlCommand())
	}
}

// Pasted text is added in one go.
func TestPasteInTextField(t *testing.T) {
	m, _ := nodesOnScreen(t)
	m = press(t, m, enterKey, key("l"), tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("env=prod team=a"), Paste: true})
	if m.input != "env=prod team=a" {
		t.Errorf("input = %q", m.input)
	}
}

// enter leaves the field for the flags; ↑ goes back to it. Space then ticks
// the flag under the cursor, not the one above it.
func TestTextFieldToFlags(t *testing.T) {
	m, _ := nodesOnScreen(t)
	m = typeText(t, press(t, m, enterKey, key("l")), "env=prod")
	m = press(t, m, enterKey, down, spaceKey) // to --overwrite, then --dry-run=server
	if m.typing() || !ticked(m.flags, flagDryRun) || ticked(m.flags, flagOverwrite) {
		t.Errorf("typing = %v, flags = %+v, want only --dry-run=server ticked", m.typing(), m.flags)
	}
	if m.kubectlCommand() != "kubectl label node worker2 env=prod --dry-run=server" {
		t.Errorf("command = %q", m.kubectlCommand())
	}
	m = press(t, m, up, up)
	if !m.typing() {
		t.Error("↑ from the first flag didn't go back to the text field")
	}
}

// y with nothing typed doesn't run: it asks for the labels.
func TestEmptyTextFieldDoesNotRun(t *testing.T) {
	m, _ := nodesOnScreen(t)
	m = press(t, m, enterKey, key("l"), enterKey)
	next, cmd := m.Update(key("y"))
	m = next.(Model)
	if cmd != nil || m.overlay != overlayConfirm || m.notice != "type the labels first" || !m.typing() {
		t.Errorf("cmd = %v, overlay = %d, notice = %q, typing = %v", cmd != nil, m.overlay, m.notice, m.typing())
	}
}

// Picking an action again starts with an empty field.
func TestTextFieldClearedOnPick(t *testing.T) {
	m, _ := nodesOnScreen(t)
	m = typeText(t, press(t, m, enterKey, key("l")), "env=prod")
	m = press(t, m, escKey, key("t"))
	if m.input != "" {
		t.Errorf("input = %q on a newly picked action, want empty", m.input)
	}
}

func TestLabelRuns(t *testing.T) {
	m, cs := nodesOnScreen(t)
	m = typeText(t, press(t, m, enterKey, key("l")), "env=prod")
	_, cmd := press(t, m, enterKey).Update(key("y"))
	m = run(t, m, cmd)
	if m.notice != "Label worker2: done" {
		t.Errorf("notice = %q", m.notice)
	}
	n, _ := cs.CoreV1().Nodes().Get(context.Background(), "worker2", metav1.GetOptions{})
	if n.Labels["env"] != "prod" {
		t.Errorf("labels = %v, want env=prod", n.Labels)
	}
}

func TestTaintRuns(t *testing.T) {
	m, cs := nodesOnScreen(t)
	m = typeText(t, press(t, m, enterKey, key("t")), "gpu=yes:NoSchedule")
	_, cmd := press(t, m, enterKey).Update(key("y"))
	m = run(t, m, cmd)
	if m.notice != "Taint worker2: done" {
		t.Errorf("notice = %q", m.notice)
	}
	n, _ := cs.CoreV1().Nodes().Get(context.Background(), "worker2", metav1.GetOptions{})
	if len(n.Spec.Taints) != 1 || n.Spec.Taints[0].ToString() != "gpu=yes:NoSchedule" {
		t.Errorf("taints = %v", n.Spec.Taints)
	}
}

// A bad taint comes back as a failed notice, and the cluster is unchanged.
func TestBadTaintFails(t *testing.T) {
	m, _ := nodesOnScreen(t)
	m = typeText(t, press(t, m, enterKey, key("t")), "gpu=yes")
	_, cmd := press(t, m, enterKey).Update(key("y"))
	m = run(t, m, cmd)
	if !strings.HasPrefix(m.notice, "Taint worker2 failed:") {
		t.Errorf("notice = %q", m.notice)
	}
}

func TestViewTextField(t *testing.T) {
	m, _ := nodesOnScreen(t)
	m = typeText(t, press(t, m, enterKey, key("l")), "env=prod")
	out := m.View()
	for _, want := range []string{"labels: env=prod█", "[ ] --overwrite", "[ ] --dry-run=server", "type the labels"} {
		if !strings.Contains(out, want) {
			t.Errorf("Label page is missing %q:\n%s", want, out)
		}
	}
	if _, bottom := topAndBottom(t, m); !strings.Contains(bottom, "kubectl label node worker2 env=prod") {
		t.Errorf("bottom border = %q", bottom)
	}
	// On the flags the caret goes and the help line is the confirm one.
	m = press(t, m, enterKey)
	if strings.Contains(m.View(), "█") || !strings.Contains(m.helpText(), "y run") {
		t.Errorf("after enter: caret still shown or help = %q", m.helpText())
	}
}

// ---------- Show, and what a dry run would change ----------

// s shows the node's labels and taints straight away: it has no flags.
func TestShowLabelsAndTaints(t *testing.T) {
	m, cs := nodesOnScreen(t)
	n, _ := cs.CoreV1().Nodes().Get(context.Background(), "worker2", metav1.GetOptions{})
	n.Labels = map[string]string{"env": "prod"}
	cs.CoreV1().Nodes().Update(context.Background(), n, metav1.UpdateOptions{})

	next, cmd := press(t, m, enterKey).Update(key("s"))
	m = run(t, next.(Model), cmd)
	if m.overlay != overlayText {
		t.Fatalf("overlay = %d, want the text page", m.overlay)
	}
	text := strings.Join(m.text, "\n")
	for _, want := range []string{"Labels:", "  env=prod", "Taints:", "  (none)"} {
		if !strings.Contains(text, want) {
			t.Errorf("Show page is missing %q:\n%s", want, text)
		}
	}
	if m.kubectlCommand() != "kubectl get node worker2 --show-labels" {
		t.Errorf("command = %q", m.kubectlCommand())
	}
}

// A dry run of Label opens a page of what it would change, and doesn't
// reload the list: nothing changed.
func TestLabelDryRunShowsChange(t *testing.T) {
	m, _ := nodesOnScreen(t)
	m = typeText(t, press(t, m, enterKey, key("l")), "env=prod")
	m = press(t, m, enterKey, down, spaceKey) // tick --dry-run=server
	_, cmd := m.Update(key("y"))
	next, reload := m.Update(cmd())
	m = next.(Model)
	if reload != nil {
		t.Error("a dry run reloaded the list")
	}
	if m.overlay != overlayText || !strings.Contains(m.notice, "dry run passed") {
		t.Fatalf("overlay = %d, notice = %q", m.overlay, m.notice)
	}
	if text := strings.Join(m.text, "\n"); !strings.Contains(text, "Labels after the change:") || !strings.Contains(text, "+ env=prod") {
		t.Errorf("dry run page:\n%s", text)
	}
}

func TestChangeText(t *testing.T) {
	got := changeText("Labels", []string{"keep=1", "old=x"}, []string{"env=prod", "keep=1"})
	want := "Labels after the change:\n\n+ env=prod\n  keep=1\n- old=x"
	if got != want {
		t.Errorf("changeText =\n%s\nwant\n%s", got, want)
	}
	if got := changeText("Taints", nil, nil); !strings.HasSuffix(got, "(none)") {
		t.Errorf("no taints: %q", got)
	}
}
