package commands

import (
	"strings"
	"testing"
	"time"

	k8s "k8s-tui/internal/kubernetes"
)

func TestDeploymentRows(t *testing.T) {
	deployments := []k8s.Deployment{
		{Namespace: "default", Name: "web", Ready: "2/3", UpToDate: 3, Available: 2, CreatedAt: podsNow.Add(-3 * time.Hour),
			Containers: []string{"app", "sidecar"}, Images: []string{"nginx", "envoy"}, Selector: "app=web"},
		{Namespace: "kube-system", Name: "coredns", Ready: "1/1", UpToDate: 1, Available: 1, CreatedAt: podsNow.Add(-8 * 24 * time.Hour)},
	}
	got := deploymentRows(deployments, false, true, podsNow)
	want := []string{
		"default       web       2/3   3   2   3h",
		"kube-system   coredns   1/1   1   1   8d",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("rows =\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}

	wide := deploymentRows(deployments, true, false, podsNow)
	if !strings.HasPrefix(wide[0], "web ") || !strings.HasSuffix(wide[0], "app,sidecar   nginx,envoy   app=web") || strings.Count(wide[1], "<none>") != 3 {
		t.Errorf("wide rows =\n%s", strings.Join(wide, "\n"))
	}
}
