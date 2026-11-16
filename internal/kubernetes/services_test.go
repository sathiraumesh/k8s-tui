package kubernetes

import (
	"context"
	"slices"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes/fake"
)

// fakeService is a ClusterIP service on port 80 for the pods app=NAME.
func fakeService(namespace, name string) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name},
		Spec: corev1.ServiceSpec{
			Type:      corev1.ServiceTypeClusterIP,
			ClusterIP: "10.96.0.10",
			Selector:  map[string]string{"app": name},
			Ports:     []corev1.ServicePort{{Port: 80, TargetPort: intstr.FromInt32(8080), Protocol: corev1.ProtocolTCP}},
		},
	}
}

func TestListServices(t *testing.T) {
	cs := fake.NewClientset(fakeService("b", "web"), fakeService("a", "api"))
	got, err := NewClientFromClientset(cs).ListServices(context.Background(), "")
	if err != nil || len(got) != 2 || got[0].Name != "api" || got[1].Name != "web" {
		t.Fatalf("services = %+v, err = %v", got, err)
	}
	if s := got[0]; s.Type != "ClusterIP" || s.ClusterIP != "10.96.0.10" || s.ExternalIP != "<none>" || s.Ports != "80/TCP" || s.Selector != "app=api" {
		t.Errorf("service = %+v", s)
	}
}

func TestToServiceColumns(t *testing.T) {
	for _, tc := range []struct {
		name            string
		edit            func(s *corev1.Service)
		external, ports string
	}{
		{"node port", func(s *corev1.Service) {
			s.Spec.Type = corev1.ServiceTypeNodePort
			s.Spec.Ports[0].NodePort = 30080
		}, "<none>", "80:30080/TCP"},
		{"load balancer waiting", func(s *corev1.Service) {
			s.Spec.Type = corev1.ServiceTypeLoadBalancer
		}, "<pending>", "80/TCP"},
		{"load balancer with address", func(s *corev1.Service) {
			s.Spec.Type = corev1.ServiceTypeLoadBalancer
			s.Status.LoadBalancer.Ingress = []corev1.LoadBalancerIngress{{IP: "203.0.113.5"}, {Hostname: "lb.example.com"}}
		}, "203.0.113.5,lb.example.com", "80/TCP"},
		{"external name", func(s *corev1.Service) {
			s.Spec.Type = corev1.ServiceTypeExternalName
			s.Spec.ExternalName = "db.example.com"
			s.Spec.Ports = nil
		}, "db.example.com", "<none>"},
		{"external IPs", func(s *corev1.Service) {
			s.Spec.ExternalIPs = []string{"198.51.100.1"}
		}, "198.51.100.1", "80/TCP"},
	} {
		s := fakeService("a", "web")
		tc.edit(s)
		if got := toService(*s); got.ExternalIP != tc.external || got.Ports != tc.ports {
			t.Errorf("%s: external = %q, ports = %q, want %q, %q", tc.name, got.ExternalIP, got.Ports, tc.external, tc.ports)
		}
	}
}

func TestDescribeService(t *testing.T) {
	text, err := NewClientFromClientset(fake.NewClientset(fakeService("apps", "web"))).DescribeService(context.Background(), "apps", "web", true)
	if err != nil || !strings.Contains(text, "web") || !strings.Contains(text, "10.96.0.10") || !strings.Contains(text, "app=web") {
		t.Errorf("describe = %q, err = %v", text, err)
	}
}

func TestServiceEndpoints(t *testing.T) {
	ready, notReady := true, false
	port, proto, node := int32(8080), corev1.ProtocolTCP, "worker1"
	slice := &discoveryv1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{Namespace: "apps", Name: "web-abc", Labels: map[string]string{discoveryv1.LabelServiceName: "web"}},
		Ports:      []discoveryv1.EndpointPort{{Port: &port, Protocol: &proto}},
		Endpoints: []discoveryv1.Endpoint{
			{Addresses: []string{"10.244.1.5"}, Conditions: discoveryv1.EndpointConditions{Ready: &ready}, NodeName: &node, TargetRef: &corev1.ObjectReference{Kind: "Pod", Name: "web-1"}},
			{Addresses: []string{"10.244.1.6"}, Conditions: discoveryv1.EndpointConditions{Ready: &notReady}, TargetRef: &corev1.ObjectReference{Kind: "Pod", Name: "web-2"}},
		},
	}
	other := slice.DeepCopy()
	other.Name, other.Labels = "api-abc", map[string]string{discoveryv1.LabelServiceName: "api"}
	got, err := NewClientFromClientset(fake.NewClientset(slice, other)).ServiceEndpoints(context.Background(), "apps", "web")
	want := []string{
		"10.244.1.5:8080/TCP   ready       pod web-1 on worker1",
		"10.244.1.6:8080/TCP   not ready   pod web-2",
	}
	if err != nil || !slices.Equal(got, want) {
		t.Errorf("endpoints =\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestCreateService(t *testing.T) {
	cs := fake.NewClientset()
	c := NewClientFromClientset(cs)
	if err := c.CreateService(context.Background(), "apps", "web", corev1.ServiceTypeNodePort, "80:8080,443", false); err != nil {
		t.Fatal(err)
	}
	s, err := cs.CoreV1().Services("apps").Get(context.Background(), "web", metav1.GetOptions{})
	if err != nil || s.Spec.Type != corev1.ServiceTypeNodePort || s.Spec.Selector["app"] != "web" || len(s.Spec.Ports) != 2 ||
		s.Spec.Ports[0].TargetPort.IntValue() != 8080 || s.Spec.Ports[1].TargetPort.IntValue() != 443 || s.Spec.Ports[0].Name == "" {
		t.Errorf("service = %+v, err = %v", s, err)
	}
	for _, bad := range []struct{ name, ports string }{{"web2", ""}, {"web2", "http"}, {"web2", "80:0"}, {"web2", "70000"}, {"Bad_Name", "80"}} {
		if err := c.CreateService(context.Background(), "apps", bad.name, corev1.ServiceTypeClusterIP, bad.ports, false); err == nil {
			t.Errorf("%q %q: err = nil", bad.name, bad.ports)
		}
	}
}

func TestExposeDeployment(t *testing.T) {
	cs := fake.NewClientset(fakeDeployment("apps", "web"))
	if err := NewClientFromClientset(cs).ExposeDeployment(context.Background(), "apps", "web", "80:8080", false); err != nil {
		t.Fatal(err)
	}
	s, err := cs.CoreV1().Services("apps").Get(context.Background(), "web", metav1.GetOptions{})
	if err != nil || s.Spec.Selector["app"] != "web" || s.Spec.Ports[0].Port != 80 || s.Spec.Ports[0].TargetPort.IntValue() != 8080 || s.Spec.Ports[0].Name != "" {
		t.Errorf("service = %+v, err = %v", s, err)
	}
}

func TestLabelAndDeleteService(t *testing.T) {
	cs := fake.NewClientset(fakeService("apps", "web"))
	c := NewClientFromClientset(cs)
	ch, err := c.LabelService(context.Background(), "apps", "web", []string{"env=prod"}, false, false)
	if err != nil || !slices.Equal(ch.After, []string{"env=prod"}) {
		t.Errorf("label: %+v, %v", ch, err)
	}
	if err := c.DeleteService(context.Background(), "apps", "web", false); err != nil {
		t.Fatal(err)
	}
	if _, err := cs.CoreV1().Services("apps").Get(context.Background(), "web", metav1.GetOptions{}); err == nil {
		t.Error("still there")
	}
}
