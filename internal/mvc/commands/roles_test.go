package commands

import (
	"slices"
	"testing"
)

func TestRoleArgs(t *testing.T) {
	if got := roleArgs("reader get,list pods,deployments"); got != "reader --verb=get,list --resource=pods,deployments" {
		t.Errorf("roleArgs = %q", got)
	}
	if got := splitList("get, list,,watch"); !slices.Equal(got, []string{"get", "list", "watch"}) {
		t.Errorf("splitList = %q", got)
	}
}
