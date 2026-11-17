package commands

import (
	"context"
	"strings"

	k8s "k8s-tui/internal/kubernetes"

	tea "github.com/charmbracelet/bubbletea"
)

// roleBindingsView is RoleBindings in More: a role (or cluster role) given
// to users, groups or service accounts, in one namespace. Its own: making
// one.
func roleBindingsView() View {
	return View{
		Type:      k8s.ResourceType{Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "rolebindings", Kind: "RoleBinding", Namespaced: true},
		Title:     "rolebinding",
		Own:       []Action{bindingAction("rolebinding", " -n %s", true)},
		OwnButton: "+ create",
		OwnTitle:  "Create a role binding",
	}
}

// bindingAction makes a RoleBinding (namespaced) or a ClusterRoleBinding:
// "name role subject ...", e.g. "deployers clusterrole:edit
// serviceaccount:apps:deployer".
func bindingAction(kind, inNamespace string, namespaced bool) Action {
	about := "name role:R|clusterrole:R subject...: viewers clusterrole:view user:alice group:devs serviceaccount:apps:ci"
	if !namespaced {
		about = "name clusterrole:R subject...: admins clusterrole:cluster-admin user:alice"
	}
	return Action{
		Key: "c", Name: kind, About: about,
		Changes:   true,
		Input:     "name role subject ...",
		InputArgs: bindingArgs,
		Flags:     []Flag{{Name: FlagDryRun}},
		Command:   "kubectl create " + kind + " " + InputHere + inNamespace,
		Run: func(c *k8s.Client, namespace, input string, flags []Flag) tea.Cmd {
			name, rest := nameAndRest(input) // rest: the role, then the subjects
			role, subjects := "", []string(nil)
			if len(rest) > 0 {
				role, subjects = rest[0], rest[1:]
			}
			if !namespaced {
				namespace = ""
			}
			target := name
			if namespaced {
				target = namespace + "/" + name
			}
			return Change("Create", target, flags, func(ctx context.Context) (string, error) {
				return "", c.CreateBinding(ctx, namespace, name, role, subjects, Ticked(flags, FlagDryRun))
			})
		},
	}
}

// bindingArgs writes "viewers clusterrole:view user:alice" as kubectl
// takes it: "viewers --clusterrole=view --user=alice".
func bindingArgs(input string) string {
	f := strings.Fields(input)
	if len(f) < 3 {
		return input
	}
	parts := []string{f[0]}
	for _, kv := range f[1:] {
		kind, name, ok := strings.Cut(kv, ":")
		if !ok {
			return input
		}
		parts = append(parts, "--"+kind+"="+name)
	}
	return strings.Join(parts, " ")
}
