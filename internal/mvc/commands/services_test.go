package commands

import (
	"strings"
	"testing"
	"time"

	k8s "k8s-tui/internal/kubernetes"
)

func TestServiceRows(t *testing.T) {
	services := []k8s.Service{
		{Namespace: "default", Name: "web", Type: "ClusterIP", ClusterIP: "10.96.0.10", ExternalIP: "<none>", Ports: "80/TCP", Selector: "app=web", CreatedAt: podsNow.Add(-3 * time.Hour)},
		{Namespace: "default", Name: "lb", Type: "LoadBalancer", ClusterIP: "10.96.0.11", ExternalIP: "<pending>", Ports: "80:30080/TCP", Selector: "app=lb", CreatedAt: podsNow.Add(-5 * time.Minute)},
	}
	got := serviceRows(services, false, true, podsNow)
	want := []string{
		"default   web   ClusterIP      10.96.0.10   <none>      80/TCP         3h",
		"default   lb    LoadBalancer   10.96.0.11   <pending>   80:30080/TCP   5m",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("rows =\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	if wide := serviceRows(services, true, false, podsNow); !strings.HasPrefix(wide[0], "web ") || !strings.HasSuffix(wide[0], "app=web") {
		t.Errorf("wide rows =\n%s", strings.Join(wide, "\n"))
	}
}

func TestServiceArgs(t *testing.T) {
	if got := serviceArgs("web 80:8080"); got != "web --tcp=80:8080" {
		t.Errorf("serviceArgs = %q", got)
	}
	if got := exposeArgs("80:8080"); got != "--port=80 --target-port=8080" {
		t.Errorf("exposeArgs = %q", got)
	}
	if got := exposeArgs("80"); got != "--port=80" {
		t.Errorf("exposeArgs = %q", got)
	}
}

func TestPendingExternalIPYellow(t *testing.T) {
	if ServiceExternalIP.Level("<pending>") != LevelWarn || ServiceExternalIP.Level("<none>") != LevelNone {
		t.Error("<pending> should be yellow, the rest not colored")
	}
}
