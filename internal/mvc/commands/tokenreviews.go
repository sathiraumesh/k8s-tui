package commands

import (
	"context"

	k8s "k8s-tui/internal/kubernetes"

	tea "github.com/charmbracelet/bubbletea"
)

// tokenReviewsView is TokenReviews in More: asking whose a token is. Its
// own: Check token.
func tokenReviewsView() View {
	return View{
		Type:       k8s.ResourceType{Group: "authentication.k8s.io", Version: "v1", Resource: "tokenreviews", Kind: "TokenReview"},
		Title:      "token",
		CreateOnly: "Nothing to list: a review is a question to the API server. Press + to check a token.",
		Own: []Action{{
			Key: "c", Name: "Check token", About: "paste a token (ctrl+shift+v): whether it is accepted, and as whom",
			Input:   "token",
			Command: "kubectl create -f - # TokenReview of the token pasted",
			Run: func(c *k8s.Client, _, token string, _ []Flag) tea.Cmd {
				return Read("Check token", "", func(ctx context.Context) (string, error) {
					return c.CheckToken(ctx, token)
				})
			},
		}},
		OwnButton: "+ ask",
		OwnTitle:  "Check a token",
	}
}
