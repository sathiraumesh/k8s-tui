package commands

import (
	"testing"

	k8s "k8s-tui/internal/kubernetes"
)

func TestLiteralArgs(t *testing.T) {
	if got := literalArgs("cfg mode=fast level=3"); got != "cfg --from-literal=mode=fast --from-literal=level=3" {
		t.Errorf("literalArgs = %q", got)
	}
}

func TestEntriesText(t *testing.T) {
	got := entriesText([]k8s.KeyValue{{Key: "mode", Value: "fast"}, {Key: "a.yaml", Value: "x: 1\ny: 2\n"}})
	if want := "mode:\n  fast\na.yaml:\n  x: 1\n  y: 2\n"; got != want {
		t.Errorf("entriesText = %q, want %q", got, want)
	}
	if entriesText(nil) != "(no entries)" {
		t.Error("no entries")
	}
}

func TestResourceTabConfigMaps(t *testing.T) {
	if tab, _, ok := ResourceTab("configmaps   cm   v1   true   ConfigMap"); !ok || tab != "configmaps" {
		t.Errorf("tab = %q, %v", tab, ok)
	}
}
