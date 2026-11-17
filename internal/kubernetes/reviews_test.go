package kubernetes

import (
	"context"
	"testing"

	authenticationv1 "k8s.io/api/authentication/v1"
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
