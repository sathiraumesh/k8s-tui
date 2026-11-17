package kubernetes

import (
	"context"
	"fmt"

	certificatesv1 "k8s.io/api/certificates/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// What certificate signing requests have of their own; the rest is the
// generic calls (generic.go). A request asks a signer for a certificate,
// e.g. a new node's kubelet, or a user's for kubectl; a person (or a
// controller) approves or denies it.

// ApproveCertificate approves (approve true) or denies the request, like
// `kubectl certificate approve NAME` (or deny). An approved request is
// then signed by its signer. A request that already has a decision keeps
// it: approving one that was denied is refused, as by kubectl.
func (c *Client) ApproveCertificate(ctx context.Context, name string, approve, dryRun bool) error {
	csrs := c.clientset.CertificatesV1().CertificateSigningRequests()
	csr, err := csrs.Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return err
	}
	want, reason := certificatesv1.CertificateApproved, "KubectlApprove"
	if !approve {
		want, reason = certificatesv1.CertificateDenied, "KubectlDeny"
	}
	for _, cond := range csr.Status.Conditions {
		switch {
		case cond.Type == want:
			return fmt.Errorf("it is %s already", want)
		case cond.Type == certificatesv1.CertificateApproved || cond.Type == certificatesv1.CertificateDenied:
			return fmt.Errorf("it is %s: a decision can't be changed", cond.Type)
		}
	}
	csr.Status.Conditions = append(csr.Status.Conditions, certificatesv1.CertificateSigningRequestCondition{
		Type:           want,
		Status:         corev1.ConditionTrue,
		Reason:         reason,
		Message:        "This CSR was " + map[bool]string{true: "approved", false: "denied"}[approve] + " by k8s-tui",
		LastUpdateTime: metav1.Now(),
	})
	_, err = csrs.UpdateApproval(ctx, name, csr, metav1.UpdateOptions{DryRun: dryRunOption(dryRun)})
	return err
}
