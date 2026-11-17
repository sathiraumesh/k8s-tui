package commands

import "testing"

func TestAutoscaleArgs(t *testing.T) {
	if got := autoscaleArgs("web 1 5 80"); got != "web --min=1 --max=5 --cpu=80%" {
		t.Errorf("autoscaleArgs = %q", got)
	}
}
