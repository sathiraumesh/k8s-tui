package mvc

import (
	"strings"
	"testing"

	"k8s.io/client-go/kubernetes/fake"
)

// A review can't be listed: its view says so, and + asks it.
func TestWhoAmIView(t *testing.T) {
	m := openKindFromMore(t, fake.NewClientset(), "selfsubjectreviews   authentication.k8s.io/v1   false   SelfSubjectReview")
	m = press(t, m, m.activeTab().load(m.client, false, "")())
	if rows := m.rows(); len(rows) != 1 || !strings.Contains(rows[0], "Press + to ask") {
		t.Fatalf("rows = %q", rows)
	}
	if m = press(t, m, enterKey); m.overlay != overlayNone {
		t.Error("enter on the line opened actions")
	}
	if m = press(t, m, key(KeyOwn)); m.kubectlCommand() != "kubectl auth whoami" {
		t.Errorf("command = %q", m.kubectlCommand())
	}
}
