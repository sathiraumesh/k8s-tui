package commands

import (
	"context"
	"strings"

	k8s "k8s-tui/internal/kubernetes"

	tea "github.com/charmbracelet/bubbletea"
)

// subjectAccessReviewsView is SubjectAccessReviews in More: asking whether
// someone else may do something. Its own: Can they.
func subjectAccessReviewsView() View {
	return View{
		Type:       k8s.ResourceType{Group: "authorization.k8s.io", Version: "v1", Resource: "subjectaccessreviews", Kind: "SubjectAccessReview"},
		Title:      "can-they",
		CreateOnly: "Nothing to list: a review is a question to the API server. Press + to ask whether someone may do something.",
		Own:        []Action{canTheyAction(false)},
		OwnButton:  "+ ask",
		OwnTitle:   "Ask whether they may",
	}
}

// canTheyAction asks whether someone may do something: everywhere (or in
// the namespace typed), or, local, in the namespace picked, with a
// LocalSubjectAccessReview.
func canTheyAction(local bool) Action {
	about, input := "who verb resource [namespace]: user:alice create deployments apps (or group:, serviceaccount:NS:NAME)", "who verb resource [namespace]"
	if local {
		about, input = "who verb resource, in this namespace: serviceaccount:apps:ci create deployments", "who verb resource"
	}
	return Action{
		Key: "c", Name: "Can they", About: about,
		Input:     input,
		InputArgs: canTheyArgs,
		Command:   "kubectl auth can-i " + InputHere,
		Run: func(c *k8s.Client, namespace, in string, _ []Flag) tea.Cmd {
			f := append(strings.Fields(in), "", "", "", "")
			if !local {
				namespace = f[3]
			}
			return Read("Can they", strings.TrimSpace(in), func(ctx context.Context) (string, error) {
				return c.CanThey(ctx, f[0], f[1], f[2], namespace, local)
			})
		},
	}
}

// canTheyArgs writes "user:alice create pods apps" as kubectl takes it:
// "create pods -n apps --as=alice".
func canTheyArgs(input string) string {
	f := strings.Fields(input)
	if len(f) < 3 {
		return input
	}
	args := f[1] + " " + f[2]
	if len(f) > 3 {
		args += " -n " + f[3]
	}
	kind, name, _ := strings.Cut(f[0], ":")
	switch kind {
	case "group":
		return args + " --as=someone --as-group=" + name
	case "serviceaccount":
		return args + " --as=system:serviceaccount:" + name
	}
	return args + " --as=" + name
}
