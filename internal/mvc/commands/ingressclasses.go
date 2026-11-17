package commands

import k8s "k8s-tui/internal/kubernetes"

// ingressClassesView is IngressClasses in More: which ingress controller
// handles an ingress. Its own: Make default.
func ingressClassesView() View {
	t := k8s.ResourceType{Group: "networking.k8s.io", Version: "v1", Resource: "ingressclasses", Kind: "IngressClass"}
	return View{
		Type:  t,
		Title: "ingressclass",
		Actions: []Action{
			makeDefaultAction(t, "ingressclass.kubernetes.io/is-default-class", "the class ingresses get when they name none"),
		},
	}
}
