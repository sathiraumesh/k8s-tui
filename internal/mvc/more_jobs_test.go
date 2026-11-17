package mvc

import (
	"testing"

	"k8s.io/client-go/kubernetes/fake"
)

func TestJobActionCommands(t *testing.T) {
	for k, want := range map[string]string{
		"o": "kubectl logs job/hello -n apps --all-containers --tail=500",
		"p": `kubectl patch job hello -n apps -p '{"spec":{"suspend":true}}'`,
		"u": `kubectl patch job hello -n apps -p '{"spec":{"suspend":false}}'`,
	} {
		m := openKindFromMore(t, fake.NewClientset(), "jobs   batch/v1   true   Job", "apps   hello   Complete   1/1   5s   3h")
		m = press(t, m, enterKey)
		for i, a := range m.actions() {
			if a.Key == k {
				m.overlayCursor = i
			}
		}
		if got := m.kubectlCommand(); got != want {
			t.Errorf("%s: %q, want %q", k, got, want)
		}
	}
}

func TestCreateJobCommand(t *testing.T) {
	m := openKindFromMore(t, fake.NewClientset(), "jobs   batch/v1   true   Job")
	m = typeText(t, press(t, m, key(KeyOwn), key("c")), "hello busybox echo hi")
	if m.kubectlCommand() != "kubectl create job hello --image=busybox -- echo hi -n default" {
		t.Errorf("command = %q", m.kubectlCommand())
	}
}
