package mvc

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ---------- the kubectl command for each tab ----------

func TestKubectlCommandPerTab(t *testing.T) {
	want := map[string]string{
		tabNodes:       "kubectl get nodes",
		tabPods:        "kubectl get pods --all-namespaces",
		tabDeployments: "kubectl get deployments --all-namespaces",
		tabServices:    "kubectl get services --all-namespaces",
		tabMore:        "kubectl api-resources",
	}
	m := New(nil)
	for i, tb := range m.tabs {
		m.active = i
		if got := m.kubectlCommand(); got != want[tb.name] {
			t.Errorf("%s: command = %q, want %q", tb.name, got, want[tb.name])
		}
	}
}

// Wide adds "-o wide", and only for the tab that is wide.
func TestKubectlCommandWide(t *testing.T) {
	m := press(t, New(nil), wideKey)
	if got := m.kubectlCommand(); got != "kubectl get nodes -o wide" {
		t.Errorf("Nodes wide: command = %q, want kubectl get nodes -o wide", got)
	}
	m = press(t, m, right) // Pods: not wide
	if got := m.kubectlCommand(); got != "kubectl get pods --all-namespaces" {
		t.Errorf("Pods: command = %q, want it without -o wide", got)
	}
}

// Every tab has a kubectl command.
func TestEveryTabHasCommand(t *testing.T) {
	for _, tb := range allTabs() {
		if !strings.HasPrefix(tb.command, "kubectl ") {
			t.Errorf("%s: command = %q, want a kubectl command", tb.name, tb.command)
		}
	}
}

// ---------- on screen ----------

// The command is on the left of the panel's bottom border: "└─ $ kubectl ...".
func TestViewShowsCommandOnBottomBorder(t *testing.T) {
	b := bottomBorder(t, sized(t, testModel(), 80, 15))
	if !strings.HasPrefix(b, "└─ $ kubectl get nodes ─") {
		t.Errorf("bottom border = %q, want it to start with \"└─ $ kubectl get nodes ─\"", b)
	}
}

func TestViewCommandFollowsTabAndWide(t *testing.T) {
	m := sized(t, testModel(), 100, 15)
	m = press(t, m, wideKey)
	if b := bottomBorder(t, m); !strings.Contains(b, "$ kubectl get nodes -o wide") {
		t.Errorf("bottom border = %q, want the wide command", b)
	}
	m = press(t, m, right)
	if b := bottomBorder(t, m); !strings.Contains(b, "$ kubectl get pods --all-namespaces") {
		t.Errorf("bottom border = %q on Pods, want the pods command", b)
	}
}

// The command and the scroll position share the bottom border.
func TestViewCommandAndScrollInfoTogether(t *testing.T) {
	b := strings.TrimRight(bottomBorder(t, inList(t, longList(t, 30))), " ")
	if !strings.HasPrefix(b, "└─ $ kubectl get nodes ─") || !strings.HasSuffix(b, " 1-9 of 30 ─┘") {
		t.Errorf("bottom border = %q, want the command on the left and 1-9 of 30 on the right", b)
	}
}

// On a narrow terminal the command is cut off and the scroll position
// dropped, but the border keeps its width.
func TestViewCommandCutWhenNarrow(t *testing.T) {
	m := inList(t, longList(t, 30))
	m = press(t, m, tea.WindowSizeMsg{Width: 24, Height: 15})
	b := bottomBorder(t, m)
	if w := lipgloss.Width(b); w != 24 {
		t.Errorf("bottom border is %d wide, want 24: %q", w, b)
	}
	if !strings.HasPrefix(b, "└─ $ kubectl") {
		t.Errorf("bottom border = %q, want the start of the command", b)
	}
	if strings.Contains(b, "of 30") {
		t.Errorf("bottom border = %q, want the scroll position dropped when there is no room", b)
	}
}
