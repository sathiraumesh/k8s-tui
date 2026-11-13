package mvc

import (
	"errors"
	"strings"
	"testing"
	"time"

	k8s "k8s-tui/internal/kubernetes"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

// readyNode builds a client-go node that reports Ready.
func readyNode(name string) *corev1.Node {
	return &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Status: corev1.NodeStatus{
			Conditions: []corev1.NodeCondition{{Type: corev1.NodeReady, Status: corev1.ConditionTrue}},
			NodeInfo:   corev1.NodeSystemInfo{KubeletVersion: "v1.37.2"},
		},
	}
}

func TestNodeRows(t *testing.T) {
	rows := nodeRows([]k8s.Node{
		{Name: "controlplane", Status: "Ready", Version: "v1.37.2"},
		{Name: "worker1", Status: "NotReady", Version: "v1.36.0"},
	}, false)
	if len(rows) != 2 {
		t.Fatalf("got %d rows, want 2", len(rows))
	}
	for i, want := range [][]string{
		{"controlplane", "Ready", "v1.37.2"},
		{"worker1", "NotReady", "v1.36.0"},
	} {
		for _, part := range want {
			if !strings.Contains(rows[i], part) {
				t.Errorf("row %d %q is missing %q", i, rows[i], part)
			}
		}
	}
}

// Columns line up: the status starts at the same place on every row.
func TestNodeRowsAligned(t *testing.T) {
	rows := nodeRows([]k8s.Node{
		{Name: "a", Status: "Ready", Version: "v1"},
		{Name: "a-much-longer-node-name", Status: "Ready", Version: "v1"},
	}, false)
	if strings.Index(rows[0], "Ready") != strings.Index(rows[1], "Ready") {
		t.Errorf("status columns not aligned:\n%q\n%q", rows[0], rows[1])
	}
}

func TestNodeRowsEmpty(t *testing.T) {
	if rows := nodeRows(nil, false); len(rows) != 0 {
		t.Errorf("got %v, want no rows", rows)
	}
}

func TestLoadNodes(t *testing.T) {
	client := k8s.NewClientFromClientset(fake.NewClientset(readyNode("worker1"), readyNode("controlplane")))

	msg, ok := loadNodes(client, false, "")().(itemsLoadedMsg)
	if !ok {
		t.Fatalf("loadNodes sent %T, want itemsLoadedMsg", msg)
	}
	if msg.err != nil {
		t.Fatalf("err = %v", msg.err)
	}
	if msg.tab != "Nodes" {
		t.Errorf("tab = %q, want Nodes", msg.tab)
	}
	if len(msg.rows) != 2 || !strings.HasPrefix(msg.rows[0], "controlplane") {
		t.Errorf("rows = %q, want controlplane first", msg.rows)
	}
}

func TestLoadNodesError(t *testing.T) {
	cs := fake.NewClientset()
	cs.PrependReactor("list", "nodes", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("connection refused")
	})

	msg := loadNodes(k8s.NewClientFromClientset(cs), false, "")().(itemsLoadedMsg)
	if msg.err == nil {
		t.Fatal("err = nil, want an error")
	}
	if msg.tab != "Nodes" {
		t.Errorf("tab = %q, want Nodes", msg.tab)
	}
}

func TestInitWithoutClientLoadsNothing(t *testing.T) {
	if cmd := New(nil).Init(); cmd != nil {
		t.Error("Init returned a command, want nil when there is no client")
	}
}

func TestInitLoadsNodes(t *testing.T) {
	client := k8s.NewClientFromClientset(fake.NewClientset(readyNode("worker1")))
	m := New(client)
	m.refreshEvery = time.Millisecond // so running the timer command doesn't wait 5s

	// Init sends a batch: load the nodes now + start the refresh timer.
	msgs := runBatch(t, m.Init())
	if !hasLoaded(msgs, "Nodes") {
		t.Errorf("Init sent %#v, want an itemsLoadedMsg for Nodes", msgs)
	}
	if !hasTick(msgs) {
		t.Errorf("Init sent %#v, want a refreshTickMsg (timer started)", msgs)
	}
}

func TestNodesStartAsLoading(t *testing.T) {
	m := New(nil)
	if m.tabs[0].name != tabNodes {
		t.Errorf("first tab = %q, want Nodes", m.tabs[0].name)
	}
	if rows := m.items["Nodes"]; len(rows) != 1 || rows[0] != "loading..." {
		t.Errorf("Nodes rows = %q, want [loading...]", rows)
	}
}

func TestUpdateStoresLoadedItems(t *testing.T) {
	m := press(t, New(nil), itemsLoadedMsg{tab: "Nodes", rows: []string{"n1", "n2"}})
	if got := m.items["Nodes"]; len(got) != 2 || got[0] != "n1" {
		t.Errorf("Nodes rows = %q, want [n1 n2]", got)
	}
	if m.err != nil {
		t.Errorf("err = %v, want nil", m.err)
	}
}

func TestUpdateKeepsOldRowsOnError(t *testing.T) {
	m := press(t, New(nil), itemsLoadedMsg{tab: "Nodes", err: errors.New("boom")})
	if m.err == nil || m.err.Error() != "boom" {
		t.Errorf("err = %v, want boom", m.err)
	}
	if got := m.items["Nodes"]; len(got) != 1 || got[0] != "loading..." {
		t.Errorf("Nodes rows = %q, want them unchanged", got)
	}
}

func TestUpdateClearsErrorOnSuccess(t *testing.T) {
	m := press(t, New(nil),
		itemsLoadedMsg{tab: "Nodes", err: errors.New("boom")},
		itemsLoadedMsg{tab: "Nodes", rows: []string{"n1"}},
	)
	if m.err != nil {
		t.Errorf("err = %v, want nil after a successful load", m.err)
	}
}

func TestViewShowsError(t *testing.T) {
	m := sized(t, New(nil), 80, 24)
	m = press(t, m, itemsLoadedMsg{tab: "Nodes", err: errors.New("connection refused")})
	if out := m.View(); !strings.Contains(out, "error: connection refused") {
		t.Errorf("view does not show the error:\n%s", out)
	}
}

func TestViewShowsLoadedNodes(t *testing.T) {
	m := sized(t, New(nil), 80, 24)
	m = press(t, m, itemsLoadedMsg{tab: "Nodes", rows: []string{"controlplane  Ready", "worker1  Ready"}})
	out := m.View()
	if strings.Contains(out, "loading...") {
		t.Error("view still shows loading... after nodes arrived")
	}
	for _, row := range []string{"controlplane", "worker1"} {
		if !strings.Contains(out, row) {
			t.Errorf("view is missing %q", row)
		}
	}
}
