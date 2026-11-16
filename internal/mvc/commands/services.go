package commands

import (
	"context"
	"strings"
	"time"

	k8s "k8s-tui/internal/kubernetes"

	tea "github.com/charmbracelet/bubbletea"
	corev1 "k8s.io/api/core/v1"
)

// Commands for the Services tab: the same as for Pods and Deployments
// where a service can do the same (Describe, Show, Edit, Label, Delete,
// and making a new one), and Endpoints and Port-forward, which only a
// service has. The cluster calls are in internal/kubernetes/services.go
// and terminal.go.

// LoadServices calls the API for the services (like `kubectl get services
// --all-namespaces`, or with wide, `-o wide`).
func LoadServices(c *k8s.Client, wide bool, namespace string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), APITimeout)
		defer cancel()

		services, err := c.ListServices(ctx, namespace) // "": all namespaces
		now := time.Now()
		rows := serviceRows(services, wide, namespace == AllNamespaces, now)
		return ItemsLoadedMsg{Tab: TabServices, Rows: rows, Err: err, At: now, Wide: wide, Namespace: namespace}
	}
}

// serviceRows turns each service into one line, with its columns lined
// up, as `kubectl get services --all-namespaces` shows them:
//
//	NAMESPACE  NAME  TYPE  CLUSTER-IP  EXTERNAL-IP  PORT(S)  AGE
//
// With withNamespace false the NAMESPACE column is left out. With wide,
// SELECTOR comes after. now is the time the ages are counted to.
func serviceRows(services []k8s.Service, wide, withNamespace bool, now time.Time) []string {
	cells := make([][]string, 0, len(services))
	for _, s := range services {
		row := []string{s.Name, s.Type, s.ClusterIP, s.ExternalIP, s.Ports, age(s.CreatedAt, now)}
		if withNamespace {
			row = append([]string{s.Namespace}, row...)
		}
		if wide {
			row = append(row, s.Selector)
		}
		cells = append(cells, row)
	}
	return AlignColumns(cells)
}

// ServiceExternalIP is the Services tab's EXTERNAL-IP column: NAME TYPE
// CLUSTER-IP EXTERNAL-IP. A load balancer still waiting for its address,
// "<pending>", is yellow.
var ServiceExternalIP = StatusColumn{Column: 3, Level: func(ip string) Level {
	if ip == "<pending>" {
		return LevelWarn
	}
	return LevelNone
}}

// ServiceActions are the actions of the Services tab: enter on a service
// opens them. The keys are those of Pods and Deployments for the same action.
func ServiceActions() []Action {
	return []Action{
		{
			Key: "d", Name: "Describe", About: "everything about the service, like kubectl describe",
			Flags:   []Flag{{Name: flagNoEvents}},
			Command: "kubectl describe service %s",
			Run: func(c *k8s.Client, target, _ string, flags []Flag) tea.Cmd {
				namespace, name := SplitTarget(target)
				return Read("Describe", target, func(ctx context.Context) (string, error) {
					return c.DescribeService(ctx, namespace, name, !Ticked(flags, flagNoEvents))
				})
			},
		},
		{
			Key: "s", Name: "Show", About: "the service's labels, annotations and selector",
			Command: "kubectl get service %s --show-labels",
			Run: func(c *k8s.Client, target, _ string, _ []Flag) tea.Cmd {
				namespace, name := SplitTarget(target)
				return Read("Show", target, func(ctx context.Context) (string, error) {
					labels, annotations, selector, err := c.ServiceLabelsAndAnnotations(ctx, namespace, name)
					return SectionsText(Section{"Labels", labels}, Section{"Annotations", annotations}, Section{"Selector (the pods it sends traffic to)", selector}), err
				})
			},
		},
		{
			Key: "p", Name: "Endpoints", About: "where it sends traffic: the address of each pod behind it, ready or not",
			Command: "kubectl get endpointslices -l kubernetes.io/service-name=%s",
			Run: func(c *k8s.Client, target, _ string, _ []Flag) tea.Cmd {
				namespace, name := SplitTarget(target)
				return Read("Endpoints", target, func(ctx context.Context) (string, error) {
					endpoints, err := c.ServiceEndpoints(ctx, namespace, name)
					return SectionsText(Section{"Endpoints", endpoints}), err
				})
			},
		},
		{
			Key: "f", Name: "Port-forward", About: "reach it from this computer: 8080:80 makes localhost:8080 its port 80; ctrl+c stops",
			Input:   "ports (LOCAL:REMOTE)",
			Command: "kubectl port-forward service/%s",
			Run: func(c *k8s.Client, target, ports string, _ []Flag) tea.Cmd {
				namespace, name := SplitTarget(target)
				cmd, err := c.PortForwardService(namespace, name, ports)
				return InTerminal("Port-forward", target, "Port-forward to "+target+" stopped", false, cmd, err)
			},
		},
		{
			Key: "e", Name: "Edit", About: "the service's YAML in your editor",
			Command: "kubectl edit service %s",
			Run: func(c *k8s.Client, target, _ string, _ []Flag) tea.Cmd {
				namespace, name := SplitTarget(target)
				cmd, err := c.EditService(namespace, name)
				return InTerminal("Edit", target, "Edit "+target+": editor closed", false, cmd, err)
			},
		},
		{
			Key: "l", Name: "Label", About: "set labels (key=value) or remove them (key-); its selector doesn't change",
			Changes: true,
			Input:   "labels",
			Flags:   []Flag{{Name: FlagOverwrite}, {Name: FlagDryRun}},
			Command: "kubectl label service %s",
			Run: func(c *k8s.Client, target, input string, flags []Flag) tea.Cmd {
				namespace, name := SplitTarget(target)
				return Change("Label", target, flags, func(ctx context.Context) (string, error) {
					ch, err := c.LabelService(ctx, namespace, name, strings.Fields(input), Ticked(flags, FlagOverwrite), Ticked(flags, FlagDryRun))
					return ChangeText("Labels", ch.Before, ch.After), err
				})
			},
		},
		{
			Key: "r", Name: "Delete", About: "remove the service; its pods keep running",
			Changes: true,
			Flags:   []Flag{{Name: FlagDryRun}},
			Command: "kubectl delete service %s",
			Run: func(c *k8s.Client, target, _ string, flags []Flag) tea.Cmd {
				namespace, name := SplitTarget(target)
				return Change("Delete", target, flags, func(ctx context.Context) (string, error) {
					return "", c.DeleteService(ctx, namespace, name, Ticked(flags, FlagDryRun))
				})
			},
		},
	}
}

// serviceTypes are the kinds of service the tab's own actions make, with
// the key that picks each.
var serviceTypes = []struct {
	key   string
	typ   corev1.ServiceType
	about string
}{
	{"c", corev1.ServiceTypeClusterIP, "reachable inside the cluster only"},
	{"n", corev1.ServiceTypeNodePort, "also on a port of every node"},
	{"b", corev1.ServiceTypeLoadBalancer, "also on an outside address, if the cluster can give one"},
}

// CreateServiceActions are the Services tab's own actions: one per type,
// to make a service like `kubectl create service clusterip NAME
// --tcp=80:8080 -n default`. It sends traffic to the pods labelled
// app=NAME, as those of `kubectl create deployment NAME` are.
func CreateServiceActions() []Action {
	actions := make([]Action, 0, len(serviceTypes))
	for _, st := range serviceTypes {
		actions = append(actions, Action{
			Key: st.key, Name: string(st.typ), About: st.about + "; for the pods labelled app=NAME",
			Changes:   true,
			Input:     "name and ports, e.g. web 80:8080",
			InputArgs: serviceArgs,
			Flags:     []Flag{{Name: FlagDryRun}},
			Command:   "kubectl create service " + strings.ToLower(string(st.typ)) + " " + InputHere + " -n %s",
			Run: func(c *k8s.Client, namespace, input string, flags []Flag) tea.Cmd {
				name, ports, _ := strings.Cut(strings.TrimSpace(input), " ")
				return Change("Create", namespace+"/"+name, flags, func(ctx context.Context) (string, error) {
					return "", c.CreateService(ctx, namespace, name, st.typ, strings.TrimSpace(ports), Ticked(flags, FlagDryRun))
				})
			},
		})
	}
	return actions
}

// serviceArgs writes "web 80:8080" as kubectl takes it: "web --tcp=80:8080".
func serviceArgs(input string) string {
	name, ports, ok := strings.Cut(strings.TrimSpace(input), " ")
	if !ok {
		return input
	}
	return name + " --tcp=" + strings.TrimSpace(ports)
}

// exposeAction is the Deployments action that makes a service for the
// deployment, like `kubectl expose deployment NAME --port=80
// --target-port=8080`.
func exposeAction() Action {
	return Action{
		Key: "p", Name: "Expose", About: "make a service for it (PORT or PORT:TARGETPORT, e.g. 80:8080), shown on Services",
		Changes:   true,
		Input:     "ports",
		InputArgs: exposeArgs,
		Flags:     []Flag{{Name: FlagDryRun}},
		Command:   "kubectl expose deployment %s",
		Run: func(c *k8s.Client, target, ports string, flags []Flag) tea.Cmd {
			namespace, name := SplitTarget(target)
			return Change("Expose", target, flags, func(ctx context.Context) (string, error) {
				return "", c.ExposeDeployment(ctx, namespace, name, strings.TrimSpace(ports), Ticked(flags, FlagDryRun))
			})
		},
	}
}

// exposeArgs writes "80:8080" as kubectl takes it: "--port=80 --target-port=8080".
func exposeArgs(input string) string {
	port, target, ok := strings.Cut(strings.TrimSpace(input), ":")
	if !ok {
		return "--port=" + port
	}
	return "--port=" + port + " --target-port=" + target
}
