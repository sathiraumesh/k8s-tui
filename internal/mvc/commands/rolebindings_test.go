package commands

import "testing"

func TestBindingArgs(t *testing.T) {
	if got := bindingArgs("viewers clusterrole:view user:alice serviceaccount:apps:ci"); got != "viewers --clusterrole=view --user=alice --serviceaccount=apps:ci" {
		t.Errorf("bindingArgs = %q", got)
	}
}
