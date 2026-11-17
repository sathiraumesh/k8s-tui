package kubernetes

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestCreatePodDisruptionBudget(t *testing.T) {
	cs := fake.NewClientset()
	c := NewClientFromClientset(cs)
	if err := c.CreatePodDisruptionBudget(context.Background(), "apps", "web", "app=web", "50%", false); err != nil {
		t.Fatal(err)
	}
	pdb, _ := cs.PolicyV1().PodDisruptionBudgets("apps").Get(context.Background(), "web", metav1.GetOptions{})
	if pdb.Spec.Selector.MatchLabels["app"] != "web" || pdb.Spec.MinAvailable.String() != "50%" {
		t.Errorf("pdb = %+v", pdb.Spec)
	}
	for _, bad := range [][2]string{{"", "1"}, {"app=web", ""}, {"app=web", "lots"}} {
		if err := c.CreatePodDisruptionBudget(context.Background(), "apps", "x", bad[0], bad[1], false); err == nil {
			t.Errorf("%q: err = nil", bad)
		}
	}
}
