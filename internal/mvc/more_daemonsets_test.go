package mvc

import (
	"testing"

	"k8s.io/client-go/kubernetes/fake"
)

func TestDaemonSetActionCommands(t *testing.T) {
	for k, want := range map[string]string{
		"t": "kubectl rollout restart daemonsets.apps kindnet -n kube-system",
		"r": "kubectl delete daemonsets.apps kindnet -n kube-system",
	} {
		m := openKindFromMore(t, fake.NewClientset(), "daemonsets   ds   apps/v1   true   DaemonSet", "kube-system   kindnet   3   3   3   3   3   <none>   86d")
		if m = press(t, m, enterKey, key(k)); m.kubectlCommand() != want {
			t.Errorf("%s: %q, want %q", k, m.kubectlCommand(), want)
		}
	}
	m := openKindFromMore(t, fake.NewClientset(), "daemonsets   ds   apps/v1   true   DaemonSet", "kube-system   kindnet   3   3   3   3   3   <none>   86d")
	for _, a := range press(t, m, enterKey).actions() {
		if a.Key == "c" {
			t.Error("a daemonset has Scale")
		}
	}
}
