package mvc

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"k8s-tui/internal/mvc/commands"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	wheelDown = tea.MouseMsg{Action: tea.MouseActionPress, Button: tea.MouseButtonWheelDown}
	wheelUp   = tea.MouseMsg{Action: tea.MouseActionPress, Button: tea.MouseButtonWheelUp}
)

// manyRows is a model on Nodes with n rows loaded, on the tabs.
func manyRows(t *testing.T, n int) Model {
	t.Helper()
	rows := make([]string, n)
	for i := range rows {
		rows[i] = fmt.Sprint("node", i)
	}
	m := sized(t, New(nil), 100, 20)
	return press(t, m, commands.ItemsLoadedMsg{Tab: tabNodes, Rows: rows, At: time.Now()})
}

// One step of the wheel is one row, and brings the focus to the list.
func TestWheelMovesOneRow(t *testing.T) {
	m := press(t, manyRows(t, 50), wheelDown)
	if m.focus != focusList || m.cursor != 1 {
		t.Fatalf("focus = %d, cursor = %d, want the list, row 1", m.focus, m.cursor)
	}
	m = press(t, m, wheelDown, wheelDown, wheelUp)
	if m.cursor != 2 {
		t.Errorf("cursor = %d, want 2", m.cursor)
	}
}

// The wheel scrolls the list once the cursor reaches the bottom.
func TestWheelScrollsList(t *testing.T) {
	m := manyRows(t, 50)
	for range 30 {
		m = press(t, m, wheelDown)
	}
	if m.cursor != 30 || m.offset != 30-m.listHeight()+1 {
		t.Errorf("cursor = %d, offset = %d", m.cursor, m.offset)
	}
}

// At the top row the wheel stays in the list: it doesn't go to the buttons.
func TestWheelUpStaysInList(t *testing.T) {
	m := press(t, manyRows(t, 5), wheelDown, wheelUp, wheelUp, wheelUp)
	if m.focus != focusList || m.cursor != 0 {
		t.Errorf("focus = %d, cursor = %d, want the list's first row", m.focus, m.cursor)
	}
}

// On a page of text the wheel scrolls it, within its lines.
func TestWheelScrollsText(t *testing.T) {
	m := manyRows(t, 5)
	m = m.showText(strings.Repeat("line\n", 100))
	m = press(t, m, wheelDown, wheelDown)
	if m.textOffset != 2 {
		t.Errorf("offset = %d, want 2", m.textOffset)
	}
	m = press(t, m, wheelUp, wheelUp, wheelUp)
	if m.textOffset != 0 {
		t.Errorf("offset = %d, want 0", m.textOffset)
	}
}

// On the action list the wheel moves its cursor.
func TestWheelMovesActionCursor(t *testing.T) {
	m, _ := nodesOnScreen(t)
	m = press(t, m, enterKey, wheelDown, wheelDown)
	if m.overlay != overlayActions || m.overlayCursor != 2 {
		t.Errorf("overlay = %d, cursor = %d", m.overlay, m.overlayCursor)
	}
}

// Clicks and releases do nothing.
func TestMouseClickIgnored(t *testing.T) {
	m := manyRows(t, 5)
	m = press(t, m, tea.MouseMsg{Action: tea.MouseActionPress, Button: tea.MouseButtonLeft},
		tea.MouseMsg{Action: tea.MouseActionRelease, Button: tea.MouseButtonWheelDown})
	if m.focus != focusTabs || m.cursor != 0 {
		t.Errorf("focus = %d, cursor = %d, want nothing changed", m.focus, m.cursor)
	}
}

// ---------- clicks ----------

// at finds text on the screen m draws and returns where it is: its first
// column and its line. It fails the test if it isn't there.
func at(t *testing.T, m Model, text string) (x, y int) {
	t.Helper()
	for y, line := range strings.Split(ansi.Strip(m.View()), "\n") {
		if i := strings.Index(line, text); i >= 0 {
			return ansi.StringWidth(line[:i]), y
		}
	}
	t.Fatalf("%q is not on the screen:\n%s", text, ansi.Strip(m.View()))
	return 0, 0
}

// click is a left click on the middle of text on the screen.
func click(t *testing.T, m Model, text string) Model {
	t.Helper()
	x, y := at(t, m, text)
	return press(t, m, tea.MouseMsg{X: x + ansi.StringWidth(text)/2, Y: y, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft})
}

var rightClick = tea.MouseMsg{Action: tea.MouseActionPress, Button: tea.MouseButtonRight}

func TestClickTab(t *testing.T) {
	m := click(t, manyRows(t, 3), "Services")
	if m.activeName() != tabServices || m.focus != focusTabs {
		t.Errorf("active = %q, focus = %d", m.activeName(), m.focus)
	}
}

func TestClickButton(t *testing.T) {
	m := click(t, manyRows(t, 3), "wide: off")
	if !m.activeWide() || m.focus != focusButtons {
		t.Errorf("wide = %v, focus = %d", m.activeWide(), m.focus)
	}
	m = click(t, m, "/ search")
	if !m.searching {
		t.Error("clicking search didn't start a search")
	}
}

// A click selects a row; a second click on it opens its actions.
func TestClickRowThenActions(t *testing.T) {
	m, _ := nodesOnScreen(t)
	m = click(t, m, "worker1")
	if m.focus != focusList || m.cursor != 0 || m.overlay != overlayNone {
		t.Fatalf("focus = %d, cursor = %d, overlay = %d", m.focus, m.cursor, m.overlay)
	}
	m = click(t, m, "worker1")
	if m.overlay != overlayActions || m.target != "worker1" {
		t.Errorf("overlay = %d, target = %q, want worker1's actions", m.overlay, m.target)
	}
}

// Clicks pick an action, tick a flag and run it; right click goes back.
func TestClickConfirmPage(t *testing.T) {
	m, cs := nodesOnScreen(t) // cursor on worker2
	m = press(t, m, enterKey)
	m = click(t, m, "Cordon")
	if m.overlay != overlayConfirm || m.chosen().Name != "Cordon" {
		t.Fatalf("overlay = %d, chosen = %q", m.overlay, m.chosen().Name)
	}
	m = click(t, m, "--dry-run=server")
	if !commands.Ticked(m.flags, commands.FlagDryRun) {
		t.Fatal("clicking the tick box didn't tick it")
	}
	if m = press(t, m, rightClick); m.overlay != overlayActions {
		t.Fatalf("right click: overlay = %d, want back on the actions", m.overlay)
	}
	m = click(t, m, "Cordon")
	next, cmd := m.Update(tea.MouseMsg{X: 5, Y: lastRowY(t, m, "y run"), Action: tea.MouseActionPress, Button: tea.MouseButtonLeft})
	m = run(t, next.(Model), cmd)
	if m.notice != "Cordon worker2: done" {
		t.Errorf("notice = %q", m.notice)
	}
	if n, _ := cs.CoreV1().Nodes().Get(t.Context(), "worker2", metav1.GetOptions{}); !n.Spec.Unschedulable {
		t.Error("worker2 wasn't cordoned")
	}
}

// lastRowY is the line of text on the screen.
func lastRowY(t *testing.T, m Model, text string) int {
	t.Helper()
	_, y := at(t, m, text)
	return y
}

// Clicking the text field puts the cursor there.
func TestClickTextField(t *testing.T) {
	m, _ := nodesOnScreen(t)
	m = press(t, m, enterKey)
	m = click(t, m, "Label")
	m = press(t, m, down) // off the field, to the first flag
	if m.typing() {
		t.Fatal("still typing after ↓")
	}
	if m = click(t, m, "labels:"); !m.typing() {
		t.Error("clicking the field didn't put the cursor in it")
	}
}

// Clicking a namespace picks it.
func TestClickNamespace(t *testing.T) {
	m, _ := podsOnScreen(t)
	m = click(t, m, "ns: all")
	m = press(t, m, namespacesLoadedMsg{names: []string{"default", "kube-system"}})
	if m = click(t, m, "kube-system"); m.namespace != "kube-system" || m.overlay != overlayNone {
		t.Errorf("namespace = %q, overlay = %d", m.namespace, m.overlay)
	}
}

// Clicking a container picks it.
func TestClickContainer(t *testing.T) {
	m, _ := podsOnScreen(t)
	m = press(t, m, down, down, down, enterKey, key("o"))
	m = press(t, m, containersLoadedMsg{target: "default/web", names: []string{"app", "sidecar"}})
	if m = click(t, m, "sidecar"); m.overlay != overlayConfirm || m.input != "sidecar" {
		t.Errorf("overlay = %d, input = %q", m.overlay, m.input)
	}
}

// A click on a tab while a page is open closes the page.
func TestClickTabClosesPage(t *testing.T) {
	m, _ := nodesOnScreen(t)
	m = press(t, m, enterKey)
	if m = click(t, m, "Pods"); m.overlay != overlayNone || m.activeName() != tabPods {
		t.Errorf("overlay = %d, active = %q", m.overlay, m.activeName())
	}
}
