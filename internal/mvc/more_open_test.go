package mvc

import (
	"strings"
	"testing"
)

// moreWithRows is a model on More with its rows loaded from a fake cluster
// serving pods and deployments, the cursor on the list's first row.
func moreWithRows(t *testing.T) Model {
	t.Helper()
	m := sized(t, onMore(t, New(resourcesClient())), 120, 30)
	m = press(t, m, m.activeTab().load(m.client, false, "")())
	return press(t, m, down, down) // tabs → buttons → list
}

// moveTo puts the cursor on the row that starts with name.
func moveTo(t *testing.T, m Model, name string) Model {
	t.Helper()
	for i, row := range m.rows() {
		if strings.HasPrefix(row, name+" ") {
			m.cursor = i
			return m
		}
	}
	t.Fatalf("no row %q in %q", name, m.rows())
	return m
}

// enter on "pods" goes to the Pods tab, its list, and loads it.
func TestMoreOpensPodsTab(t *testing.T) {
	m := moveTo(t, moreWithRows(t), "pods")
	if m.focus != focusList {
		t.Fatalf("focus = %d, want the list", m.focus)
	}
	next, cmd := m.Update(enterKey)
	m = next.(Model)
	if m.activeName() != tabPods || m.focus != focusList || m.cursor != 0 || m.overlay != overlayNone {
		t.Fatalf("active = %q, focus = %d, cursor = %d, overlay = %d", m.activeName(), m.focus, m.cursor, m.overlay)
	}
	if !hasLoaded(runBatch(t, cmd), tabPods) {
		t.Error("Pods wasn't loaded")
	}
}

func TestMoreOpensDeploymentsTab(t *testing.T) {
	m := press(t, moveTo(t, moreWithRows(t), "deployments"), enterKey)
	if m.activeName() != tabDeployments {
		t.Errorf("active = %q, want Deployments", m.activeName())
	}
}

// A second click on the selected row opens its tab, as enter does.
func TestMoreClickOpensTab(t *testing.T) {
	m := moreWithRows(t)
	m = click(t, m, "pods ")
	m = click(t, m, "pods ")
	if m.activeName() != tabPods {
		t.Errorf("active = %q, want Pods", m.activeName())
	}
}

// A kind without a tab of its own says so, and stays on More.
func TestMoreKindWithoutTab(t *testing.T) {
	m := moreWithRows(t)
	m.items[tabMore] = []string{"configmaps   cm   v1   true   ConfigMap"}
	m.cursor = 0
	if m = press(t, m, enterKey); m.activeName() != tabMore || m.notice != "configmaps has no tab of its own yet" {
		t.Errorf("active = %q, notice = %q", m.activeName(), m.notice)
	}
}

func TestMoreHelpSaysEnterOpens(t *testing.T) {
	if help := moreWithRows(t).helpText(); !strings.Contains(help, "enter open") || strings.Contains(help, "enter actions") {
		t.Errorf("help = %q", help)
	}
}
