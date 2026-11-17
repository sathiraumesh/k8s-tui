package commands

import (
	"context"

	k8s "k8s-tui/internal/kubernetes"

	tea "github.com/charmbracelet/bubbletea"
)

// selfSubjectReviewsView is SelfSubjectReviews in More: asking the API
// server who you are. Its own: Who am I.
func selfSubjectReviewsView() View {
	return View{
		Type:       k8s.ResourceType{Group: "authentication.k8s.io", Version: "v1", Resource: "selfsubjectreviews", Kind: "SelfSubjectReview"},
		Title:      "whoami",
		CreateOnly: "Nothing to list: a review is a question to the API server. Press + to ask who you are.",
		Own: []Action{{
			Key: "w", Name: "Who am I", About: "your user name and groups, as the API server sees them",
			Command: "kubectl auth whoami",
			Run: func(c *k8s.Client, _, _ string, _ []Flag) tea.Cmd {
				return Read("Who am I", "", func(ctx context.Context) (string, error) {
					return c.WhoAmIText(ctx)
				})
			},
		}},
		OwnButton: "+ ask",
		OwnTitle:  "Ask who you are",
	}
}
