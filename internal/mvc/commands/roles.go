package commands

import (
	"context"
	"strings"

	k8s "k8s-tui/internal/kubernetes"

	tea "github.com/charmbracelet/bubbletea"
)

// rolesView is Roles in More: what may be done to which resources, in one
// namespace; a role binding gives a role to someone. Its own: making one.
func rolesView() View {
	return View{
		Type:      k8s.ResourceType{Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "roles", Kind: "Role", Namespaced: true},
		Title:     "role",
		Own:       []Action{roleAction("role", " -n %s", true)},
		OwnButton: "+ create",
		OwnTitle:  "Create a role",
	}
}

// roleAction makes a Role (namespaced) or a ClusterRole: "name verbs
// resources", as kubectl create role (or clusterrole) takes them.
func roleAction(kind, inNamespace string, namespaced bool) Action {
	return Action{
		Key: "c", Name: kind, About: "name verbs resources: reader get,list,watch pods,deployments",
		Changes:   true,
		Input:     "name verbs resources",
		InputArgs: roleArgs,
		Flags:     []Flag{{Name: FlagDryRun}},
		Command:   "kubectl create " + kind + " " + InputHere + inNamespace,
		Run: func(c *k8s.Client, namespace, input string, flags []Flag) tea.Cmd {
			f := append(strings.Fields(input), "", "", "")
			if !namespaced {
				namespace = ""
			}
			target := f[0]
			if namespaced {
				target = namespace + "/" + f[0]
			}
			return Change("Create", target, flags, func(ctx context.Context) (string, error) {
				return "", c.CreateRole(ctx, namespace, f[0], splitList(f[1]), splitList(f[2]), Ticked(flags, FlagDryRun))
			})
		},
	}
}

// roleArgs writes "reader get,list pods" as kubectl takes it:
// "reader --verb=get,list --resource=pods".
func roleArgs(input string) string {
	f := strings.Fields(input)
	if len(f) != 3 {
		return input
	}
	return f[0] + " --verb=" + f[1] + " --resource=" + f[2]
}

// splitList is a comma-separated list's items, none empty.
func splitList(list string) []string {
	var out []string
	for _, s := range strings.Split(list, ",") {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return out
}
