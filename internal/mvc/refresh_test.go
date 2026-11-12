package mvc

import (
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	k8s "k8s-tui/internal/kubernetes"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"k8s.io/client-go/kubernetes/fake"
)

// runBatch runs a command and returns every message it produces.
// A tea.Batch command returns a tea.BatchMsg (a list of commands); each of
// those is run too. nil commands produce nothing.
func runBatch(t *testing.T, cmd tea.Cmd) []tea.Msg {
	t.Helper()
	if cmd == nil {
		return nil
	}
	msg := cmd()
	batch, ok := msg.(tea.BatchMsg)
	if !ok {
		return []tea.Msg{msg}
	}
	var msgs []tea.Msg
	for _, c := range batch {
		msgs = append(msgs, runBatch(t, c)...)
	}
	return msgs
}

func hasLoaded(msgs []tea.Msg, tab string) bool {
	for _, msg := range msgs {
		if l, ok := msg.(itemsLoadedMsg); ok && l.tab == tab {
			return true
		}
	}
	return false
}

func hasTick(msgs []tea.Msg) bool {
	for _, msg := range msgs {
		if _, ok := msg.(refreshTickMsg); ok {
			return true
		}
	}
	return false
}

// fastClient returns a model with a fake cluster and a 1ms refresh, so
// timer commands can be run in tests without waiting.
func fastClient() Model {
	m := New(k8s.NewClientFromClientset(fake.NewClientset(readyNode("worker1"))))
	m.refreshEvery = time.Millisecond
	return m
}

func TestDefaultRefreshRate(t *testing.T) {
	if got := New(nil).refreshEvery; got != 5*time.Second {
		t.Errorf("refreshEvery = %s, want 5s", got)
	}
}

// Each tick reloads the active tab and schedules the next tick.
func TestTickReloadsActiveTabAndSchedulesNext(t *testing.T) {
	_, cmd := fastClient().Update(refreshTickMsg{})
	msgs := runBatch(t, cmd)
	if !hasLoaded(msgs, "Nodes") {
		t.Errorf("tick sent %#v, want a reload of Nodes", msgs)
	}
	if !hasTick(msgs) {
		t.Errorf("tick sent %#v, want the next tick scheduled", msgs)
	}
}

// A tab without a loader (placeholder data) still keeps the timer going.
func TestTickOnTabWithoutLoaderKeepsTicking(t *testing.T) {
	m := fastClient()
	m.active = 1 // Pods: no loader yet
	_, cmd := m.Update(refreshTickMsg{})
	msgs := runBatch(t, cmd)
	if hasLoaded(msgs, "Pods") || hasLoaded(msgs, "Nodes") {
		t.Errorf("tick sent %#v, want no load on a tab without a loader", msgs)
	}
	if !hasTick(msgs) {
		t.Errorf("tick sent %#v, want the next tick scheduled", msgs)
	}
}

func TestSwitchingTabLoadsItNow(t *testing.T) {
	m := fastClient()
	m.active = 1             // Pods
	_, cmd := m.Update(left) // back to Nodes
	if !hasLoaded(runBatch(t, cmd), "Nodes") {
		t.Error("switching to Nodes did not load it right away")
	}
}

func TestNoClientNoLoad(t *testing.T) {
	if cmd := New(nil).loadActive(); cmd != nil {
		t.Error("loadActive returned a command without a client")
	}
}

func TestLoadRecordsUpdateTime(t *testing.T) {
	at := time.Date(2026, 11, 12, 14, 30, 5, 0, time.Local)
	m := press(t, New(nil), itemsLoadedMsg{tab: "Nodes", rows: []string{"n1"}, at: at})
	if got := m.updated["Nodes"]; !got.Equal(at) {
		t.Errorf("updated[Nodes] = %v, want %v", got, at)
	}
}

func TestFailedLoadKeepsOldUpdateTime(t *testing.T) {
	at := time.Date(2026, 11, 12, 14, 30, 5, 0, time.Local)
	m := press(t, New(nil),
		itemsLoadedMsg{tab: "Nodes", rows: []string{"n1"}, at: at},
		itemsLoadedMsg{tab: "Nodes", err: errTest, at: at.Add(time.Minute)},
	)
	if got := m.updated["Nodes"]; !got.Equal(at) {
		t.Errorf("updated[Nodes] = %v, want the last successful time %v", got, at)
	}
}

// ---------- a: auto refresh on / off ----------

func TestAutoRefreshStartsOn(t *testing.T) {
	if !New(nil).autoRefresh {
		t.Error("autoRefresh = false at start, want true")
	}
}

func TestAutoRefreshTurnOff(t *testing.T) {
	m, cmd := fastClient().Update(autoKey)
	got := m.(Model)
	if got.autoRefresh {
		t.Error("autoRefresh = true after pressing a, want false")
	}
	if cmd != nil {
		t.Error("turning off returned a command, want none (no load, no timer)")
	}
}

func TestAutoRefreshTurnBackOnLoadsAndStartsTimer(t *testing.T) {
	m := press(t, fastClient(), autoKey) // off
	next, cmd := m.Update(autoKey)       // on again
	if !next.(Model).autoRefresh {
		t.Fatal("autoRefresh = false after pressing a twice, want true")
	}
	msgs := runBatch(t, cmd)
	if !hasLoaded(msgs, "Nodes") {
		t.Errorf("turning on sent %#v, want an immediate reload", msgs)
	}
	if !hasTick(msgs) {
		t.Errorf("turning on sent %#v, want the timer started", msgs)
	}
}

func TestTickIgnoredWhenOff(t *testing.T) {
	m := press(t, fastClient(), autoKey) // off
	if _, cmd := m.Update(refreshTickMsg{gen: m.tickGen}); cmd != nil {
		t.Error("a tick while off returned a command, want the timer to stop")
	}
}

// A tick from the timer that was running before a toggle or rate change is
// dropped, so two timers never run at once.
func TestOldTickIgnored(t *testing.T) {
	m := press(t, fastClient(), autoKey, autoKey) // off, on: new timer
	if _, cmd := m.Update(refreshTickMsg{gen: m.tickGen - 1}); cmd != nil {
		t.Error("a tick from the old timer returned a command, want it dropped")
	}
	if _, cmd := m.Update(refreshTickMsg{gen: m.tickGen}); cmd == nil {
		t.Error("a tick from the current timer returned nothing, want reload + next tick")
	}
}

// ---------- f: rate 2s / 5s / 10s ----------

func TestRateCycles(t *testing.T) {
	m := New(nil)
	for _, want := range []time.Duration{10 * time.Second, 2 * time.Second, 5 * time.Second} {
		m = press(t, m, rateKey)
		if m.refreshEvery != want {
			t.Fatalf("refreshEvery = %s, want %s", m.refreshEvery, want)
		}
	}
}

func TestRateChangeRestartsTimer(t *testing.T) {
	m := New(k8s.NewClientFromClientset(fake.NewClientset()))
	next, cmd := m.Update(rateKey)
	if cmd == nil {
		t.Fatal("changing the rate returned no command, want a new timer")
	}
	if next.(Model).tickGen == m.tickGen {
		t.Error("tickGen unchanged, want a new timer generation so the old tick is dropped")
	}
}

func TestRateChangeWhileOffStartsNoTimer(t *testing.T) {
	m := press(t, fastClient(), autoKey) // off
	next, cmd := m.Update(rateKey)
	if cmd != nil {
		t.Error("changing the rate while off returned a command, want none")
	}
	if next.(Model).refreshEvery == m.refreshEvery {
		t.Error("rate did not change while off; it should still be changeable")
	}
}

// ---------- buttons on the panel's top border ----------

// labels returns the buttons' labels, for comparing in tests.
func labels(buttons []button) []string {
	var out []string
	for _, b := range buttons {
		out = append(out, b.label)
	}
	return out
}

func TestRefreshButtonsOn(t *testing.T) {
	buttons := New(nil).refreshButtons()
	if got, want := labels(buttons), []string{"refresh: on", "every 5s"}; !slices.Equal(got, want) {
		t.Errorf("labels = %q, want %q", got, want)
	}
	for _, b := range buttons {
		if !b.on {
			t.Errorf("button %q is off, want on while auto refresh runs", b.label)
		}
	}
}

func TestRefreshButtonsOff(t *testing.T) {
	buttons := press(t, New(nil), autoKey).refreshButtons()
	if got, want := labels(buttons), []string{"refresh: off", "every 5s"}; !slices.Equal(got, want) {
		t.Errorf("labels = %q, want %q", got, want)
	}
	for _, b := range buttons {
		if b.on {
			t.Errorf("button %q is on, want off (gray) while auto refresh is off", b.label)
		}
	}
}

func TestRefreshButtonsShowRate(t *testing.T) {
	buttons := press(t, New(nil), rateKey).refreshButtons() // 5s → 10s
	if got := buttons[1].label; got != "every 10s" {
		t.Errorf("rate button = %q, want every 10s", got)
	}
}

func TestRefreshButtonsUpdatedTime(t *testing.T) {
	at := time.Date(2026, 11, 12, 14, 30, 5, 0, time.Local)
	m := press(t, New(nil), itemsLoadedMsg{tab: "Nodes", rows: []string{"n1"}, at: at})
	if got := labels(m.refreshButtons()); !slices.Contains(got, "updated 14:30:05") {
		t.Errorf("labels = %q, want updated 14:30:05", got)
	}

	// "updated" belongs to the active tab: Pods has never loaded.
	m.active = 1
	for _, l := range labels(m.refreshButtons()) {
		if strings.HasPrefix(l, "updated") {
			t.Errorf("on Pods: labels show %q, want no updated time", l)
		}
	}
}

// The buttons sit in the panel's top border: the line right under the tabs.
func TestViewButtonsOnPanelTopBorder(t *testing.T) {
	lines := strings.Split(sized(t, New(nil), 80, 24).View(), "\n")
	top := lines[1]
	if !strings.HasPrefix(top, "┌─") || !strings.HasSuffix(strings.TrimRight(top, " "), "┐") {
		t.Errorf("line under the tabs %q, want the panel's top border", top)
	}
	for _, want := range []string{"refresh: on", "every 5s"} {
		if !strings.Contains(top, want) {
			t.Errorf("top border %q is missing %q", top, want)
		}
	}
}

// The top border with buttons is exactly as wide as the rest of the box.
func TestWithButtonsKeepsWidth(t *testing.T) {
	box := Panel.Width(40).Height(3).Render("x")
	got := withButtons(box, []button{{label: "refresh: on", on: true}, {label: "every 5s"}}, lipgloss.Width(box))
	lines := strings.Split(got, "\n")
	for i, line := range lines {
		if w := lipgloss.Width(line); w != lipgloss.Width(box) {
			t.Errorf("line %d is %d wide, want %d: %q", i, w, lipgloss.Width(box), line)
		}
	}
	if len(lines) != lipgloss.Height(box) {
		t.Errorf("got %d lines, want %d (no line added or lost)", len(lines), lipgloss.Height(box))
	}
}

// When the buttons don't fit, the box keeps a plain top border.
func TestWithButtonsTooNarrow(t *testing.T) {
	box := Panel.Width(10).Height(1).Render("x")
	got := withButtons(box, []button{{label: "refresh: on", on: true}}, lipgloss.Width(box))
	if got != box {
		t.Errorf("box changed though the buttons don't fit:\n%s", got)
	}
}

// On a narrow terminal no line is wider than the terminal.
func TestViewFitsWidthWhenNarrow(t *testing.T) {
	for _, line := range strings.Split(sized(t, New(nil), 30, 24).View(), "\n") {
		if w := lipgloss.Width(line); w > 30 {
			t.Errorf("line %q is %d wide, want at most 30", line, w)
		}
	}
}

// When a refresh returns fewer rows, the cursor moves onto the last one left.
func TestCursorClampedWhenRowsShrink(t *testing.T) {
	m := press(t, testModel(), down, down, down) // cursor on row 2 of 3
	m = press(t, m, itemsLoadedMsg{tab: "Nodes", rows: []string{"only-one"}})
	if m.cursor != 0 {
		t.Errorf("cursor = %d, want 0 after rows shrank to 1", m.cursor)
	}
}

// A refresh of another tab must not move the cursor on the active one.
func TestCursorKeptWhenOtherTabLoads(t *testing.T) {
	m := press(t, testModel(), down, down, down) // cursor on row 2 of Nodes
	m = press(t, m, itemsLoadedMsg{tab: "Pods", rows: nil})
	if m.cursor != 2 {
		t.Errorf("cursor = %d, want 2 (unchanged)", m.cursor)
	}
}

// errTest is a fixed error for tests.
var errTest = errors.New("test error")

// Key presses for auto refresh.
var (
	autoKey = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(KeyAutoRefresh)}
	rateKey = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(KeyRefreshRate)}
)
