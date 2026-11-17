package kubernetes

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestAutoscale(t *testing.T) {
	cs := fake.NewClientset(fakeDeployment("apps", "web"))
	c := NewClientFromClientset(cs)
	if err := c.Autoscale(context.Background(), "apps", "web", "2", "8", "70%", false); err != nil {
		t.Fatal(err)
	}
	hpa, _ := cs.AutoscalingV2().HorizontalPodAutoscalers("apps").Get(context.Background(), "web", metav1.GetOptions{})
	if *hpa.Spec.MinReplicas != 2 || hpa.Spec.MaxReplicas != 8 || *hpa.Spec.Metrics[0].Resource.Target.AverageUtilization != 70 || hpa.Spec.ScaleTargetRef.Name != "web" {
		t.Errorf("hpa = %+v", hpa.Spec)
	}
	for _, bad := range [][3]string{{"0", "5", "80"}, {"5", "2", "80"}, {"1", "5", "x"}} {
		if err := c.Autoscale(context.Background(), "apps", "web", bad[0], bad[1], bad[2], false); err == nil {
			t.Errorf("%q: err = nil", bad)
		}
	}
	if err := c.Autoscale(context.Background(), "apps", "nope", "1", "2", "80", false); err == nil {
		t.Error("missing deployment: err = nil")
	}
}
