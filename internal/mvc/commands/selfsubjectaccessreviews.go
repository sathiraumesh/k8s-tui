package commands

import (
	"context"
	"strings"

	k8s "k8s-tui/internal/kubernetes"

	tea "github.com/charmbracelet/bubbletea"
)

// selfSubjectAccessReviewsView is SelfSubjectAccessReviews in More: asking
// whether you may do something. Its own: Can I.
func selfSubjectAccessReviewsView() View {
	return View{
		Type:       k8s.ResourceType{Group: "authorization.k8s.io", Version: "v1", Resource: "selfsubjectaccessreviews", Kind: "SelfSubjectAccessReview"},
		Title:      "can-i",
		CreateOnly: "Nothing to list: a review is a question to the API server. Press + to ask whether you may do something.",
		Own: []Action{{
			Key: "c", Name: "Can I", About: "verb resource [namespace]: create deployments apps (no namespace: all of them)",
			Input:     "verb resource [namespace]",
			InputArgs: canIArgs,
			Command:   "kubectl auth can-i " + InputHere,
			Run: func(c *k8s.Client, _, input string, _ []Flag) tea.Cmd {
				f := append(strings.Fields(input), "", "", "")
				return Read("Can I", strings.TrimSpace(input), func(ctx context.Context) (string, error) {
					return c.CanI(ctx, f[0], f[1], f[2])
				})
			},
		}},
		OwnButton: "+ ask",
		OwnTitle:  "Ask whether you may",
	}
}

// canIArgs writes "create deployments apps" as kubectl takes it:
// "create deployments -n apps".
func canIArgs(input string) string {
	f := strings.Fields(input)
	if len(f) == 3 {
		return f[0] + " " + f[1] + " -n " + f[2]
	}
	return input
}
