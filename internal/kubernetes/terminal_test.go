package kubernetes

import (
	"errors"
	"slices"
	"testing"

	"k8s.io/client-go/kubernetes/fake"
)

func TestNodeShellArgs(t *testing.T) {
	for _, tc := range []struct {
		image    string
		sysadmin bool
		context  string
		want     []string
	}{
		{ShellImageBusybox, false, "", []string{"debug", "node/worker1", "-it", "--image=busybox"}},
		{ShellImageUbuntu, true, "", []string{"debug", "node/worker1", "-it", "--image=ubuntu", "--profile=sysadmin"}},
		{ShellImageBusybox, false, "prod", []string{"debug", "node/worker1", "-it", "--image=busybox", "--context=prod"}},
	} {
		if got := NodeShellArgs("worker1", tc.image, tc.sysadmin, tc.context); !slices.Equal(got, tc.want) {
			t.Errorf("NodeShellArgs(%s, %v, %q) = %v, want %v", tc.image, tc.sysadmin, tc.context, got, tc.want)
		}
	}
}

// pretendKubectl makes lookPath find kubectl at path, or fail if path is "",
// until the test ends.
func pretendKubectl(t *testing.T, path string) {
	old := lookPath
	t.Cleanup(func() { lookPath = old })
	lookPath = func(string) (string, error) {
		if path == "" {
			return "", errors.New("not found")
		}
		return path, nil
	}
}

func TestNodeShell(t *testing.T) {
	pretendKubectl(t, "/bin/kubectl")
	c := NewClientFromClientset(fake.NewClientset())
	c.context = "lab"
	cmd, err := c.NodeShell("worker1", ShellImageUbuntu, false)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"/bin/kubectl", "debug", "node/worker1", "-it", "--image=ubuntu", "--context=lab"}
	if cmd.Path != "/bin/kubectl" || !slices.Equal(cmd.Args, want) {
		t.Errorf("command = %s %v, want %v", cmd.Path, cmd.Args, want)
	}
}

func TestNodeShellWithoutKubectl(t *testing.T) {
	pretendKubectl(t, "")
	if _, err := NewClientFromClientset(fake.NewClientset()).NodeShell("worker1", ShellImageBusybox, false); err == nil {
		t.Error("err = nil, want kubectl not found")
	}
}

func TestEditNode(t *testing.T) {
	pretendKubectl(t, "/bin/kubectl")
	c := NewClientFromClientset(fake.NewClientset())
	c.context = "lab"
	cmd, err := c.EditNode("worker1")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"/bin/kubectl", "edit", "node", "worker1", "--context=lab"}
	if !slices.Equal(cmd.Args, want) {
		t.Errorf("command = %v, want %v", cmd.Args, want)
	}
	if got := EditNodeArgs("worker1", ""); !slices.Equal(got, []string{"edit", "node", "worker1"}) {
		t.Errorf("without a context: %v", got)
	}
}

func TestEditNodeWithoutKubectl(t *testing.T) {
	pretendKubectl(t, "")
	if _, err := NewClientFromClientset(fake.NewClientset()).EditNode("worker1"); err == nil {
		t.Error("err = nil, want kubectl not found")
	}
}
