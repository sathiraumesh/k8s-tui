package commands

import (
	"context"
	"fmt"
	"strings"

	k8s "k8s-tui/internal/kubernetes"

	tea "github.com/charmbracelet/bubbletea"
)

// storageClassesView is StorageClasses in More: the kinds of storage the
// cluster can make volumes of, for claims. Its own: Make default.
func storageClassesView() View {
	t := k8s.ResourceType{Group: "storage.k8s.io", Version: "v1", Resource: "storageclasses", Kind: "StorageClass"}
	return View{
		Type:  t,
		Title: "sc",
		Actions: []Action{
			makeDefaultAction(t, "storageclass.kubernetes.io/is-default-class", "the class claims get when they name none"),
		},
		DeleteAbout: "remove it; volumes made with it stay, but claims naming it can't get new ones",
	}
}

// makeDefaultAction is Make default (m) of a kind marked default by
// annotation: kubectl patch TYPE NAME -p '{"metadata":{"annotations":{...}}}'.
func makeDefaultAction(t k8s.ResourceType, annotation, about string) Action {
	return Action{
		Key: "m", Name: "Make default", About: "make it " + about + "; the old default stops being it",
		Changes: true,
		Flags:   []Flag{{Name: FlagDryRun}},
		Command: fmt.Sprintf(`kubectl patch %s %%s -p '{"metadata":{"annotations":{"%s":"true"}}}'`, t.Arg(), annotation),
		Run: func(c *k8s.Client, target, _ string, flags []Flag) tea.Cmd {
			return Change("Make default", target, flags, func(ctx context.Context) (string, error) {
				was, err := c.MakeDefaultClass(ctx, t, target, annotation, Ticked(flags, FlagDryRun))
				if len(was) == 0 {
					return "", err
				}
				return "No longer the default: " + strings.Join(was, ", "), err
			})
		},
	}
}
