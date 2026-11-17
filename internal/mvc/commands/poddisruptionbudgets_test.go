package commands

import "testing"

func TestPdbArgs(t *testing.T) {
	if got := pdbArgs("web app=web 1"); got != "web --selector=app=web --min-available=1" {
		t.Errorf("pdbArgs = %q", got)
	}
}
