package commands

import "testing"

// The password isn't shown in the command.
func TestRegistryArgs(t *testing.T) {
	if got := registryArgs("regcred ghcr.io me s3cret"); got != "regcred --docker-server=ghcr.io --docker-username=me --docker-password=***" {
		t.Errorf("registryArgs = %q", got)
	}
}

func TestResourceTabSecrets(t *testing.T) {
	if tab, _, ok := ResourceTab("secrets   v1   true   Secret"); !ok || tab != "secrets" {
		t.Errorf("tab = %q, %v", tab, ok)
	}
}
