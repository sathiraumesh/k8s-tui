package mvc

import (
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestStatefulSetActionCommands(t *testing.T) {
	for k, want := range map[string]string{
		"c": "kubectl scale statefulsets.apps db -n apps",
		"t": "kubectl rollout restart statefulsets.apps db -n apps",
		"d": "kubectl describe statefulsets.apps db -n apps",
	} {
		m := openKindFromMore(t, fake.NewClientset(), "statefulsets   sts   apps/v1   true   StatefulSet", "apps   db   2/3   3h")
		if m = press(t, m, enterKey, key(k)); m.kubectlCommand() != want {
			t.Errorf("%s: %q, want %q", k, m.kubectlCommand(), want)
		}
	}
}

func TestStatefulSetScaleRuns(t *testing.T) {
	replicas := int32(3)
	cs := fake.NewClientset(&appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Namespace: "apps", Name: "db"}, Spec: appsv1.StatefulSetSpec{Replicas: &replicas}})
	m := openKindFromMore(t, cs, "statefulsets   sts   apps/v1   true   StatefulSet", "apps   db   3/3   3h")
	m = typeText(t, press(t, m, enterKey, key("c")), "5")
	if m.kubectlCommand() != "kubectl scale statefulsets.apps db -n apps --replicas=5" {
		t.Errorf("command = %q", m.kubectlCommand())
	}
	_, cmd := press(t, m, enterKey).Update(key(KeyYes))
	if m = run(t, m, cmd); m.notice != "Scale apps/db: done" {
		t.Errorf("notice = %q", m.notice)
	}
	if s, _ := cs.AppsV1().StatefulSets("apps").Get(t.Context(), "db", metav1.GetOptions{}); *s.Spec.Replicas != 5 {
		t.Errorf("replicas = %d", *s.Spec.Replicas)
	}
}
