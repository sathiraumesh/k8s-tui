package kubernetes

import (
	"context"
	"testing"

	certificatesv1 "k8s.io/api/certificates/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestApproveCertificate(t *testing.T) {
	cs := fake.NewClientset(&certificatesv1.CertificateSigningRequest{ObjectMeta: metav1.ObjectMeta{Name: "alice"}})
	c := NewClientFromClientset(cs)
	if err := c.ApproveCertificate(context.Background(), "alice", true, false); err != nil {
		t.Fatal(err)
	}
	csr, _ := cs.CertificatesV1().CertificateSigningRequests().Get(context.Background(), "alice", metav1.GetOptions{})
	if len(csr.Status.Conditions) != 1 || csr.Status.Conditions[0].Type != certificatesv1.CertificateApproved {
		t.Errorf("conditions = %+v", csr.Status.Conditions)
	}
	if err := c.ApproveCertificate(context.Background(), "alice", true, false); err == nil {
		t.Error("approving twice: err = nil")
	}
	if err := c.ApproveCertificate(context.Background(), "alice", false, false); err == nil {
		t.Error("denying an approved one: err = nil")
	}
}
