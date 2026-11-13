package kubernetes

import (
	"fmt"
	"os/exec"
)

// Some things need the terminal to themselves, such as a shell on a node
// or an editor.
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
	return kubectl(NodeShellArgs(name, image, sysadmin, c.context))
}

// EditNode returns the command that opens the node in an editor, like
// `kubectl edit node NAME`: kubectl writes the node as YAML to a file, opens
// it in $KUBE_EDITOR or $EDITOR (vi if neither is set), and when the
// editor closes, saves what was changed. Closing without a change, or
// emptying the file, changes nothing.
//
// As for NodeShell, the client's context is named.
func (c *Client) EditNode(name string) (*exec.Cmd, error) {
	return kubectl(EditNodeArgs(name, c.context))
}

// EditNodeArgs are the arguments EditNode gives kubectl.
func EditNodeArgs(name, kubeContext string) []string {
	return withContext([]string{"edit", "node", name}, kubeContext)
}

// kubectl returns the command that runs kubectl with args.
func kubectl(args []string) (*exec.Cmd, error) {
	path, err := lookPath("kubectl")
	if err != nil {
		return nil, fmt.Errorf("this needs kubectl, which was not found: %w", err)
	}
	return exec.Command(path, args...), nil
}

// withContext adds --context to args, unless kubeContext is "".
func withContext(args []string, kubeContext string) []string {
	if kubeContext != "" {
		args = append(args, "--context="+kubeContext)
	}
	return args
}

// NodeShellArgs are the arguments NodeShell gives kubectl. kubeContext is
// left out when it is "".
func NodeShellArgs(name, image string, sysadmin bool, kubeContext string) []string {
	args := []string{"debug", "node/" + name, "-it", "--image=" + image}
	if sysadmin {
		args = append(args, "--profile=sysadmin")
	}
	return withContext(args, kubeContext)
}
