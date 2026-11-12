package mvc

import (
	"errors"
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
	if got := New(nil).refreshEvery; got != refreshNormal {
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

// A placeholder tab (not loaded from the cluster yet) is refreshed like any
// other: the tick reloads it, and only it.
func TestTickReloadsPlaceholderTab(t *testing.T) {
	m := fastClient()
	m.active = 1 // Pods: placeholder rows
	_, cmd := m.Update(refreshTickMsg{})
	msgs := runBatch(t, cmd)
	if !hasLoaded(msgs, tabPods) {
		t.Errorf("tick sent %#v, want a reload of Pods", msgs)
	}
	if hasLoaded(msgs, tabNodes) {
		t.Errorf("tick sent %#v, want only the active tab reloaded", msgs)
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

// ---------- r / the refresh button: off → 2s → 5s → 10s → off ----------

func TestRateCycles(t *testing.T) {
	m := New(nil) // starts at 5s
	for _, want := range []time.Duration{refreshSlow, refreshOff, refreshFast, refreshNormal} {
		m = press(t, m, rateKey)
		if m.refreshEvery != want {
			t.Fatalf("refreshEvery = %s, want %s", m.refreshEvery, want)
		}
	}
}

func TestTurningOffStopsTimer(t *testing.T) {
	m := New(k8s.NewClientFromClientset(fake.NewClientset()))
	m.refreshEvery = refreshSlow // the step before off
	next, cmd := m.Update(rateKey)
	if next.(Model).refreshEvery != 0 {
		t.Fatalf("refreshEvery = %s, want off (0)", next.(Model).refreshEvery)
	}
	if cmd != nil {
		t.Error("turning off returned a command, want none (no load, no timer)")
	}
}

func TestTickIgnoredWhenOff(t *testing.T) {
	m := New(nil)
	m.refreshEvery = refreshOff
	if _, cmd := m.Update(refreshTickMsg{gen: m.tickGen}); cmd != nil {
		t.Error("a tick while off returned a command, want the timer to stop")
	}
}

// Turning on from off reloads straight away and starts the timer.
// (Only the load is run here: running the timer would wait 2s.)
func TestTurningOnLoadsAndStartsTimer(t *testing.T) {
	m := fastClient()
	m.refreshEvery = refreshOff // off
	next, cmd := m.Update(rateKey)
	if got := next.(Model).refreshEvery; got != refreshFast {
		t.Fatalf("refreshEvery = %s, want 2s", got)
	}
	batch, ok := cmd().(tea.BatchMsg)
	if !ok || len(batch) != 2 {
		t.Fatalf("turning on sent %#v, want a batch of load + timer", cmd())
	}
	if msg, ok := batch[0]().(itemsLoadedMsg); !ok || msg.tab != "Nodes" {
		t.Errorf("first command sent %#v, want a reload of Nodes", msg)
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

// A tick from the timer that was running before a rate change is dropped,
// so two timers never run at once.
func TestOldTickIgnored(t *testing.T) {
	m := press(t, New(k8s.NewClientFromClientset(fake.NewClientset())), rateKey) // 5s → 10s: new timer
	if _, cmd := m.Update(refreshTickMsg{gen: m.tickGen - 1}); cmd != nil {
		t.Error("a tick from the old timer returned a command, want it dropped")
	}
	if _, cmd := m.Update(refreshTickMsg{gen: m.tickGen}); cmd == nil {
		t.Error("a tick from the current timer returned nothing, want the next tick")
	}
}

// ---------- the refresh button on the panel's top border ----------

// buttonNamed returns the button with the given name from m's buttons.
func buttonNamed(t *testing.T, m Model, name string) button {
	t.Helper()
	for _, b := range m.panelButtons() {
		if b.name == name {
			return b
		}
	}
	t.Fatalf("no %q button in %+v", name, m.panelButtons())
	return button{}
}

func TestRefreshButtonOn(t *testing.T) {
	b := buttonNamed(t, New(nil), buttonRefresh)
	if b.label != "refresh: 5s" || !b.on {
		t.Errorf("refresh button = %+v, want green \"refresh: 5s\"", b)
	}
}

func TestRefreshButtonOff(t *testing.T) {
	m := New(nil)
	m.refreshEvery = refreshOff
	if b := buttonNamed(t, m, buttonRefresh); b.label != "refresh: off" || b.on {
		t.Errorf("refresh button = %+v, want gray \"refresh: off\"", b)
	}
}

func TestRefreshButtonShowsNewRate(t *testing.T) {
	b := buttonNamed(t, press(t, New(nil), rateKey), buttonRefresh) // 5s → 10s
	if b.label != "refresh: 10s" {
		t.Errorf("label = %q, want refresh: 10s", b.label)
	}
}

// The button sits in the panel's top border: the line right under the tabs.
func TestViewButtonOnPanelTopBorder(t *testing.T) {
	lines := strings.Split(sized(t, New(nil), 80, 24).View(), "\n")
	top := lines[1+panelMarginY] // after the tab bar and the empty margin rows
	if !strings.HasPrefix(top, "┌─") || !strings.HasSuffix(strings.TrimRight(top, " "), "┐") {
		t.Errorf("line under the tabs and margin %q, want the panel's top border", top)
	}
	if !strings.Contains(top, "refresh: 5s") {
		t.Errorf("top border %q is missing refresh: 5s", top)
	}
}

// The top border with buttons is exactly as wide as the rest of the box.
func TestWithButtonsKeepsWidth(t *testing.T) {
	box := Panel.Width(40).Height(3).Render("x")
	row := buttonRow([]button{{label: "refresh: 5s", on: true}}, -1)
	got := withButtons(box, row, "", lipgloss.Width(box))
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
	row := buttonRow([]button{{label: "refresh: 5s", on: true}}, -1)
	if got := withButtons(box, row, "", lipgloss.Width(box)); got != box {
		t.Errorf("box changed though the buttons don't fit:\n%s", got)
	}
}

// ---------- focus on the buttons ----------

// onScreen is testModel with a terminal size, so the buttons are shown.
func onScreen(t *testing.T) Model {
	return sized(t, testModel(), 80, 24)
}

func TestFocusDownGoesTabsButtonsList(t *testing.T) {
	m := press(t, onScreen(t), down)
	if m.focus != focusButtons {
		t.Fatalf("after ↓ from tabs: focus = %d, want buttons", m.focus)
	}
	m = press(t, m, down)
	if m.focus != focusList || m.cursor != 0 {
		t.Fatalf("after ↓ from buttons: focus = %d cursor = %d, want list row 0", m.focus, m.cursor)
	}
}

func TestFocusUpGoesListButtonsTabs(t *testing.T) {
	m := press(t, onScreen(t), down, down) // on the list, row 0
	m = press(t, m, up)
	if m.focus != focusButtons {
		t.Fatalf("after ↑ from the top row: focus = %d, want buttons", m.focus)
	}
	m = press(t, m, up)
	if m.focus != focusTabs {
		t.Fatalf("after ↑ from buttons: focus = %d, want tabs", m.focus)
	}
}

func TestTabKeyStepsThroughButtons(t *testing.T) {
	m := onScreen(t)
	for _, want := range []focus{focusButtons, focusList, focusTabs} {
		m = press(t, m, tabKey)
		if m.focus != want {
			t.Fatalf("focus = %d, want %d", m.focus, want)
		}
	}
}

// On the buttons ← → move between buttons, they don't switch tabs.
func TestArrowsOnButtonsDontSwitchTab(t *testing.T) {
	m := press(t, onScreen(t), down, right, left)
	if m.active != 0 {
		t.Errorf("active tab = %d, want 0 (unchanged)", m.active)
	}
	if m.button != 0 {
		t.Errorf("button = %d, want 0 (→ then ←)", m.button)
	}
}

// → and ← stop at the last and first button instead of wrapping.
func TestButtonCursorStopsAtEnds(t *testing.T) {
	m := press(t, onScreen(t), down, right, right, right)
	if last := len(m.panelButtons()) - 1; m.button != last {
		t.Errorf("button = %d after many →, want the last one (%d)", m.button, last)
	}
	m = press(t, m, left, left, left)
	if m.button != 0 {
		t.Errorf("button = %d after many ←, want 0", m.button)
	}
}

func TestEnterAndSpacePressRefreshButton(t *testing.T) {
	for _, key := range []tea.KeyMsg{enterKey, spaceKey} {
		t.Run(key.String(), func(t *testing.T) {
			m := press(t, onScreen(t), down, right, key) // buttons, → to refresh, press
			if m.refreshEvery != refreshSlow {
				t.Errorf("refreshEvery = %s, want 10s after pressing the button", m.refreshEvery)
			}
		})
	}
}

func TestEnterElsewhereDoesNothing(t *testing.T) {
	m := press(t, onScreen(t), enterKey) // focus on tabs
	if m.refreshEvery != refreshNormal {
		t.Errorf("refreshEvery = %s, want 5s (unchanged)", m.refreshEvery)
	}
}

func TestFocusedButton(t *testing.T) {
	m := onScreen(t)
	if got := m.focusedButton(); got != -1 {
		t.Errorf("on tabs: focusedButton = %d, want -1", got)
	}
	if got := press(t, m, down).focusedButton(); got != 0 {
		t.Errorf("on buttons: focusedButton = %d, want 0", got)
	}
}

// When the buttons don't fit, the cursor skips them.
func TestFocusSkipsHiddenButtons(t *testing.T) {
	m := press(t, sized(t, testModel(), 12, 24), down)
	if m.focus != focusList {
		t.Errorf("focus = %d, want list (buttons are hidden at 12 columns)", m.focus)
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

// Key presses for the refresh button.
var (
	rateKey  = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(KeyRefreshRate)}
	enterKey = tea.KeyMsg{Type: tea.KeyEnter}
	spaceKey = tea.KeyMsg{Type: tea.KeySpace, Runes: []rune(" ")}
)

// ---------- last update time in the top border's right corner ----------

// loadedAt is a model whose Nodes tab loaded at 14:30:05, on an 80×24 screen.
func loadedAt(t *testing.T) Model {
	t.Helper()
	at := time.Date(2026, 11, 12, 14, 30, 5, 0, time.Local)
	return press(t, onScreen(t), itemsLoadedMsg{tab: "Nodes", rows: []string{"n1"}, at: at})
}

// topBorder is the panel's top border line of the rendered screen.
func topBorder(t *testing.T, m Model) string {
	t.Helper()
	return strings.Split(m.View(), "\n")[1+panelMarginY]
}

func TestLastUpdated(t *testing.T) {
	if got := loadedAt(t).lastUpdated(); got != "updated 14:30:05" {
		t.Errorf("lastUpdated = %q, want %q", got, "updated 14:30:05")
	}
}

func TestLastUpdatedEmptyBeforeFirstLoad(t *testing.T) {
	if got := onScreen(t).lastUpdated(); got != "" {
		t.Errorf("lastUpdated = %q, want empty before the first load", got)
	}
}

func TestLastUpdatedEmptyWhenOff(t *testing.T) {
	m := loadedAt(t)
	m.refreshEvery = refreshOff
	if got := m.lastUpdated(); got != "" {
		t.Errorf("lastUpdated = %q, want empty while refresh is off", got)
	}
}

// The time belongs to the active tab: Pods has never loaded.
func TestLastUpdatedIsForActiveTab(t *testing.T) {
	m := loadedAt(t)
	m.active = 1
	if got := m.lastUpdated(); got != "" {
		t.Errorf("on Pods: lastUpdated = %q, want empty", got)
	}
}

// The time is at the right end of the top border: "updated 14:30:05 ─┐".
func TestViewShowsUpdatedInRightCorner(t *testing.T) {
	top := strings.TrimRight(topBorder(t, loadedAt(t)), " ")
	if !strings.HasSuffix(top, "updated 14:30:05 ─┐") {
		t.Errorf("top border %q, want it to end with \"updated 14:30:05 ─┐\"", top)
	}
	// The refresh button is still on the left.
	if !strings.HasPrefix(top, "┌─") || !strings.Contains(top, "refresh: 5s") {
		t.Errorf("top border %q, want the refresh button on the left", top)
	}
	if strings.Index(top, "refresh") > strings.Index(top, "updated") {
		t.Errorf("top border %q, want refresh left of updated", top)
	}
}

func TestViewHidesUpdatedWhenOff(t *testing.T) {
	m := press(t, loadedAt(t), rateKey, rateKey) // 5s → 10s → off
	if top := topBorder(t, m); strings.Contains(top, "updated") {
		t.Errorf("top border %q shows the time while refresh is off", top)
	}
}

// The top border with the time is exactly as wide as the rest of the box.
func TestWithButtonsAndTimeKeepsWidth(t *testing.T) {
	box := Panel.Width(60).Height(3).Render("x")
	row := buttonRow([]button{{label: "refresh: 5s", on: true}}, -1)
	got := withButtons(box, row, "updated 14:30:05", lipgloss.Width(box))
	for i, line := range strings.Split(got, "\n") {
		if w := lipgloss.Width(line); w != lipgloss.Width(box) {
			t.Errorf("line %d is %d wide, want %d: %q", i, w, lipgloss.Width(box), line)
		}
	}
}

// When there is room for the button but not the time, the time is dropped
// and the button stays.
func TestWithButtonsDropsTimeWhenNarrow(t *testing.T) {
	row := buttonRow([]button{{label: "refresh: 5s", on: true}}, -1)
	box := Panel.Width(lipgloss.Width(row) + 4).Height(1).Render("x") // room for the button only
	got := withButtons(box, row, "updated 14:30:05", lipgloss.Width(box))
	top := strings.Split(got, "\n")[0]
	if strings.Contains(top, "updated") {
		t.Errorf("top border %q has the time though it doesn't fit", top)
	}
	if !strings.Contains(top, "refresh: 5s") {
		t.Errorf("top border %q lost the button", top)
	}
	if lipgloss.Width(top) != lipgloss.Width(box) {
		t.Errorf("top border is %d wide, want %d", lipgloss.Width(top), lipgloss.Width(box))
	}
}
