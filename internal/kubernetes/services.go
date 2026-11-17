package kubernetes

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/kubectl/pkg/describe"
)

// Service is one service, with what `kubectl get services -o wide` shows
// of it.
type Service struct {
	Namespace string
	Name      string

	Type       string // ClusterIP, NodePort, LoadBalancer or ExternalName
	ClusterIP  string // "None" for a headless service
	ExternalIP string // as kubectl shows it: "<none>", "<pending>", the IPs or the external name
	Ports      string // e.g. "80/TCP,443:30443/TCP", or "<none>"
	Selector   string // e.g. "app=web", or "<none>"

	CreatedAt time.Time
}

// ListServices returns the services in namespace, or in every namespace if
// it is "", sorted by namespace and name, like `kubectl get services
// --all-namespaces`.
func (c *Client) ListServices(ctx context.Context, namespace string) ([]Service, error) {
	list, err := c.clientset.CoreV1().Services(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	services := make([]Service, 0, len(list.Items))
	for _, s := range list.Items {
		services = append(services, toService(s))
	}
	sort.Slice(services, func(i, j int) bool {
		if services[i].Namespace != services[j].Namespace {
			return services[i].Namespace < services[j].Namespace
		}
		return services[i].Name < services[j].Name
	})
	return services, nil
}

// toService converts client-go's corev1.Service into our Service, with its
// columns as kubectl writes them.
func toService(s corev1.Service) Service {
	svc := Service{
		Namespace: s.Namespace,
		Name:      s.Name,
		Type:      string(s.Spec.Type),
		ClusterIP: orNoneText(s.Spec.ClusterIP),
		CreatedAt: s.CreationTimestamp.Time,
	}
	if svc.Type == "" {
		svc.Type = string(corev1.ServiceTypeClusterIP)
	}
	svc.ExternalIP = externalIP(s)

	var ports []string
	for _, p := range s.Spec.Ports {
		port := strconv.Itoa(int(p.Port))
		if p.NodePort != 0 {
			port += ":" + strconv.Itoa(int(p.NodePort))
		}
		ports = append(ports, port+"/"+string(p.Protocol))
	}
	svc.Ports = orNoneText(strings.Join(ports, ","))
	svc.Selector = orNoneText(strings.Join(pairs(s.Spec.Selector), ","))
	return svc
}

// externalIP is the EXTERNAL-IP column, worked out as kubectl does: an
// ExternalName service shows its name; a load balancer the addresses it
// was given, or "<pending>" until it has some; the others their external
// IPs, if any.
func externalIP(s corev1.Service) string {
	ips := append([]string{}, s.Spec.ExternalIPs...)
	switch s.Spec.Type {
	case corev1.ServiceTypeExternalName:
		return s.Spec.ExternalName
	case corev1.ServiceTypeLoadBalancer:
		for _, in := range s.Status.LoadBalancer.Ingress {
			if in.IP != "" {
				ips = append(ips, in.IP)
			} else if in.Hostname != "" {
				ips = append(ips, in.Hostname)
			}
		}
		if len(ips) == 0 {
			return "<pending>"
		}
	}
	return orNoneText(strings.Join(ips, ","))
}

// orNoneText is s, or "<none>" if it is empty, as kubectl shows it.
func orNoneText(s string) string {
	if s == "" {
		return "<none>"
	}
	return s
}

// DescribeService returns what `kubectl describe service NAME -n
// NAMESPACE` prints, using kubectl's own describer: type, IPs, ports,
// endpoints and events.
func (c *Client) DescribeService(_ context.Context, namespace, name string, showEvents bool) (string, error) {
	d := &describe.ServiceDescriber{Interface: c.clientset}
	return d.Describe(namespace, name, describe.DescriberSettings{ShowEvents: showEvents, ChunkSize: 500})
}

// ServiceEndpoints lists where the service sends its traffic: one line per
// address and port of its endpoint slices, with whether it is ready and
// the pod and node behind it, like `kubectl get endpointslices -l
// kubernetes.io/service-name=NAME`:
//
//	10.244.1.5:80/TCP   ready       pod web-7d9f-abcde on worker1
//	10.244.2.7:80/TCP   not ready   pod web-7d9f-fghij on worker2
func (c *Client) ServiceEndpoints(ctx context.Context, namespace, name string) ([]string, error) {
	slices, err := c.clientset.DiscoveryV1().EndpointSlices(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: discoveryv1.LabelServiceName + "=" + name,
	})
	if err != nil {
		return nil, err
	}
	var rows [][]string
	for _, s := range slices.Items {
		for _, e := range s.Endpoints {
			ready := "ready"
			if e.Conditions.Ready != nil && !*e.Conditions.Ready {
				ready = "not ready"
			}
			behind := ""
			if e.TargetRef != nil {
				behind = strings.ToLower(e.TargetRef.Kind) + " " + e.TargetRef.Name
			}
			if e.NodeName != nil && *e.NodeName != "" {
				behind += " on " + *e.NodeName
			}
			for _, addr := range e.Addresses {
				if len(s.Ports) == 0 {
					rows = append(rows, []string{addr, ready, behind})
				}
				for _, p := range s.Ports {
					port := addr
					if p.Port != nil {
						port += ":" + strconv.Itoa(int(*p.Port))
					}
					if p.Protocol != nil {
						port += "/" + string(*p.Protocol)
					}
					rows = append(rows, []string{port, ready, behind})
				}
			}
		}
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i][0] < rows[j][0] })
	return alignedRows(rows), nil
}

// alignedRows joins each row's cells with their columns lined up.
func alignedRows(rows [][]string) []string {
	var widths []int
	for _, row := range rows {
		for i, cell := range row {
			if i == len(widths) {
				widths = append(widths, 0)
			}
			widths[i] = max(widths[i], len(cell))
		}
	}
	out := make([]string, 0, len(rows))
	for _, row := range rows {
		var b strings.Builder
		for i, cell := range row {
			b.WriteString(cell)
			if i < len(row)-1 {
				b.WriteString(strings.Repeat(" ", widths[i]-len(cell)+3))
			}
		}
		out = append(out, strings.TrimRight(b.String(), " "))
	}
	return out
}

// ServiceLabelsAndAnnotations returns the service's labels, annotations
// and selector, as key=value sorted by key.
func (c *Client) ServiceLabelsAndAnnotations(ctx context.Context, namespace, name string) (labels, annotations, selector []string, err error) {
	s, err := c.clientset.CoreV1().Services(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, nil, nil, err
	}
	return pairs(s.Labels), pairs(s.Annotations), pairs(s.Spec.Selector), nil
}

// LabelService sets and removes labels on the service, like `kubectl label
// service NAME -n NAMESPACE env=prod old-`. Its selector, which pods it
// sends traffic to, doesn't change.
func (c *Client) LabelService(ctx context.Context, namespace, name string, changes []string, overwrite, dryRun bool) (Change, error) {
	return labelObject[corev1.Service](ctx, c.clientset.CoreV1().Services(namespace), name, changes, overwrite, dryRun)
}

// DeleteService deletes the service, like `kubectl delete service NAME -n
// NAMESPACE`. Its pods keep running; only the way to reach them goes.
func (c *Client) DeleteService(ctx context.Context, namespace, name string, dryRun bool) error {
	opts := metav1.DeleteOptions{}
	if dryRun {
		opts.DryRun = []string{metav1.DryRunAll}
	}
	return c.clientset.CoreV1().Services(namespace).Delete(ctx, name, opts)
}

// parsePorts reads ports as kubectl's --tcp takes them: "PORT" or
// "PORT:TARGETPORT", several with commas, e.g. "80:8080,443".
func parsePorts(ports string) ([]corev1.ServicePort, error) {
	if strings.TrimSpace(ports) == "" {
		return nil, errors.New("no ports given: type PORT or PORT:TARGETPORT, e.g. 80:8080")
	}
	var out []corev1.ServicePort
	for _, p := range strings.Split(ports, ",") {
		port, target, hasTarget := strings.Cut(strings.TrimSpace(p), ":")
		n, err := strconv.Atoi(port)
		if err != nil || n < 1 || n > 65535 {
			return nil, fmt.Errorf("port %q: type a number from 1 to 65535", port)
		}
		t := n
		if hasTarget {
			if t, err = strconv.Atoi(target); err != nil || t < 1 || t > 65535 {
				return nil, fmt.Errorf("target port %q: type a number from 1 to 65535", target)
			}
		}
		out = append(out, corev1.ServicePort{
			Name:       fmt.Sprintf("%d-%d", n, t), // kubectl's names, needed when there are several
			Port:       int32(n),
			TargetPort: intstr.FromInt32(int32(t)),
			Protocol:   corev1.ProtocolTCP,
		})
	}
	return out, nil
}

// CreateService makes a service of type (ClusterIP, NodePort or
// LoadBalancer) called name in namespace, like `kubectl create service
// clusterip NAME --tcp=80:8080 -n NAMESPACE`. As with kubectl, it sends
// traffic to the pods labelled app=NAME, the label `kubectl create
// deployment NAME` gives them.
func (c *Client) CreateService(ctx context.Context, namespace, name string, serviceType corev1.ServiceType, ports string, dryRun bool) error {
	if errs := validation.IsDNS1035Label(name); len(errs) > 0 {
		return fmt.Errorf("service name %q: %s", name, strings.Join(errs, "; "))
	}
	servicePorts, err := parsePorts(ports)
	if err != nil {
		return err
	}
	selector := map[string]string{"app": name}
	return c.createService(ctx, namespace, name, serviceType, selector, servicePorts, dryRun)
}

// ExposeDeployment makes a ClusterIP service for the deployment, called
// as it is, like `kubectl expose deployment NAME --port=80
// --target-port=8080 -n NAMESPACE`: it sends traffic to the deployment's
// pods, picked with the deployment's own selector.
func (c *Client) ExposeDeployment(ctx context.Context, namespace, name, ports string, dryRun bool) error {
	servicePorts, err := parsePorts(ports)
	if err != nil {
		return err
	}
	d, err := c.clientset.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return err
	}
	selector := deploymentSelector(d)
	if len(selector) == 0 {
		return errors.New("the deployment's selector has no plain labels (matchLabels): a service can't use it")
	}
	return c.createService(ctx, namespace, name, corev1.ServiceTypeClusterIP, selector, servicePorts, dryRun)
}

// deploymentSelector is the labels a service can pick the deployment's pods
// with: its selector's matchLabels.
func deploymentSelector(d *appsv1.Deployment) map[string]string {
	if d.Spec.Selector == nil {
		return nil
	}
	return d.Spec.Selector.MatchLabels
}

// createService makes the service.
func (c *Client) createService(ctx context.Context, namespace, name string, serviceType corev1.ServiceType, selector map[string]string, ports []corev1.ServicePort, dryRun bool) error {
	if len(ports) == 1 {
		ports[0].Name = "" // a single port needs no name, as with kubectl
	}
	s := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace, Labels: map[string]string{"app": name}},
		Spec:       corev1.ServiceSpec{Type: serviceType, Selector: selector, Ports: ports},
	}
	opts := metav1.CreateOptions{}
	if dryRun {
		opts.DryRun = []string{metav1.DryRunAll}
	}
	_, err := c.clientset.CoreV1().Services(namespace).Create(ctx, s, opts)
	return err
}
