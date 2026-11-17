//go:build e2e

package mvc

// End-to-end tests of every feature against a real cluster, through the
// model, as if keys were pressed: each step's commands run against the
// cluster of the kubeconfig's current context. Run them with
//
//	go test -tags e2e ./internal/mvc -run E2E -v
//
// Everything is made in the namespace k8s-tui-e2e (deleted at the end) or,
// cluster-wide, with names starting k8s-tui-e2e. What would change things
// that were already there (cordoning a node, the default storage class) is
// only a server dry run.

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	k8s "k8s-tui/internal/kubernetes"
	"k8s-tui/internal/mvc/commands"

	tea "github.com/charmbracelet/bubbletea"
	appsv1 "k8s.io/api/apps/v1"
	certificatesv1 "k8s.io/api/certificates/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	e2eNS     = "k8s-tui-e2e"
	e2ePrefix = "k8s-tui-e2e"
)

// e2e is a test against the cluster: the app's client and model, and a
// clientset of its own to set things up and check them.
type e2e struct {
	t   *testing.T
	c   *k8s.Client
	cs  kubernetes.Interface
	dyn dynamic.Interface
	m   Model
}

// newE2E connects to the kubeconfig's current context, which must be a
// kind cluster, and makes the namespace the tests work in.
func newE2E(t *testing.T) *e2e {
	t.Helper()
	loader := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(clientcmd.NewDefaultClientConfigLoadingRules(), &clientcmd.ConfigOverrides{})
	raw, err := loader.RawConfig()
	if err != nil {
		t.Skip(err)
	}
	if !strings.HasPrefix(raw.CurrentContext, "kind-") {
		t.Skipf("the current context %q isn't a kind cluster", raw.CurrentContext)
	}
	config, err := loader.ClientConfig()
	if err != nil {
		t.Fatal(err)
	}
	c, err := k8s.NewClient()
	if err != nil {
		t.Fatal(err)
	}
	e := &e2e{t: t, c: c, cs: kubernetes.NewForConfigOrDie(config), dyn: dynamic.NewForConfigOrDie(config)}
	e.m = New(c)
	e.m.refreshEvery = refreshOff // no timer: each step runs its own commands
	e.m = press(t, e.m, tea.WindowSizeMsg{Width: 200, Height: 50})
	return e
}

// settle runs cmd and what follows from it until nothing is left, as the
// program would. A command that needs the terminal (a shell, an editor)
// isn't run: its name is returned.
func (e *e2e) settle(cmd tea.Cmd) (terminal []string) {
	queue := []tea.Cmd{cmd}
	for steps := 0; len(queue) > 0 && steps < 100; steps++ {
		c := queue[0]
		queue = queue[1:]
		if c == nil {
			continue
		}
		msg := c()
		switch msg := msg.(type) {
		case nil:
		case tea.BatchMsg:
			queue = append(queue, msg...)
		default:
			if t := fmt.Sprintf("%T", msg); strings.Contains(t, "exec") {
				terminal = append(terminal, t)
				continue
			}
			next, more := e.m.Update(msg)
			e.m = next.(Model)
			queue = append(queue, more)
		}
	}
	return terminal
}

// press presses keys, running what each starts.
func (e *e2e) press(keys ...tea.Msg) {
	for _, k := range keys {
		next, cmd := e.m.Update(k)
		e.m = next.(Model)
		e.settle(cmd)
	}
}

// tab goes to the tab called name, on its list, loaded.
func (e *e2e) tab(name string) {
	e.t.Helper()
	e.m = e.m.closeOverlay()
	if e.m.inKind() {
		e.m = e.m.closeKind()
	}
	for i, t := range e.m.tabs {
		if t.name == name {
			e.m.active = i
		}
	}
	e.m.cursor, e.m.offset = 0, 0
	e.m.focus = focusList
	e.settle(e.m.loadActive())
}

// more opens the view of the More row of plural (and apiVersion, if not "").
func (e *e2e) more(plural, apiVersion string) {
	e.t.Helper()
	e.tab(tabMore)
	for i, row := range e.m.rows() {
		name, version, _, ok := commands.ResourceOf(row)
		if ok && name == plural && (apiVersion == "" || version == apiVersion) {
			e.m.cursor = i
			e.press(enterKey)
			if !e.m.inKind() && e.m.activeName() == tabMore {
				e.t.Fatalf("enter on %q opened nothing: %s", row, e.m.notice)
			}
			return
		}
	}
	e.t.Fatalf("More has no row %s %s", plural, apiVersion)
}

// reload loads what is on screen again.
func (e *e2e) reload() { e.settle(e.m.loadActive()) }

// selectRow puts the cursor on the row with the field name, reloading
// until it is there (a minute at most).
func (e *e2e) selectRow(name string) {
	e.t.Helper()
	deadline := time.Now().Add(time.Minute)
	for {
		for i, row := range e.m.rows() {
			for _, f := range strings.Fields(row) {
				if f == name {
					e.m.cursor = i
					e.m.focus = focusList
					return
				}
			}
		}
		if time.Now().After(deadline) {
			e.t.Fatalf("no row %q in %s:\n%s", name, e.m.activeName(), strings.Join(e.m.rows(), "\n"))
		}
		time.Sleep(time.Second)
		e.reload()
	}
}

// do runs the action with key on the row the cursor is on (or, own, the
// view's own action), with input typed and the flags named ticked, and
// returns what the line under the panel then says. The page of text it
// opened, if any, is e.m.text.
func (e *e2e) do(own bool, key, input string, flags ...string) string {
	e.t.Helper()
	e.m.notice = ""
	if own {
		e.press(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(KeyOwn)})
	} else {
		e.press(enterKey)
	}
	if e.m.overlay != overlayActions {
		e.t.Fatalf("no action list on %s (overlay %d, notice %q)", e.m.activeName(), e.m.overlay, e.m.notice)
	}
	found := false
	for _, a := range e.m.actions() {
		found = found || a.Key == key
	}
	if !found {
		e.t.Fatalf("%s has no action %q", e.m.activeName(), key)
	}
	e.press(key2(key))
	if e.m.overlay == overlayContainers { // pick the first container
		e.press(enterKey)
	}
	if e.m.overlay != overlayConfirm {
		return e.m.notice // it ran at once (a read with nothing to set)
	}
	if input != "" {
		e.m.input = input
	}
	for i := range e.m.flags {
		for _, f := range flags {
			if e.m.flags[i].Name == f {
				e.m.flags[i].On = true
			}
		}
	}
	e.m.overlayCursor = e.m.fieldRows() + len(e.m.flags) // off the text field
	if e.m.chosen().Changes {
		e.press(key2(KeyYes))
	} else {
		e.press(enterKey)
	}
	return e.m.notice
}

// key2 is a key press of s.
func key2(s string) tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)} }

// text is the page of text on screen.
func (e *e2e) text() string { return strings.Join(e.m.text, "\n") }

// back goes back from a page to the list.
func (e *e2e) back() { e.m = e.m.closeOverlay() }

// expectDone checks an action's notice says it worked.
func (e *e2e) expectDone(what, notice string) {
	e.t.Helper()
	if !strings.HasSuffix(notice, ": done") && !strings.Contains(notice, "dry run passed") && !strings.HasPrefix(notice, "running:") {
		e.t.Errorf("%s: %q", what, notice)
	}
	if strings.Contains(notice, "failed") {
		e.t.Errorf("%s failed: %q", what, notice)
	}
}

// expectText checks a read action opened a page with want in it.
func (e *e2e) expectText(what, want string) {
	e.t.Helper()
	if e.m.overlay != overlayText || !strings.Contains(e.text(), want) {
		e.t.Errorf("%s: overlay %d, notice %q, page:\n%.600s", what, e.m.overlay, e.m.notice, e.text())
	}
	e.back()
}

// eventually waits up to timeout for ok.
func (e *e2e) eventually(what string, timeout time.Duration, ok func() bool) {
	e.t.Helper()
	deadline := time.Now().Add(timeout)
	for !ok() {
		if time.Now().After(deadline) {
			e.t.Fatalf("%s: not after %s", what, timeout)
		}
		time.Sleep(time.Second)
	}
}

// podRunning waits for the pod to run.
func (e *e2e) podRunning(name string) {
	e.t.Helper()
	e.eventually("pod "+name+" running", 2*time.Minute, func() bool {
		p, err := e.cs.CoreV1().Pods(e2eNS).Get(context.Background(), name, metav1.GetOptions{})
		return err == nil && p.Status.Phase == corev1.PodRunning
	})
}

// cleanup deletes what the tests made, also if they failed.
func (e *e2e) cleanup() {
	ctx := context.Background()
	bg := metav1.DeletePropagationBackground
	opts := metav1.DeleteOptions{PropagationPolicy: &bg}
	e.cs.CoreV1().Namespaces().Delete(ctx, e2eNS, opts)
	e.cs.CoreV1().PersistentVolumes().Delete(ctx, e2ePrefix+"-pv", opts)
	e.cs.SchedulingV1().PriorityClasses().Delete(ctx, e2ePrefix+"-pc", opts)
	e.cs.RbacV1().ClusterRoles().Delete(ctx, e2ePrefix+"-cr", opts)
	e.cs.RbacV1().ClusterRoleBindings().Delete(ctx, e2ePrefix+"-crb", opts)
	e.cs.NetworkingV1().IngressClasses().Delete(ctx, e2ePrefix+"-ic", opts)
	e.cs.CertificatesV1().CertificateSigningRequests().Delete(ctx, e2ePrefix+"-csr-a", opts)
	e.cs.CertificatesV1().CertificateSigningRequests().Delete(ctx, e2ePrefix+"-csr-d", opts)
	e.dyn.Resource(crdGVR).Delete(ctx, "widgets.e2e.k8s-tui.dev", opts)
	e.eventually("namespace "+e2eNS+" gone", 3*time.Minute, func() bool {
		_, err := e.cs.CoreV1().Namespaces().Get(ctx, e2eNS, metav1.GetOptions{})
		return err != nil
	})
}

var crdGVR = schema.GroupVersionResource{Group: "apiextensions.k8s.io", Version: "v1", Resource: "customresourcedefinitions"}

// TestE2E runs every feature, in order: the namespace first, then each tab
// and each kind of More.
func TestE2E(t *testing.T) {
	e := newE2E(t)
	e.cleanup() // what an earlier run may have left
	t.Cleanup(e.cleanup)

	t.Run("namespaces", e.namespaces)
	t.Run("nodes", e.nodes)
	t.Run("pods", e.pods)
	t.Run("deployments", e.deployments)
	t.Run("services", e.services)
	t.Run("storage", e.storage)
	t.Run("config", e.config)
	t.Run("workloads", e.workloads)
	t.Run("networking", e.networking)
	t.Run("policy", e.policy)
	t.Run("rbac", e.rbac)
	t.Run("reviews", e.reviews)
	t.Run("certificates", e.certificates)
	t.Run("crds", e.crds)
	t.Run("every kind lists, describes and shows its YAML", e.everyKind)
}

func (e *e2e) namespaces(t *testing.T) {
	e.t = t
	e.more("namespaces", "")
	e.expectDone("create namespace", e.do(true, "n", e2eNS))
	e.reload()
	e.selectRow(e2eNS)
	e.do(false, "u", "") // Use
	if e.m.namespace != e2eNS {
		t.Fatalf("Use: namespace %q, notice %q", e.m.namespace, e.m.notice)
	}
	e.do(false, "d", "")
	e.expectText("describe namespace", "Active")
}

func (e *e2e) nodes(t *testing.T) {
	e.t = t
	e.tab(tabNodes)
	e.selectRow("k8s-101-worker")
	e.do(false, "d", "")
	e.expectText("describe node", "Name:               k8s-101-worker")
	e.do(false, "s", "")
	e.expectText("show node", "kubernetes.io/hostname=k8s-101-worker")
	// Changes to a node that was there: dry runs only.
	e.expectDone("cordon", e.do(false, "c", "", commands.FlagDryRun))
	e.expectDone("uncordon", e.do(false, "u", "", commands.FlagDryRun))
	e.expectDone("label", e.do(false, "l", "e2e=yes", commands.FlagDryRun))
	e.back()
	e.expectDone("taint", e.do(false, "t", "e2e=yes:NoSchedule", commands.FlagDryRun))
	e.back()
	// wide
	e.press(key2(KeyWide))
	if !strings.Contains(e.m.rows()[0], "containerd://") {
		t.Errorf("wide rows: %q", e.m.rows()[0])
	}
	e.press(key2(KeyWide))
	// Shell on a node: kubectl debug, run without a terminal.
	cmd, err := e.c.NodeShell("k8s-101-worker", k8s.ShellImageBusybox, false)
	if err != nil {
		t.Fatal(err)
	}
	args := []string{"debug", "node/k8s-101-worker", "--image=busybox", "-n", e2eNS, "--attach", "--", "cat", "/host/etc/hostname"}
	if !strings.Contains(strings.Join(cmd.Args, " "), "debug node/k8s-101-worker -it --image=busybox") {
		t.Errorf("node shell command: %q", cmd.Args)
	}
	if out, err := exec.Command(cmd.Path, append(args, "--context="+e.c.Context())...).CombinedOutput(); err != nil || !strings.Contains(string(out), "k8s-101-worker") {
		t.Errorf("kubectl debug node: %v\n%s", err, out)
	}
}

func (e *e2e) pods(t *testing.T) {
	e.t = t
	ctx := context.Background()
	// A pod of two containers that write logs.
	logger := func(name string) corev1.Container {
		return corev1.Container{Name: name, Image: "busybox:latest", ImagePullPolicy: corev1.PullIfNotPresent,
			Command: []string{"sh", "-c", "while true; do echo " + name + "-says-hi; sleep 2; done"}}
	}
	_, err := e.cs.CoreV1().Pods(e2eNS).Create(ctx, &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "multi"},
		Spec:       corev1.PodSpec{Containers: []corev1.Container{logger("app"), logger("sidecar")}},
	}, metav1.CreateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	e.podRunning("multi")
	time.Sleep(3 * time.Second) // some logs

	e.tab(tabPods)
	if !strings.Contains(e.m.kubectlCommand(), "-n "+e2eNS) {
		t.Errorf("Pods command %q, want the namespace picked", e.m.kubectlCommand())
	}
	// + run: nginx
	e.expectDone("run pod", e.do(true, "n", "web"))
	e.podRunning("web")
	e.reload()
	e.selectRow("multi")
	e.do(false, "d", "")
	e.expectText("describe pod", "sidecar:")
	e.do(false, "o", "") // Logs: the picker, its first choice "all containers"
	e.expectText("logs", "==> sidecar <==")
	e.do(false, "s", "")
	e.expectText("show pod", "Labels:")
	e.expectDone("label pod", e.do(false, "l", "e2e=yes"))
	e.reload()
	e.selectRow("web")
	e.expectDone("set image", e.do(false, "i", "web=nginx:latest"))
	// Shell: kubectl exec, run without a terminal.
	cmd, err := e.c.PodShell(e2eNS, "multi", "sidecar", false)
	if err != nil {
		t.Fatal(err)
	}
	args := strings.Join(cmd.Args, " ")
	if !strings.Contains(args, "exec -it multi -n "+e2eNS+" -c sidecar") {
		t.Errorf("shell command: %q", cmd.Args)
	}
	out, err := exec.Command(cmd.Path, "exec", "multi", "-n", e2eNS, "-c", "sidecar", "--context="+e.c.Context(), "--", "sh", "-c", "echo shell-ok").CombinedOutput()
	if err != nil || !strings.Contains(string(out), "shell-ok") {
		t.Errorf("kubectl exec: %v\n%s", err, out)
	}
	e.expectDone("delete pod", e.do(false, "r", "", "--now"))
	e.eventually("web gone", time.Minute, func() bool {
		_, err := e.cs.CoreV1().Pods(e2eNS).Get(ctx, "web", metav1.GetOptions{})
		return err != nil
	})
}

func (e *e2e) deployments(t *testing.T) {
	e.t = t
	ctx := context.Background()
	e.tab(tabDeployments)
	e.expectDone("create deployment", e.do(true, "n", "shop"))
	e.reload()
	e.selectRow("shop")
	e.expectDone("scale", e.do(false, "c", "2"))
	e.eventually("2 ready", 2*time.Minute, func() bool {
		d, _ := e.cs.AppsV1().Deployments(e2eNS).Get(ctx, "shop", metav1.GetOptions{})
		return d != nil && d.Status.ReadyReplicas == 2
	})
	e.expectDone("restart", e.do(false, "t", ""))
	e.expectDone("set image", e.do(false, "i", "nginx=nginx:latest"))
	e.expectDone("label", e.do(false, "l", "e2e=yes"))
	e.do(false, "s", "")
	e.expectText("show", "e2e=yes")
	e.do(false, "d", "")
	e.expectText("describe", "Replicas:")
	e.expectDone("expose", e.do(false, "p", "80"))
	if _, err := e.cs.CoreV1().Services(e2eNS).Get(ctx, "shop", metav1.GetOptions{}); err != nil {
		t.Errorf("expose made no service: %v", err)
	}
}

func (e *e2e) services(t *testing.T) {
	e.t = t
	e.tab(tabServices)
	e.selectRow("shop")
	e.do(false, "d", "")
	e.expectText("describe", "Selector:")
	e.do(false, "s", "")
	e.expectText("show", "app=shop")
	time.Sleep(3 * time.Second) // the endpoint slices
	e.do(false, "p", "")
	e.expectText("endpoints", ":80/TCP   ready")
	e.expectDone("label", e.do(false, "l", "e2e=yes"))
	e.expectDone("create nodeport", e.do(true, "n", "api 80:8080"))
	// Port-forward, run without a terminal: localhost:18080 is nginx, once
	// the restart and new image of the deployment have rolled out.
	e.eventually("shop rolled out", 2*time.Minute, func() bool {
		d, _ := e.cs.AppsV1().Deployments(e2eNS).Get(context.Background(), "shop", metav1.GetOptions{})
		pods, _ := e.cs.CoreV1().Pods(e2eNS).List(context.Background(), metav1.ListOptions{LabelSelector: "app=shop"})
		return d != nil && d.Status.ObservedGeneration == d.Generation && d.Status.UpdatedReplicas == *d.Spec.Replicas &&
			d.Status.ReadyReplicas == *d.Spec.Replicas && pods != nil && len(pods.Items) == int(*d.Spec.Replicas)
	})
	cmd, err := e.c.PortForwardService(e2eNS, "shop", "18080:80")
	if err != nil {
		t.Fatal(err)
	}
	var forwardOut strings.Builder
	cmd.Stdout, cmd.Stderr = &forwardOut, &forwardOut
	defer func() {
		if t.Failed() {
			t.Logf("kubectl port-forward said:\n%s", forwardOut.String())
		}
	}()
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer cmd.Process.Kill()
	var body string
	e.eventually("port-forward answers", 30*time.Second, func() bool {
		resp, err := http.Get("http://localhost:18080")
		if err != nil {
			return false
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		body = string(b)
		return true
	})
	if !strings.Contains(body, "Welcome to nginx") {
		t.Errorf("port-forward: %.200s", body)
	}
	e.reload()
	e.selectRow("api")
	e.expectDone("delete service", e.do(false, "r", ""))
}

func (e *e2e) storage(t *testing.T) {
	e.t = t
	ctx := context.Background()
	e.more("persistentvolumes", "")
	e.expectDone("create hostPath pv", e.do(true, "h", e2ePrefix+"-pv 1Gi /tmp/k8s-tui-e2e"))
	e.reload()
	e.selectRow(e2ePrefix + "-pv")
	e.expectDone("reclaim policy", e.do(false, "p", "Delete"))
	e.expectDone("label pv", e.do(false, "l", "e2e=yes"))
	e.do(false, "d", "")
	e.expectText("describe pv", "/tmp/k8s-tui-e2e")
	e.do(false, "a", "") // Make available: refused, nothing to free
	e.m = e.m.closeOverlay()

	// A claim of no class binds to it.
	e.more("persistentvolumeclaims", "")
	e.expectDone("create claim", e.do(true, "c", "data 1Gi -"))
	e.eventually("claim bound", time.Minute, func() bool {
		c, _ := e.cs.CoreV1().PersistentVolumeClaims(e2eNS).Get(ctx, "data", metav1.GetOptions{})
		return c != nil && c.Status.Phase == corev1.ClaimBound
	})
	e.reload()
	e.selectRow("data")
	if !strings.Contains(e.m.rows()[e.m.cursor], "Bound") {
		t.Errorf("claim row: %q", e.m.rows()[e.m.cursor])
	}
	e.do(false, "u", "")
	e.expectText("used by", "Pods using it")
	if n := e.do(false, "c", "2Gi"); !strings.Contains(n, "failed") {
		t.Errorf("resize of a claim with no class: %q, want it refused", n)
	}
	e.m = e.m.closeOverlay()

	// Storage classes: make default only as a dry run (it would change the default).
	e.more("storageclasses", "")
	e.selectRow("standard")
	e.expectDone("make default (dry run)", e.do(false, "m", "", commands.FlagDryRun))
	e.m = e.m.closeOverlay()
}

func (e *e2e) config(t *testing.T) {
	e.t = t
	e.more("configmaps", "")
	e.expectDone("create configmap", e.do(true, "c", "cfg mode=fast level=3"))
	e.reload()
	e.selectRow("cfg")
	e.do(false, "a", "")
	e.expectText("data", "mode:\n  fast")
	// Edit, with an editor that adds a label.
	cmd, err := e.c.EditResource(k8s.ResourceType{Version: "v1", Resource: "configmaps", Kind: "ConfigMap", Namespaced: true}, e2eNS, "cfg")
	if err != nil {
		t.Fatal(err)
	}
	editor := t.TempDir() + "/editor.sh"
	os.WriteFile(editor, []byte("#!/bin/sh\nsed -i 's/^  name: cfg$/  name: cfg\\n  labels:\\n    edited: \"yes\"/' \"$1\"\n"), 0o755)
	cmd.Env = append(os.Environ(), "KUBE_EDITOR="+editor)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Errorf("kubectl edit: %v\n%s", err, out)
	}
	e.do(false, "s", "")
	e.expectText("edited label", "edited=yes")
	e.do(false, "y", "")
	e.expectText("yaml", "level: \"3\"")

	e.more("secrets", "")
	e.expectDone("create generic secret", e.do(true, "g", "login user=admin password=s3cret"))
	e.expectDone("create registry secret", e.do(true, "d", "regcred ghcr.io me pw"))
	e.reload()
	e.selectRow("login")
	e.do(false, "a", "")
	e.expectText("decode", "password:\n  s3cret")

	e.more("serviceaccounts", "")
	e.expectDone("create service account", e.do(true, "s", "deployer"))
	e.reload()
	e.selectRow("deployer")
	e.do(false, "t", "1h")
	e.expectText("token", "Expires")
	e.more("resourcequotas", "")
	e.expectDone("create quota", e.do(true, "c", "team pods=50 requests.cpu=8"))
	e.more("limitranges", "")
	e.expectDone("create limit range", e.do(true, "c", "defaults 500m 256Mi"))
}

func (e *e2e) workloads(t *testing.T) {
	e.t = t
	ctx := context.Background()
	sleeper := corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "sleeper"}},
		Spec: corev1.PodSpec{TerminationGracePeriodSeconds: new(int64), Containers: []corev1.Container{{
			Name: "sleep", Image: "busybox:latest", ImagePullPolicy: corev1.PullIfNotPresent, Command: []string{"sleep", "3600"}}}},
	}
	one := int32(1)
	sel := &metav1.LabelSelector{MatchLabels: map[string]string{"app": "sleeper"}}
	if _, err := e.cs.AppsV1().StatefulSets(e2eNS).Create(ctx, &appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: "db"},
		Spec: appsv1.StatefulSetSpec{Replicas: &one, Selector: sel, Template: sleeper}}, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	ds := sleeper.DeepCopy()
	ds.Labels = map[string]string{"app": "ds"}
	if _, err := e.cs.AppsV1().DaemonSets(e2eNS).Create(ctx, &appsv1.DaemonSet{ObjectMeta: metav1.ObjectMeta{Name: "agent"},
		Spec: appsv1.DaemonSetSpec{Selector: &metav1.LabelSelector{MatchLabels: ds.Labels}, Template: *ds}}, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}

	e.more("statefulsets", "")
	e.selectRow("db")
	e.expectDone("scale sts", e.do(false, "c", "2"))
	e.expectDone("restart sts", e.do(false, "t", ""))
	e.expectDone("set image sts", e.do(false, "i", "sleep=busybox:latest"))
	e.more("daemonsets", "")
	e.selectRow("agent")
	e.expectDone("restart ds", e.do(false, "t", ""))
	e.more("replicasets", "")
	rs, _ := e.cs.AppsV1().ReplicaSets(e2eNS).List(ctx, metav1.ListOptions{LabelSelector: "app=shop"})
	if len(rs.Items) == 0 {
		t.Fatal("the deployment shop has no replica set")
	}
	e.selectRow(rs.Items[0].Name)
	e.expectDone("scale rs (dry run)", e.do(false, "c", "1", commands.FlagDryRun))

	e.more("jobs", "")
	e.expectDone("create job", e.do(true, "c", "hello busybox echo hello-from-a-job"))
	e.eventually("job complete", 2*time.Minute, func() bool {
		j, _ := e.cs.BatchV1().Jobs(e2eNS).Get(ctx, "hello", metav1.GetOptions{})
		return j != nil && j.Status.Succeeded == 1
	})
	e.reload()
	e.selectRow("hello")
	if !strings.Contains(e.m.rows()[e.m.cursor], "Complete") {
		t.Errorf("job row: %q", e.m.rows()[e.m.cursor])
	}
	e.do(false, "o", "")
	e.expectText("job logs", "hello-from-a-job")
	e.expectDone("suspend job", e.do(false, "p", ""))
	e.expectDone("resume job", e.do(false, "u", ""))

	e.more("cronjobs", "")
	e.expectDone("create cronjob", e.do(true, "c", "tick busybox @hourly echo tick"))
	e.reload()
	e.selectRow("tick")
	e.expectDone("run now", e.do(false, "n", ""))
	jobs, _ := e.cs.BatchV1().Jobs(e2eNS).List(ctx, metav1.ListOptions{})
	ran := false
	for _, j := range jobs.Items {
		ran = ran || strings.HasPrefix(j.Name, "tick-manual-")
	}
	if !ran {
		t.Error("run now made no job")
	}
	e.expectDone("suspend cronjob", e.do(false, "p", ""))
	e.expectDone("resume cronjob", e.do(false, "u", ""))

	e.more("horizontalpodautoscalers", "")
	e.expectDone("autoscale", e.do(true, "a", "shop 1 3 80"))
	e.more("poddisruptionbudgets", "")
	e.expectDone("create pdb", e.do(true, "c", "shop app=shop 1"))

	// Bind a pod no scheduler picks up to a node by hand.
	if _, err := e.cs.CoreV1().Pods(e2eNS).Create(ctx, &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "unscheduled"},
		Spec: corev1.PodSpec{SchedulerName: "nobody", Containers: sleeper.Spec.Containers}}, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	e.more("bindings", "")
	e.expectDone("bind", e.do(true, "b", "unscheduled k8s-101-worker2"))
	if p, _ := e.cs.CoreV1().Pods(e2eNS).Get(ctx, "unscheduled", metav1.GetOptions{}); p.Spec.NodeName != "k8s-101-worker2" {
		t.Errorf("bound to %q", p.Spec.NodeName)
	}
}

func (e *e2e) networking(t *testing.T) {
	e.t = t
	e.more("ingresses", "")
	e.expectDone("create ingress", e.do(true, "c", "shop shop.e2e.test/*=shop:80"))
	e.more("networkpolicies", "")
	e.expectDone("deny all ingress", e.do(true, "d", "deny-all"))
	e.expectDone("allow same namespace", e.do(true, "s", "same-ns"))
	e.more("ingressclasses", "")
	if _, err := e.cs.NetworkingV1().IngressClasses().Create(context.Background(), &networkingClass, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	e.reload()
	e.selectRow(e2ePrefix + "-ic")
	e.expectDone("make ingress class default", e.do(false, "m", ""))
	e.do(false, "s", "")
	e.expectText("is default", "is-default-class=true")
	e.more("endpointslices", "")
	if rows := e.m.rows(); len(rows) == 0 || !strings.HasPrefix(rows[0], "shop-") {
		t.Errorf("endpoint slices of %s: %q", e2eNS, rows)
	}
	e.more("events", "v1")
	e.reload()
	if len(e.m.rows()) == 0 || e.m.rows()[0] == "loading..." {
		t.Errorf("no events in %s", e2eNS)
	}
}

func (e *e2e) policy(t *testing.T) {
	e.t = t
	e.more("priorityclasses", "")
	e.expectDone("create priority class", e.do(true, "c", e2ePrefix+"-pc 1000"))
	e.reload()
	e.selectRow(e2ePrefix + "-pc")
	e.expectDone("delete priority class", e.do(false, "r", ""))
}

func (e *e2e) rbac(t *testing.T) {
	e.t = t
	e.more("roles", "")
	e.expectDone("create role", e.do(true, "c", "reader get,list,watch pods,deployments"))
	e.more("rolebindings", "")
	e.expectDone("create role binding", e.do(true, "c", "readers role:reader serviceaccount:"+e2eNS+":deployer"))
	e.more("clusterroles", "")
	e.expectDone("create cluster role", e.do(true, "c", e2ePrefix+"-cr get,list nodes"))
	e.more("clusterrolebindings", "")
	e.expectDone("create cluster role binding", e.do(true, "c", e2ePrefix+"-crb clusterrole:"+e2ePrefix+"-cr user:e2e-alice"))
}

func (e *e2e) reviews(t *testing.T) {
	e.t = t
	e.more("selfsubjectreviews", "")
	e.do(true, "w", "")
	e.expectText("whoami", "Username: kubernetes-admin")
	e.more("selfsubjectaccessreviews", "")
	e.do(true, "c", "create deployments "+e2eNS)
	e.expectText("can i", "yes")
	e.more("selfsubjectrulesreviews", "")
	e.do(true, "m", e2eNS)
	e.expectText("my rules", "[*]")
	e.more("subjectaccessreviews", "")
	e.do(true, "c", "serviceaccount:"+e2eNS+":deployer list pods "+e2eNS)
	e.expectText("can they (role binding)", "yes")
	e.do(true, "c", "serviceaccount:"+e2eNS+":deployer delete pods "+e2eNS)
	e.expectText("can they not", "no")
	e.more("localsubjectaccessreviews", "")
	e.do(true, "c", "user:e2e-alice list pods")
	e.expectText("can they here", "no")
	// A token of the service account, checked.
	token, _, err := e.c.ServiceAccountToken(context.Background(), e2eNS, "deployer", "1h")
	if err != nil {
		t.Fatal(err)
	}
	e.more("tokenreviews", "")
	e.do(true, "c", token)
	e.expectText("token", "system:serviceaccount:"+e2eNS+":deployer")
}

func (e *e2e) certificates(t *testing.T) {
	e.t = t
	ctx := context.Background()
	for _, name := range []string{e2ePrefix + "-csr-a", e2ePrefix + "-csr-d"} {
		if _, err := e.cs.CertificatesV1().CertificateSigningRequests().Create(ctx, newCSR(t, name), metav1.CreateOptions{}); err != nil {
			t.Fatal(err)
		}
	}
	e.more("certificatesigningrequests", "")
	e.selectRow(e2ePrefix + "-csr-a")
	e.expectDone("approve", e.do(false, "a", ""))
	e.selectRow(e2ePrefix + "-csr-d")
	e.expectDone("deny", e.do(false, "n", ""))
	e.reload()
	e.selectRow(e2ePrefix + "-csr-d")
	if !strings.Contains(e.m.rows()[e.m.cursor], "Denied") {
		t.Errorf("denied row: %q", e.m.rows()[e.m.cursor])
	}
}

// newCSR is a request for a client certificate for the user e2e-alice.
func newCSR(t *testing.T, name string) *certificatesv1.CertificateSigningRequest {
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	der, err := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{Subject: pkix.Name{CommonName: "e2e-alice"}}, key)
	if err != nil {
		t.Fatal(err)
	}
	return &certificatesv1.CertificateSigningRequest{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: certificatesv1.CertificateSigningRequestSpec{
			Request:    pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: der}),
			SignerName: certificatesv1.KubeAPIServerClientSignerName,
			Usages:     []certificatesv1.KeyUsage{certificatesv1.UsageClientAuth},
		},
	}
}

func (e *e2e) crds(t *testing.T) {
	e.t = t
	ctx := context.Background()
	crd := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "apiextensions.k8s.io/v1", "kind": "CustomResourceDefinition",
		"metadata": map[string]any{"name": "widgets.e2e.k8s-tui.dev"},
		"spec": map[string]any{
			"group": "e2e.k8s-tui.dev", "scope": "Namespaced",
			"names": map[string]any{"plural": "widgets", "singular": "widget", "kind": "Widget"},
			"versions": []any{map[string]any{"name": "v1", "served": true, "storage": true,
				"schema": map[string]any{"openAPIV3Schema": map[string]any{"type": "object", "x-kubernetes-preserve-unknown-fields": true}}}},
		},
	}}
	if _, err := e.dyn.Resource(crdGVR).Create(ctx, crd, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	widgets := schema.GroupVersionResource{Group: "e2e.k8s-tui.dev", Version: "v1", Resource: "widgets"}
	e.eventually("CRD served", time.Minute, func() bool {
		_, err := e.dyn.Resource(widgets).Namespace(e2eNS).Create(ctx, &unstructured.Unstructured{Object: map[string]any{
			"apiVersion": "e2e.k8s-tui.dev/v1", "kind": "Widget", "metadata": map[string]any{"name": "gizmo"}}}, metav1.CreateOptions{})
		return err == nil
	})
	e.more("customresourcedefinitions", "")
	e.selectRow("widgets.e2e.k8s-tui.dev")
	e.do(false, "o", "") // Objects: a view of the widgets
	if e.m.activeName() != "widgets.e2e.k8s-tui.dev" {
		t.Fatalf("Objects opened %q (notice %q)", e.m.activeName(), e.m.notice)
	}
	e.selectRow("gizmo")
	e.do(false, "y", "")
	e.expectText("widget yaml", "kind: Widget")
	// And from More's own list, with the generic view.
	e.more("widgets", "e2e.k8s-tui.dev/v1")
	e.selectRow("gizmo")
	e.expectDone("delete widget", e.do(false, "r", ""))
}

// everyKind opens every kind the cluster lists in More, and on a row of
// each that has one, Describe and YAML.
func (e *e2e) everyKind(t *testing.T) {
	e.t = t
	e.tab(tabMore)
	kinds := append([]string{}, e.m.rows()...)
	for _, row := range kinds {
		name, apiVersion, _, ok := commands.ResourceOf(row)
		if !ok {
			continue
		}
		t.Run(name+"."+apiVersion, func(t *testing.T) {
			e.t = t
			e.more(name, apiVersion)
			if e.m.err != nil {
				t.Fatalf("list: %v", e.m.err)
			}
			rows := e.m.rows()
			if len(rows) == 0 || rows[0] == "loading..." || len(e.activeActions()) == 0 {
				return // nothing listed, or a review: nothing to describe
			}
			for _, k := range []string{"d", "y"} {
				if !e.hasAction(k) {
					continue
				}
				e.m.cursor = 0
				if n := e.do(false, k, ""); strings.Contains(n, "failed") {
					t.Errorf("%s on %q: %s", k, rows[0], n)
				}
				e.back()
			}
		})
	}
}

func (e *e2e) activeActions() []commands.Action { return e.m.activeTab().actions }

func (e *e2e) hasAction(key string) bool {
	for _, a := range e.activeActions() {
		if a.Key == key {
			return true
		}
	}
	return false
}

// networkingClass is an ingress class of the tests' own.
var networkingClass = networkingv1.IngressClass{
	ObjectMeta: metav1.ObjectMeta{Name: e2ePrefix + "-ic"},
	Spec:       networkingv1.IngressClassSpec{Controller: "example.com/e2e"},
}
