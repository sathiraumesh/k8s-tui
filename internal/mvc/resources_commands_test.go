package mvc

import (
	"errors"
	"strings"
	"testing"
	"time"

	k8s "k8s-tui/internal/kubernetes"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	fakediscovery "k8s.io/client-go/discovery/fake"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

// moreIndex is the index of the More tab.
func moreIndex(t *testing.T, m Model) int {
	t.Helper()
	for i, tab := range m.tabs {
		if tab == tabMore {
			return i
		}
	}
	t.Fatal("no More tab")
	return -1
}

// onMore returns m with the More tab active.
func onMore(t *testing.T, m Model) Model {
	t.Helper()
	m.active = moreIndex(t, m)
	return m
}

// resourcesClient is a client on a fake cluster serving pods and deployments.
func resourcesClient() *k8s.Client {
	cs := fake.NewClientset()
	cs.Discovery().(*fakediscovery.FakeDiscovery).Resources = []*metav1.APIResourceList{
		{GroupVersion: "v1", APIResources: []metav1.APIResource{
			{Name: "pods", ShortNames: []string{"po"}, Namespaced: true, Kind: "Pod",
				Verbs: []string{"get", "list"}, Categories: []string{"all"}},
		}},
		{GroupVersion: "apps/v1", APIResources: []metav1.APIResource{
			{Name: "deployments", ShortNames: []string{"deploy"}, Namespaced: true, Kind: "Deployment",
				Verbs: []string{"get", "list", "watch"}},
		}},
	}
	return k8s.NewClientFromClientset(cs)
}

var someResources = []k8s.Resource{
	{Name: "deployments", ShortNames: []string{"deploy"}, APIVersion: "apps/v1", Namespaced: true,
		Kind: "Deployment", Verbs: []string{"get", "list", "watch"}, Categories: []string{"all"}},
	{Name: "nodes", ShortNames: []string{"no"}, APIVersion: "v1", Kind: "Node", Verbs: []string{"get", "list"}},
	{Name: "customthings", APIVersion: "example.com/v1", Namespaced: true, Kind: "CustomThing"},
}

// ---------- rows ----------

func TestResourceRows(t *testing.T) {
	rows := resourceRows(someResources, false)
	for i, want := range [][]string{
		{"deployments", "deploy", "apps/v1", "true", "Deployment"},
		{"nodes", "no", "v1", "false", "Node"},
		{"customthings", "example.com/v1", "true", "CustomThing"},
	} {
		for _, cell := range want {
			if !strings.Contains(rows[i], cell) {
				t.Errorf("row %d %q is missing %q", i, rows[i], cell)
			}
		}
	}
	if strings.Contains(rows[0], "watch") {
		t.Errorf("narrow row %q has the verbs, want them only with wide", rows[0])
	}
}

func TestResourceRowsWide(t *testing.T) {
	rows := resourceRows(someResources, true)
	if !strings.Contains(rows[0], "get,list,watch") || !strings.Contains(rows[0], "all") {
		t.Errorf("wide row %q, want verbs get,list,watch and category all", rows[0])
	}
}

// Columns line up, even when a resource has no short names.
func TestResourceRowsAligned(t *testing.T) {
	rows := resourceRows(someResources, false)
	for _, kind := range []string{"Deployment", "Node", "CustomThing"} {
		if !strings.Contains(strings.Join(rows, "\n"), kind) {
			t.Fatalf("rows are missing %s", kind)
		}
	}
	col := strings.Index(rows[0], "Deployment")
	if strings.Index(rows[1], "Node") != col || strings.Index(rows[2], "CustomThing") != col {
		t.Errorf("KIND column not aligned:\n%s", strings.Join(rows, "\n"))
	}
}

// ---------- loading ----------

func TestLoadResources(t *testing.T) {
	msg := loadResources(resourcesClient(), false)().(itemsLoadedMsg)
	if msg.err != nil || msg.tab != tabMore || msg.wide {
		t.Fatalf("msg = %#v", msg)
	}
	if len(msg.rows) != 2 || !strings.HasPrefix(msg.rows[0], "deployments") || !strings.HasPrefix(msg.rows[1], "pods") {
		t.Errorf("rows = %q, want deployments then pods", msg.rows)
	}
}

func TestLoadResourcesWide(t *testing.T) {
	msg := loadResources(resourcesClient(), true)().(itemsLoadedMsg)
	if !msg.wide || !strings.Contains(msg.rows[0], "get,list,watch") {
		t.Errorf("msg = %#v, want wide rows with verbs", msg)
	}
}

func TestLoadResourcesError(t *testing.T) {
	cs := fake.NewClientset()
	cs.PrependReactor("get", "group", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("forbidden")
	})
	msg := loadResources(k8s.NewClientFromClientset(cs), false)().(itemsLoadedMsg)
	if msg.err == nil || msg.tab != tabMore {
		t.Errorf("msg = %#v, want an error for More", msg)
	}
}

// ---------- the More tab ----------

func TestMoreTabIsLast(t *testing.T) {
	m := New(nil)
	if m.tabs[len(m.tabs)-1] != tabMore {
		t.Errorf("tabs = %v, want More last", m.tabs)
	}
	if rows := m.items[tabMore]; len(rows) != 1 || rows[0] != "loading..." {
		t.Errorf("More rows = %q, want [loading...] before it loads", rows)
	}
}

func TestViewShowsMoreTab(t *testing.T) {
	first := strings.Split(sized(t, New(nil), 100, 24).View(), "\n")[0]
	if !strings.Contains(first, "More") {
		t.Errorf("tab bar %q is missing More", first)
	}
}

// Opening More loads the resource types.
func TestOpeningMoreLoadsIt(t *testing.T) {
	m := New(resourcesClient())
	m.active = moreIndex(t, m) - 1
	_, cmd := m.Update(right)
	if cmd == nil {
		t.Fatal("opening More returned no command, want a load")
	}
	if msg, ok := cmd().(itemsLoadedMsg); !ok || msg.tab != tabMore {
		t.Errorf("opening More sent %#v, want a load of More", msg)
	}
}

// Opening More again doesn't reload it: it is loaded once.
func TestOpeningMoreAgainDoesNotReload(t *testing.T) {
	m := New(resourcesClient())
	m = press(t, m, itemsLoadedMsg{tab: tabMore, rows: []string{"pods"}, at: time.Now()})
	m.active = moreIndex(t, m) - 1
	if _, cmd := m.Update(right); cmd != nil {
		t.Errorf("opening More again returned %#v, want no reload", cmd())
	}
}

// The timer doesn't reload More, but keeps running for the other tabs.
func TestTickDoesNotReloadMore(t *testing.T) {
	m := onMore(t, fastClient())
	_, cmd := m.Update(refreshTickMsg{gen: m.tickGen})
	msgs := runBatch(t, cmd)
	if hasLoaded(msgs, tabMore) {
		t.Errorf("tick sent %#v, want no reload of More", msgs)
	}
	if !hasTick(msgs) {
		t.Errorf("tick sent %#v, want the timer to keep running", msgs)
	}
}

// Wide on More reloads it with the wide columns.
func TestWideOnMoreReloads(t *testing.T) {
	m := onMore(t, New(resourcesClient()))
	_, cmd := m.Update(wideKey)
	if msg, ok := cmd().(itemsLoadedMsg); !ok || msg.tab != tabMore || !msg.wide {
		t.Errorf("w on More sent %#v, want a wide load of More", msg)
	}
}

// More has only the wide button: no refresh rate.
func TestMoreHasNoRefreshButton(t *testing.T) {
	buttons := onMore(t, New(nil)).panelButtons()
	if len(buttons) != 1 || buttons[0].name != buttonWide {
		t.Errorf("buttons = %+v, want only wide", buttons)
	}
}

// The button cursor stays on a button that exists when switching to More.
func TestButtonCursorKeptOnSwitchToMore(t *testing.T) {
	m := onScreen(t)
	m.active = moreIndex(t, m) - 1
	m = press(t, m, down, right) // on the refresh button (index 1)
	m.focus = focusList          // so → switches tab
	m = press(t, m, right)       // to More: only one button
	if m.button != 0 {
		t.Errorf("button = %d on More, want 0 (its only button)", m.button)
	}
}

// More shows when it was loaded, whatever the refresh rate.
func TestMoreShowsLoadedTime(t *testing.T) {
	at := time.Date(2026, 11, 12, 14, 30, 5, 0, time.Local)
	m := onMore(t, New(nil))
	m = press(t, m, itemsLoadedMsg{tab: tabMore, rows: []string{"pods"}, at: at})
	if got := m.lastUpdated(); got != "loaded 14:30:05" {
		t.Errorf("lastUpdated = %q, want loaded 14:30:05", got)
	}
	m.refreshEvery = refreshOff
	if got := m.lastUpdated(); got != "loaded 14:30:05" {
		t.Errorf("with refresh off: lastUpdated = %q, want loaded 14:30:05", got)
	}
}

// The view of More shows the rows, the wide button and the loaded time.
func TestViewMore(t *testing.T) {
	at := time.Date(2026, 11, 12, 14, 30, 5, 0, time.Local)
	m := sized(t, onMore(t, New(nil)), 100, 20)
	m = press(t, m, itemsLoadedMsg{tab: tabMore, rows: resourceRows(someResources, false), at: at})
	lines := strings.Split(m.View(), "\n")
	top := lines[1+panelMarginY]
	if !strings.Contains(top, "wide: off") || strings.Contains(top, "refresh") || !strings.Contains(top, "loaded 14:30:05") {
		t.Errorf("top border %q, want the wide button and the loaded time, no refresh", top)
	}
	out := strings.Join(lines, "\n")
	for _, want := range []string{"deployments", "apps/v1", "CustomThing"} {
		if !strings.Contains(out, want) {
			t.Errorf("view is missing %q", want)
		}
	}
}

// Turning refresh on from off reloads the tab, but not More: refreshActive
// (what the timer and the refresh rate use) skips it.
func TestRefreshActiveSkipsMore(t *testing.T) {
	m := New(resourcesClient())
	if m.refreshActive() == nil {
		t.Error("refreshActive on Nodes = nil, want a load")
	}
	if cmd := onMore(t, m).refreshActive(); cmd != nil {
		t.Error("refreshActive on More returned a load, want nil")
	}
}
