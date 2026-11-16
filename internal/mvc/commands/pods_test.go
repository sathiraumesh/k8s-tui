package commands

import (
	"strings"
	"testing"
	"time"

	k8s "k8s-tui/internal/kubernetes"
)

var podsNow = time.Date(2026, 11, 13, 12, 0, 0, 0, time.UTC)

func TestPodRows(t *testing.T) {
	pods := []k8s.Pod{
		{Namespace: "default", Name: "web", Ready: "1/1", Status: "Running", CreatedAt: podsNow.Add(-3 * time.Hour), IP: "10.0.0.5", Node: "worker1"},
		{Namespace: "kube-system", Name: "dns", Ready: "0/1", Status: "CrashLoopBackOff", Restarts: 4, LastRestart: podsNow.Add(-5 * time.Minute), CreatedAt: podsNow.Add(-8 * 24 * time.Hour)},
	}
	got := podRows(pods, false, true, podsNow)
	want := []string{
		"default       web   1/1   Running            0            3h",
		"kube-system   dns   0/1   CrashLoopBackOff   4 (5m ago)   8d",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("rows =\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}

	wide := podRows(pods, true, true, podsNow)
	if !strings.HasSuffix(wide[0], "10.0.0.5   worker1   <none>   <none>") || !strings.HasSuffix(wide[1], "8d   <none>     <none>    <none>   <none>") {
		t.Errorf("wide rows =\n%s", strings.Join(wide, "\n"))
	}
}

func TestPodAge(t *testing.T) {
	if got := age(time.Time{}, podsNow); got != "<unknown>" {
		t.Errorf("age of zero time = %q", got)
	}
	if got := age(podsNow.Add(-45*time.Second), podsNow); got != "45s" {
		t.Errorf("age = %q, want 45s", got)
	}
}
