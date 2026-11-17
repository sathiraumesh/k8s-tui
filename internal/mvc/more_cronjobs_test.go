package mvc

import (
	"testing"

	"k8s.io/client-go/kubernetes/fake"
)

func TestCronJobActionCommands(t *testing.T) {
	m := openKindFromMore(t, fake.NewClientset(), "cronjobs   cj   batch/v1   true   CronJob", "apps   backup   @daily   <none>   False   0   <none>   3h")
	if m = press(t, m, enterKey, key("n")); m.kubectlCommand() != "kubectl create job --from=cronjob/backup -n apps" {
		t.Errorf("run now: %q", m.kubectlCommand())
	}
}
