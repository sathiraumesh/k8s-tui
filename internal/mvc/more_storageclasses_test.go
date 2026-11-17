package mvc

import (
	"testing"

	"k8s.io/client-go/kubernetes/fake"
)

func TestStorageClassMakeDefaultCommand(t *testing.T) {
	m := openKindFromMore(t, fake.NewClientset(), "storageclasses   sc   storage.k8s.io/v1   false   StorageClass", "fast   x   Delete   Immediate   false   3h")
	want := `kubectl patch storageclasses.storage.k8s.io fast -p '{"metadata":{"annotations":{"storageclass.kubernetes.io/is-default-class":"true"}}}'`
	if m = press(t, m, enterKey, key("m")); m.kubectlCommand() != want {
		t.Errorf("command = %q", m.kubectlCommand())
	}
}
