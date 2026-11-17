package commands

import (
	"context"
	"strings"

	k8s "k8s-tui/internal/kubernetes"

	tea "github.com/charmbracelet/bubbletea"
)

// certificateSigningRequestsView is CertificateSigningRequests in More:
// requests for a certificate, from a new node's kubelet or a user. Its
// own: CONDITION colored, Approve and Deny.
func certificateSigningRequestsView() View {
	return View{
		Type:   k8s.ResourceType{Group: "certificates.k8s.io", Version: "v1", Resource: "certificatesigningrequests", Kind: "CertificateSigningRequest"},
		Title:  "csr",
		Status: &CertificateCondition,
		Actions: []Action{
			approveAction("a", "Approve", "let its signer sign it; check who asks (REQUESTOR) first", true),
			approveAction("n", "Deny", "refuse it; a decision can't be changed", false),
		},
	}
}

// approveAction is Approve or Deny: kubectl certificate approve (deny) NAME.
func approveAction(key, name, about string, approve bool) Action {
	return Action{
		Key: key, Name: name, About: about,
		Changes: true,
		Flags:   []Flag{{Name: FlagDryRun}},
		Command: "kubectl certificate " + strings.ToLower(name) + " %s",
		Run: func(c *k8s.Client, target, _ string, flags []Flag) tea.Cmd {
			return Change(name, target, flags, func(ctx context.Context) (string, error) {
				return "", c.ApproveCertificate(ctx, target, approve, Ticked(flags, FlagDryRun))
			})
		},
	}
}

// CertificateCondition is the CONDITION column of the requests: NAME AGE
// SIGNERNAME REQUESTOR REQUESTEDDURATION CONDITION. Approved (and Issued)
// is green, Pending yellow, Denied or Failed red.
var CertificateCondition = StatusColumn{Column: 5, Level: func(condition string) Level {
	switch {
	case strings.Contains(condition, "Denied"), strings.Contains(condition, "Failed"):
		return LevelBad
	case strings.Contains(condition, "Approved"):
		return LevelOK
	case condition == "Pending":
		return LevelWarn
	}
	return LevelNone
}}
