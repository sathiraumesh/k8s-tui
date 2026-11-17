package commands

import "testing"

func TestCanIArgs(t *testing.T) {
	if got := canIArgs("create deployments apps"); got != "create deployments -n apps" {
		t.Errorf("canIArgs = %q", got)
	}
	if got := canIArgs("get nodes"); got != "get nodes" {
		t.Errorf("canIArgs = %q", got)
	}
}
