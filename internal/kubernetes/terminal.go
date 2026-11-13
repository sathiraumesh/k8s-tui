package kubernetes

import (
	"fmt"
	"os/exec"
)

// Some things need the terminal to themselves, such as a shell on a node.
// client-go can't hand over a terminal, so these run the kubectl program,
// and the app steps aside while it runs (tea.ExecProcess, in the model).

// lookPath finds a program the way the shell does. It is a variable so
// tests can pretend kubectl is or isn't installed.
var lookPath = exec.LookPath

// ShellImages are the images NodeShell can open a shell with: busybox is
// small and starts fast, ubuntu has apt and the usual tools.
const (
	ShellImageBusybox = "busybox"
	ShellImageUbuntu  = "ubuntu"
)

// NodeShell returns the command that opens a shell on the node, like
// `kubectl debug node/NAME -it --image=busybox`: kubectl starts a debug pod
// on the node, with the node's own files under /host, and attaches the
// terminal to it.
//
// With sysadmin (`--profile=sysadmin`) the pod is privileged, so it can see
// and change the whole node (chroot /host).
//
// The client's kubeconfig context is named, so kubectl works on the same
// cluster as the app even if the current context was changed since.
//
// The debug pod is left behind when the shell ends, as with kubectl;
// it is called node-debugger-NAME-xxxxx.
func (c *Client) NodeShell(name, image string, sysadmin bool) (*exec.Cmd, error) {
	path, err := lookPath("kubectl")
	if err != nil {
		return nil, fmt.Errorf("a shell needs kubectl, which was not found: %w", err)
	}
	return exec.Command(path, NodeShellArgs(name, image, sysadmin, c.context)...), nil
}

// NodeShellArgs are the arguments NodeShell gives kubectl. kubeContext is
// left out when it is "".
func NodeShellArgs(name, image string, sysadmin bool, kubeContext string) []string {
	args := []string{"debug", "node/" + name, "-it", "--image=" + image}
	if sysadmin {
		args = append(args, "--profile=sysadmin")
	}
	if kubeContext != "" {
		args = append(args, "--context="+kubeContext)
	}
	return args
}
