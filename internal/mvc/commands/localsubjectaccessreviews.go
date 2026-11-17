package commands

import k8s "k8s-tui/internal/kubernetes"

// localSubjectAccessReviewsView is LocalSubjectAccessReviews in More:
// asking whether someone may do something in one namespace; a namespace's
// own admins may ask it. Its own: Can they, in the namespace picked.
func localSubjectAccessReviewsView() View {
	return View{
		Type:       k8s.ResourceType{Group: "authorization.k8s.io", Version: "v1", Resource: "localsubjectaccessreviews", Kind: "LocalSubjectAccessReview", Namespaced: true},
		Title:      "can-they-here",
		CreateOnly: "Nothing to list: a review is a question to the API server. Press + to ask whether someone may do something in this namespace.",
		Own:        []Action{canTheyAction(true)},
		OwnButton:  "+ ask",
		OwnTitle:   "Ask whether they may here",
	}
}
