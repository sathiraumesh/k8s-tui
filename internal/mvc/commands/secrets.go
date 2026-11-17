package commands

import (
	"context"
	"fmt"
	"strings"

	k8s "k8s-tui/internal/kubernetes"

	tea "github.com/charmbracelet/bubbletea"
)

// secretsView is Secrets in More: passwords, tokens and keys for pods,
// base64 encoded (not encrypted). Its own: Decode, and making a generic or
// an image-pull secret.
func secretsView() View {
	return View{
		Type:  k8s.ResourceType{Version: "v1", Resource: "secrets", Kind: "Secret", Namespaced: true},
		Title: "secret",
		Actions: []Action{{
			Key: "a", Name: "Decode", About: "its entries, decoded from base64: each key and its value",
			Command: "kubectl get secrets %s -o jsonpath='{.data}' # each value | base64 -d",
			Run: func(c *k8s.Client, target, _ string, _ []Flag) tea.Cmd {
				namespace, name := SplitTarget(target)
				return Read("Decode", target, func(ctx context.Context) (string, error) {
					data, err := c.SecretData(ctx, namespace, name)
					return entriesText(data), err
				})
			},
		}},
		Own: []Action{
			{
				Key: "g", Name: "generic", About: "a name and its entries: db-login user=admin password=s3cret",
				Changes:   true,
				Input:     "name key=value ...",
				InputArgs: literalArgs,
				Flags:     []Flag{{Name: FlagDryRun}},
				Command:   "kubectl create secret generic " + InputHere + " -n %s",
				Run: func(c *k8s.Client, namespace, input string, flags []Flag) tea.Cmd {
					name, literals := nameAndRest(input)
					return Change("Create", namespace+"/"+name, flags, func(ctx context.Context) (string, error) {
						return "", c.CreateSecret(ctx, namespace, name, literals, Ticked(flags, FlagDryRun))
					})
				},
			},
			{
				Key: "d", Name: "docker-registry", About: "to pull images from a private registry: name server user password",
				Changes:   true,
				Input:     "name server user password",
				InputArgs: registryArgs,
				Flags:     []Flag{{Name: FlagDryRun}},
				Command:   "kubectl create secret docker-registry " + InputHere + " -n %s",
				Run: func(c *k8s.Client, namespace, input string, flags []Flag) tea.Cmd {
					f := append(strings.Fields(input), "", "", "", "")
					return Change("Create", namespace+"/"+f[0], flags, func(ctx context.Context) (string, error) {
						return "", c.CreateRegistrySecret(ctx, namespace, f[0], f[1], f[2], f[3], Ticked(flags, FlagDryRun))
					})
				},
			},
		},
		OwnButton: "+ create",
		OwnTitle:  "Create a secret",
	}
}

// registryArgs writes "regcred ghcr.io me pw" as kubectl takes it, the
// password hidden: "regcred --docker-server=ghcr.io --docker-username=me
// --docker-password=***".
func registryArgs(input string) string {
	f := strings.Fields(input)
	if len(f) < 4 {
		return input
	}
	return fmt.Sprintf("%s --docker-server=%s --docker-username=%s --docker-password=***", f[0], f[1], f[2])
}
