package kubernetes

import (
	"context"
	"testing"

	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestParseRule(t *testing.T) {
	r, err := parseRule("shop.example.com/api*=api:8080")
	p := r.HTTP.Paths[0]
	if err != nil || r.Host != "shop.example.com" || p.Path != "/api" || *p.PathType != networkingv1.PathTypePrefix ||
		p.Backend.Service.Name != "api" || p.Backend.Service.Port.Number != 8080 {
		t.Errorf("rule = %+v, err = %v", r, err)
	}
	r, _ = parseRule("/=web:80")
	if r.Host != "" || r.HTTP.Paths[0].Path != "/" || *r.HTTP.Paths[0].PathType != networkingv1.PathTypeExact {
		t.Errorf("any host = %+v", r)
	}
	for _, bad := range []string{"web:80", "a.com/=web", "a.com/=:80", "a.com/=web:http", "a.com=web:80"} {
		if _, err := parseRule(bad); err == nil {
			t.Errorf("%q: err = nil", bad)
		}
	}
}

func TestCreateIngress(t *testing.T) {
	cs := fake.NewClientset()
	c := NewClientFromClientset(cs)
	if err := c.CreateIngress(context.Background(), "apps", "shop", []string{"a.com/*=web:80", "b.com/=api:81"}, false); err != nil {
		t.Fatal(err)
	}
	ing, _ := cs.NetworkingV1().Ingresses("apps").Get(context.Background(), "shop", metav1.GetOptions{})
	if len(ing.Spec.Rules) != 2 || ing.Spec.Rules[1].Host != "b.com" {
		t.Errorf("ingress = %+v", ing.Spec)
	}
	if err := c.CreateIngress(context.Background(), "apps", "x", nil, false); err == nil {
		t.Error("no rules: err = nil")
	}
}
