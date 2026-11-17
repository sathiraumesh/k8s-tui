package commands

import (
	"context"
	"fmt"
	"strings"

	k8s "k8s-tui/internal/kubernetes"

	tea "github.com/charmbracelet/bubbletea"
)

// configMapsView is ConfigMaps in More: settings and files for pods, as
// key=value entries. Its own: Data, and making one from entries.
func configMapsView() View {
	return View{
		Type:  k8s.ResourceType{Version: "v1", Resource: "configmaps", Kind: "ConfigMap", Namespaced: true},
		Title: "cm",
		Actions: []Action{{
			Key: "a", Name: "Data", About: "its entries: each key and its value",
			Command: "kubectl get configmaps %s -o jsonpath='{.data}'",
			Run: func(c *k8s.Client, target, _ string, _ []Flag) tea.Cmd {
				namespace, name := SplitTarget(target)
				return Read("Data", target, func(ctx context.Context) (string, error) {
					data, err := c.ConfigMapData(ctx, namespace, name)
					return entriesText(data), err
				})
			},
		}},
		Own: []Action{{
			Key: "c", Name: "from entries", About: "a name and its entries: app-config mode=fast level=3",
			Changes:   true,
			Input:     "name key=value ...",
			InputArgs: literalArgs,
			Flags:     []Flag{{Name: FlagDryRun}},
			Command:   "kubectl create configmap " + InputHere + " -n %s",
			Run: func(c *k8s.Client, namespace, input string, flags []Flag) tea.Cmd {
				name, literals := nameAndRest(input)
				return Change("Create", namespace+"/"+name, flags, func(ctx context.Context) (string, error) {
					return "", c.CreateConfigMap(ctx, namespace, name, literals, Ticked(flags, FlagDryRun))
				})
			},
		}},
		OwnButton: "+ create",
		OwnTitle:  "Create a config map",
	}
}

// entriesText is a config map's or secret's entries, each key with its
// value under it, indented:
//
//	mode:
//	  fast
//	config.yaml:
//	  a: 1
//	  b: 2
func entriesText(data []k8s.KeyValue) string {
	if len(data) == 0 {
		return "(no entries)"
	}
	var b strings.Builder
	for _, kv := range data {
		fmt.Fprintf(&b, "%s:\n", kv.Key)
		for _, line := range strings.Split(strings.TrimRight(kv.Value, "\n"), "\n") {
			b.WriteString("  " + line + "\n")
		}
	}
	return b.String()
}

// nameAndRest splits what was typed into its first word and the others.
func nameAndRest(input string) (name string, rest []string) {
	f := strings.Fields(input)
	if len(f) == 0 {
		return "", nil
	}
	return f[0], f[1:]
}

// literalArgs writes "app-config mode=fast" as kubectl takes it:
// "app-config --from-literal=mode=fast".
func literalArgs(input string) string {
	name, literals := nameAndRest(input)
	parts := []string{name}
	for _, l := range literals {
		parts = append(parts, "--from-literal="+l)
	}
	return strings.Join(parts, " ")
}
