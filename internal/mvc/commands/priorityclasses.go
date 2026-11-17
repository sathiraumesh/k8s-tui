package commands

import (
	"context"
	"strings"

	k8s "k8s-tui/internal/kubernetes"

	tea "github.com/charmbracelet/bubbletea"
)

// priorityClassesView is PriorityClasses in More: how important pods are
// when a node is full. Its own: making one.
func priorityClassesView() View {
	return View{
		Type:  k8s.ResourceType{Group: "scheduling.k8s.io", Version: "v1", Resource: "priorityclasses", Kind: "PriorityClass"},
		Title: "pc",
		Own: []Action{{
			Key: "c", Name: "priority class", About: "name value: high 1000 (higher pods may push out lower ones)",
			Changes: true,
			Input:   "name value",
			InputArgs: func(input string) string {
				name, rest := nameAndRest(input)
				if len(rest) != 1 {
					return input
				}
				return name + " --value=" + rest[0]
			},
			Flags:   []Flag{{Name: FlagDryRun}},
			Command: "kubectl create priorityclass " + InputHere,
			Run: func(c *k8s.Client, _, input string, flags []Flag) tea.Cmd {
				f := append(strings.Fields(input), "", "")
				return Change("Create", f[0], flags, func(ctx context.Context) (string, error) {
					return "", c.CreatePriorityClass(ctx, f[0], f[1], Ticked(flags, FlagDryRun))
				})
			},
		}},
		OwnButton:   "+ create",
		OwnTitle:    "Create a priority class",
		DeleteAbout: "remove it; pods already running keep their priority, new ones naming it are refused",
	}
}
