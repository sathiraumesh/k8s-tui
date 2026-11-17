package kubernetes

import (
	"context"
	"strings"
	"testing"

	authenticationv1 "k8s.io/api/authentication/v1"
	authorizationv1 "k8s.io/api/authorization/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

// answering is a fake cluster that answers creating resource with answer,
// which is given the review created.
func answering(resource string, answer func(obj runtime.Object) runtime.Object) *fake.Clientset {
	cs := fake.NewClientset()
	cs.PrependReactor("create", resource, func(a k8stesting.Action) (bool, runtime.Object, error) {
		return true, answer(a.(k8stesting.CreateAction).GetObject()), nil
	})
	return cs
}

func TestWhoAmI(t *testing.T) {
	cs := answering("selfsubjectreviews", func(runtime.Object) runtime.Object {
		return &authenticationv1.SelfSubjectReview{Status: authenticationv1.SelfSubjectReviewStatus{
			UserInfo: authenticationv1.UserInfo{Username: "kubernetes-admin", Groups: []string{"system:masters", "kubeadm:cluster-admins"}},
		}}
	})
	text, err := NewClientFromClientset(cs).WhoAmIText(context.Background())
	if err != nil || text != "Username: kubernetes-admin\nGroups:\n  kubeadm:cluster-admins\n  system:masters" {
		t.Errorf("text = %q, err = %v", text, err)
	}
}

func TestCanI(t *testing.T) {
	var asked *authorizationv1.ResourceAttributes
	cs := answering("selfsubjectaccessreviews", func(obj runtime.Object) runtime.Object {
		r := obj.(*authorizationv1.SelfSubjectAccessReview)
		asked = r.Spec.ResourceAttributes
		r.Status = authorizationv1.SubjectAccessReviewStatus{Allowed: true, Reason: "RBAC: allowed by ClusterRoleBinding"}
		return r
	})
	cs.Resources = rbacCluster().Resources
	text, err := NewClientFromClientset(cs).CanI(context.Background(), "create", "deployments", "apps")
	if err != nil || !strings.HasPrefix(text, "yes") || asked.Group != "apps" || asked.Resource != "deployments" || asked.Namespace != "apps" {
		t.Errorf("text = %q, asked = %+v, err = %v", text, asked, err)
	}
	if _, err := NewClientFromClientset(cs).CanI(context.Background(), "create", "", ""); err == nil {
		t.Error("no resource: err = nil")
	}
}

func TestMyRules(t *testing.T) {
	cs := answering("selfsubjectrulesreviews", func(obj runtime.Object) runtime.Object {
		r := obj.(*authorizationv1.SelfSubjectRulesReview)
		r.Status.ResourceRules = []authorizationv1.ResourceRule{
			{Verbs: []string{"get", "list"}, APIGroups: []string{""}, Resources: []string{"pods"}},
			{Verbs: []string{"*"}, APIGroups: []string{"apps"}, Resources: []string{"deployments"}},
		}
		r.Status.NonResourceRules = []authorizationv1.NonResourceRule{{Verbs: []string{"get"}, NonResourceURLs: []string{"/healthz"}}}
		return r
	})
	text, err := NewClientFromClientset(cs).MyRules(context.Background(), "apps")
	want := "RESOURCES          NAMES   VERBS\npods               []      [get list]\ndeployments.apps   []      [*]\n/healthz           []      [get]"
	if err != nil || text != want {
		t.Errorf("text =\n%s\nwant\n%s", text, want)
	}
}

func TestCanThey(t *testing.T) {
	var asked authorizationv1.SubjectAccessReviewSpec
	cs := answering("subjectaccessreviews", func(obj runtime.Object) runtime.Object {
		r := obj.(*authorizationv1.SubjectAccessReview)
		asked = r.Spec
		return r // not allowed
	})
	cs.Resources = rbacCluster().Resources
	text, err := NewClientFromClientset(cs).CanThey(context.Background(), "serviceaccount:apps:ci", "create", "deployments", "apps", false)
	if err != nil || text != "no" || asked.User != "system:serviceaccount:apps:ci" || asked.ResourceAttributes.Group != "apps" {
		t.Errorf("text = %q, asked = %+v, err = %v", text, asked, err)
	}
	if _, err := NewClientFromClientset(cs).CanThey(context.Background(), "alice", "get", "pods", "", false); err == nil {
		t.Error("who without a kind: err = nil")
	}
}

func TestCanTheyLocal(t *testing.T) {
	var in string
	cs := answering("localsubjectaccessreviews", func(obj runtime.Object) runtime.Object {
		r := obj.(*authorizationv1.LocalSubjectAccessReview)
		in = r.Namespace
		r.Status.Allowed = true
		return r
	})
	cs.Resources = rbacCluster().Resources
	if text, err := NewClientFromClientset(cs).CanThey(context.Background(), "user:alice", "get", "pods", "apps", true); err != nil || text != "yes" || in != "apps" {
		t.Errorf("text = %q, in %q, err = %v", text, in, err)
	}
}
