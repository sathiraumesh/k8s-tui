package mvc

import (
	"slices"
	"strings"
	"testing"
	"time"

	k8s "k8s-tui/internal/kubernetes"

	"k8s.io/client-go/kubernetes/fake"
)

// ---------- the table of tabs ----------

func TestAllTabsOrder(t *testing.T) {
	var names []string
	for _, tb := range allTabs() {
		names = append(names, tb.name)
	}
	want := []string{tabNodes, tabPods, tabDeployments, tabServices, tabMore}
	if !slices.Equal(names, want) {
		t.Errorf("tabs = %v, want %v", names, want)
	}
}

// Every tab has a loader and a unique name: nothing is special-cased.
func TestAllTabsComplete(t *testing.T) {
	seen := map[string]bool{}
	for _, tb := range allTabs() {
		if tb.name == "" || tb.load == nil {
			t.Errorf("tab %+v has no name or no loader", tb)
		}
		if seen[tb.name] {
			t.Errorf("tab %q appears twice", tb.name)
		}
		seen[tb.name] = true
	}
}

// Only More is loaded once; every other tab is auto refreshed.
func TestAutoRefreshPerTab(t *testing.T) {
	for _, tb := range allTabs() {
		if want := tb.name != tabMore; tb.autoRefresh != want {
			t.Errorf("%s: autoRefresh = %v, want %v", tb.name, tb.autoRefresh, want)
		}
	}
}

// Every tab starts as "loading..." until its rows arrive.
func TestEveryTabStartsLoading(t *testing.T) {
	m := New(nil)
	for _, tb := range m.tabs {
		if rows := m.items[tb.name]; len(rows) != 1 || rows[0] != "loading..." {
			t.Errorf("%s rows = %q, want [loading...]", tb.name, rows)
		}
	}
}

func TestActiveTab(t *testing.T) {
	m := New(nil)
	m.active = 2
	if m.activeName() != tabDeployments || m.activeTab().name != tabDeployments {
		t.Errorf("active tab = %q, want Deployments", m.activeName())
	}
}

// ---------- placeholder tabs ----------

func TestPlaceholderSendsItsRows(t *testing.T) {
	msg := placeholder("Things", "a", "b")(nil, false)().(itemsLoadedMsg)
	if msg.tab != "Things" || !slices.Equal(msg.rows, []string{"a", "b"}) || msg.err != nil {
		t.Errorf("msg = %#v, want rows a, b for Things", msg)
	}
	if time.Since(msg.at) > time.Minute {
		t.Errorf("at = %v, want the time it was sent", msg.at)
	}
}

// The placeholder keeps the wide setting, so Update doesn't drop its rows.
func TestPlaceholderKeepsWide(t *testing.T) {
	if msg := placeholder("Things", "a")(nil, true)().(itemsLoadedMsg); !msg.wide {
		t.Error("wide = false, want the wide setting it was given")
	}
}

// Every tab loads through the same path: with a cluster, each one's loader
// sends rows for that tab.
func TestEveryTabLoads(t *testing.T) {
	m := New(k8s.NewClientFromClientset(fake.NewClientset()))
	for i, tb := range m.tabs {
		m.active = i
		cmd := m.loadActive()
		if cmd == nil {
			t.Errorf("%s: loadActive = nil, want a loader", tb.name)
			continue
		}
		if msg, ok := cmd().(itemsLoadedMsg); !ok || msg.tab != tb.name {
			t.Errorf("%s: loader sent %#v, want rows for %s", tb.name, msg, tb.name)
		}
	}
}

// ---------- the last update time on every tab ----------

// Every tab, placeholder or not, shows when it was last loaded once its rows
// arrive: "updated" for auto refreshed tabs, "loaded" for More.
func TestEveryTabShowsLastUpdate(t *testing.T) {
	at := time.Date(2026, 11, 12, 14, 30, 5, 0, time.Local)
	for i, tb := range allTabs() {
		t.Run(tb.name, func(t *testing.T) {
			m := New(nil)
			m.active = i
			m = press(t, m, itemsLoadedMsg{tab: tb.name, rows: []string{"x"}, at: at})

			want := "updated 14:30:05"
			if !tb.autoRefresh {
				want = "loaded 14:30:05"
			}
			if got := m.lastUpdated(); got != want {
				t.Errorf("lastUpdated = %q, want %q", got, want)
			}

			top := strings.Split(sized(t, m, 100, 20).View(), "\n")[1+panelMarginY]
			if !strings.Contains(top, want) {
				t.Errorf("top border %q is missing %q", top, want)
			}
		})
	}
}

// A placeholder tab gets its time the same way a cluster tab does: from its
// loader's message.
func TestPlaceholderTabGetsUpdateTime(t *testing.T) {
	m := New(nil)
	m.active = 2 // Deployments
	msg := m.activeTab().load(nil, false)()
	m = press(t, m, msg)
	if !strings.HasPrefix(m.lastUpdated(), "updated ") {
		t.Errorf("lastUpdated on Deployments = %q, want an update time", m.lastUpdated())
	}
	if got := m.items[tabDeployments]; len(got) != 2 || got[0] != "nginx" {
		t.Errorf("Deployments rows = %q, want the placeholder rows", got)
	}
}
