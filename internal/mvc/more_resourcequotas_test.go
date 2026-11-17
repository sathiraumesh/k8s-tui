package mvc

import (
	"testing"

	"k8s.io/client-go/kubernetes/fake"
)

func TestCreateQuotaCommand(t *testing.T) {
	m := openKindFromMore(t, fake.NewClientset(), "resourcequotas   quota   v1   true   ResourceQuota")
	m = typeText(t, press(t, m, key(KeyOwn), key("c")), "team pods=10 requests.cpu=4")
	if m.kubectlCommand() != "kubectl create quota team --hard=pods=10,requests.cpu=4 -n default" {
		t.Errorf("command = %q", m.kubectlCommand())
	}
}
