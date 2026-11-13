package mvc

import (
	"strings"
	"testing"

	k8s "k8s-tui/internal/kubernetes"

	tea "github.com/charmbracelet/bubbletea"
	"k8s.io/client-go/kubernetes/fake"
)

var wideKey = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(KeyWide)}

// ---------- the wide button ----------

// The buttons are wide, refresh, then search, as in systems-programing.
func TestButtonOrder(t *testing.T) {
	var names []string
	for _, b := range New(nil).panelButtons() {
		names = append(names, b.name)
	}
	if got := strings.Join(names, ","); got != "wide,refresh,search" {
		t.Errorf("buttons = %s, want wide,refresh,search", got)
	}
}

func TestWideButtonStartsOff(t *testing.T) {
	if b := buttonNamed(t, New(nil), buttonWide); b.label != "wide: off" || b.on {
		t.Errorf("wide button = %+v, want gray \"wide: off\"", b)
	}
}

func TestWideKeyTurnsWideOn(t *testing.T) {
	m := press(t, New(nil), wideKey)
	if !m.activeWide() {
		t.Fatal("activeWide = false after w, want true")
	}
	if b := buttonNamed(t, m, buttonWide); b.label != "wide: on" || !b.on {
		t.Errorf("wide button = %+v, want green \"wide: on\"", b)
	}
	if m = press(t, m, wideKey); m.activeWide() {
		t.Error("activeWide = true after w twice, want false")
	}
}

// The cursor lands on the wide button first; enter or space presses it.
func TestPressWideButton(t *testing.T) {
	for _, key := range []tea.KeyMsg{enterKey, spaceKey} {
		t.Run(key.String(), func(t *testing.T) {
			m := press(t, onScreen(t), down, key) // buttons: wide is first
			if !m.activeWide() {
				t.Error("activeWide = false after pressing the wide button, want true")
			}
			if m.refreshEvery != refreshNormal {
				t.Errorf("refreshEvery = %s, want 5s (the wide button must not touch it)", m.refreshEvery)
			}
		})
	}
}

// Each tab has its own wide setting.
func TestWideIsPerTab(t *testing.T) {
	m := press(t, New(nil), wideKey) // Nodes: on
	m = press(t, m, right)           // Pods
	if m.activeWide() {
		t.Error("Pods is wide, want off: only Nodes was switched")
	}
	if b := buttonNamed(t, m, buttonWide); b.label != "wide: off" {
		t.Errorf("on Pods: wide button = %q, want wide: off", b.label)
	}
	m = press(t, m, left) // back to Nodes
	if !m.activeWide() {
		t.Error("Nodes lost its wide setting after switching tabs")
	}
}

// Switching wide reloads the tab with the new columns.
func TestWideReloadsWithWideColumns(t *testing.T) {
	m := New(k8s.NewClientFromClientset(fake.NewClientset(readyNode("worker1"))))
	_, cmd := m.Update(wideKey)
	if cmd == nil {
		t.Fatal("w returned no command, want a reload")
	}
	msg, ok := cmd().(itemsLoadedMsg)
	if !ok || msg.tab != "Nodes" || !msg.wide {
		t.Errorf("reload sent %#v, want a wide load of Nodes", msg)
	}
}

// Auto refresh keeps loading with the tab's wide setting.
func TestRefreshKeepsWide(t *testing.T) {
	m := press(t, fastClient(), wideKey)
	msgs := runBatch(t, func() tea.Msg { _, c := m.Update(refreshTickMsg{gen: m.tickGen}); return c() })
	for _, msg := range msgs {
		if l, ok := msg.(itemsLoadedMsg); ok && !l.wide {
			t.Errorf("refresh loaded %#v without wide columns", l)
		}
	}
	if !hasLoaded(msgs, "Nodes") {
		t.Errorf("refresh sent %#v, want a reload of Nodes", msgs)
	}
}

// Rows that arrive with the wrong columns (wide was switched while they were
// on their way) are dropped; the reload the switch started brings the right ones.
func TestStaleWideRowsDropped(t *testing.T) {
	m := press(t, testModel(), wideKey) // wide is now on
	m = press(t, m, itemsLoadedMsg{tab: "Nodes", rows: []string{"narrow-row"}, wide: false})
	if strings.Contains(strings.Join(m.items["Nodes"], ","), "narrow-row") {
		t.Error("narrow rows replaced the list after wide was turned on")
	}
	m = press(t, m, itemsLoadedMsg{tab: "Nodes", rows: []string{"wide-row"}, wide: true})
	if got := m.items["Nodes"]; len(got) != 1 || got[0] != "wide-row" {
		t.Errorf("rows = %q, want [wide-row]", got)
	}
}

// ---------- wide rows ----------

var twoNodes = []k8s.Node{
	{Name: "controlplane", Status: "Ready", Version: "v1.37.2", InternalIP: "10.0.0.10",
		OSImage: "Ubuntu 24.04 LTS", KernelVersion: "6.8.0", Runtime: "containerd://2.0.0"},
	{Name: "worker1", Status: "Ready", Version: "v1.37.2", InternalIP: "10.0.0.11", ExternalIP: "203.0.113.11",
		OSImage: "Ubuntu 24.04 LTS", KernelVersion: "6.8.0", Runtime: "containerd://2.0.0"},
}

func TestNodeRowsNarrowHasNoWideColumns(t *testing.T) {
	for _, row := range nodeRows(twoNodes, false) {
		for _, wide := range []string{"10.0.0.", "Ubuntu", "6.8.0", "containerd"} {
			if strings.Contains(row, wide) {
				t.Errorf("narrow row %q has wide column %q", row, wide)
			}
		}
	}
}

func TestNodeRowsWideHasAllColumns(t *testing.T) {
	rows := nodeRows(twoNodes, true)
	for i, want := range [][]string{
		{"controlplane", "Ready", "v1.37.2", "10.0.0.10", "<none>", "Ubuntu 24.04 LTS", "6.8.0", "containerd://2.0.0"},
		{"worker1", "Ready", "v1.37.2", "10.0.0.11", "203.0.113.11", "Ubuntu 24.04 LTS", "6.8.0", "containerd://2.0.0"},
	} {
		for _, cell := range want {
			if !strings.Contains(rows[i], cell) {
				t.Errorf("wide row %d %q is missing %q", i, rows[i], cell)
			}
		}
	}
}

// Every wide column starts at the same place on every row.
func TestNodeRowsWideAligned(t *testing.T) {
	rows := nodeRows(twoNodes, true)
	for _, cell := range []string{"Ready", "v1.37.2", "Ubuntu", "6.8.0", "containerd"} {
		if strings.Index(rows[0], cell) != strings.Index(rows[1], cell) {
			t.Errorf("column %q not aligned:\n%q\n%q", cell, rows[0], rows[1])
		}
	}
}

func TestAlignColumns(t *testing.T) {
	got := alignColumns([][]string{{"a", "bb", "c"}, {"aaa", "b", "c"}})
	want := []string{"a     bb   c", "aaa   b    c"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("alignColumns =\n%q\nwant\n%q", got, want)
	}
}
