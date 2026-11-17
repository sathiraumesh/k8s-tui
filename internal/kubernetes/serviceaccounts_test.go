package kubernetes

import (
	"context"
	"testing"
	"time"

	authenticationv1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

// tokenServer is a fake cluster that answers token requests with
// "TOKEN", and keeps the seconds asked for in *asked.
func tokenServer(asked *int64) *fake.Clientset {
	cs := fake.NewClientset()
	cs.PrependReactor("create", "serviceaccounts", func(a k8stesting.Action) (bool, runtime.Object, error) {
		if a.GetSubresource() != "token" {
			return false, nil, nil
		}
		req := a.(k8stesting.CreateAction).GetObject().(*authenticationv1.TokenRequest)
		*asked = *req.Spec.ExpirationSeconds
		req.Status = authenticationv1.TokenRequestStatus{Token: "TOKEN", ExpirationTimestamp: metav1.NewTime(time.Now().Add(time.Hour))}
		return true, req, nil
	})
	return cs
}

func TestServiceAccountToken(t *testing.T) {
	var asked int64
	c := NewClientFromClientset(tokenServer(&asked))
	token, expires, err := c.ServiceAccountToken(context.Background(), "apps", "deployer", "2h")
	if err != nil || token != "TOKEN" || asked != 7200 || expires.IsZero() {
		t.Errorf("token %q, expires %v, asked %d, err %v", token, expires, asked, err)
	}
	for _, bad := range []string{"", "soon", "5m"} {
		if _, _, err := c.ServiceAccountToken(context.Background(), "apps", "deployer", bad); err == nil {
			t.Errorf("duration %q: err = nil", bad)
		}
	}
}

func TestCreateServiceAccount(t *testing.T) {
	cs := fake.NewClientset()
	if err := NewClientFromClientset(cs).CreateServiceAccount(context.Background(), "apps", "deployer", false); err != nil {
		t.Fatal(err)
	}
	if _, err := cs.CoreV1().ServiceAccounts("apps").Get(context.Background(), "deployer", metav1.GetOptions{}); err != nil {
		t.Errorf("no service account: %v", err)
	}
}
