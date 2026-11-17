package commands

import (
	"context"
	"strings"

	k8s "k8s-tui/internal/kubernetes"

	tea "github.com/charmbracelet/bubbletea"
)

// selfSubjectRulesReviewsView is SelfSubjectRulesReviews in More: asking
// what you may do in a namespace. Its own: My rules.
func selfSubjectRulesReviewsView() View {
	return View{
		Type:       k8s.ResourceType{Group: "authorization.k8s.io", Version: "v1", Resource: "selfsubjectrulesreviews", Kind: "SelfSubjectRulesReview"},
		Title:      "my-rules",
		CreateOnly: "Nothing to list: a review is a question to the API server. Press + to see what you may do in a namespace.",
		Own: []Action{{
			Key: "m", Name: "My rules", About: "what you may do in the namespace typed: each resource and its verbs",
			Input:        "namespace",
			InputDefault: "default",
			InputArgs:    func(ns string) string { return "--list -n " + strings.TrimSpace(ns) },
			Command:      "kubectl auth can-i " + InputHere,
			Run: func(c *k8s.Client, _, namespace string, _ []Flag) tea.Cmd {
				namespace = strings.TrimSpace(namespace)
				return Read("My rules", namespace, func(ctx context.Context) (string, error) {
					return c.MyRules(ctx, namespace)
				})
			},
		}},
		OwnButton: "+ ask",
		OwnTitle:  "Ask what you may do",
	}
}
