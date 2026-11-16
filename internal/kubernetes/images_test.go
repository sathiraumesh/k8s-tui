package kubernetes

import (
	"context"
	"slices"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestSetDeploymentImage(t *testing.T) {
	cs := fake.NewClientset(fakeDeployment("apps", "web"))
	ch, err := NewClientFromClientset(cs).SetDeploymentImage(context.Background(), "apps", "web", []string{"app=nginx:1.28"}, false)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"setup=busybox", "app=nginx:1.28", "sidecar=envoy"}
	if !slices.Equal(ch.After, want) || !slices.Contains(ch.Before, "app=nginx:1.27") {
		t.Errorf("change = %+v", ch)
	}
	d, _ := cs.AppsV1().Deployments("apps").Get(context.Background(), "web", metav1.GetOptions{})
	if d.Spec.Template.Spec.Containers[0].Image != "nginx:1.28" {
		t.Errorf("image = %q", d.Spec.Template.Spec.Containers[0].Image)
	}
}

func TestSetDeploymentImageAll(t *testing.T) {
	cs := fake.NewClientset(fakeDeployment("apps", "web"))
	ch, err := NewClientFromClientset(cs).SetDeploymentImage(context.Background(), "apps", "web", []string{"*=alpine"}, false)
	if err != nil || !slices.Equal(ch.After, []string{"setup=alpine", "app=alpine", "sidecar=alpine"}) {
		t.Errorf("change = %+v, err = %v", ch, err)
	}
}

func TestSetImageErrors(t *testing.T) {
	c := NewClientFromClientset(fake.NewClientset(fakeDeployment("apps", "web")))
	for _, typed := range [][]string{nil, {"app"}, {"app="}, {"=nginx"}, {"nope=nginx"}} {
		if _, err := c.SetDeploymentImage(context.Background(), "apps", "web", typed, false); err == nil {
			t.Errorf("%q: err = nil", typed)
		}
	}
}

func TestSetPodImage(t *testing.T) {
	cs := fake.NewClientset(fakePod("apps", "web"))
	ch, err := NewClientFromClientset(cs).SetPodImage(context.Background(), "apps", "web", []string{"app=nginx:1.28"}, false)
	if err != nil || !slices.Equal(ch.After, []string{"app=nginx:1.28"}) {
		t.Errorf("change = %+v, err = %v", ch, err)
	}
}
