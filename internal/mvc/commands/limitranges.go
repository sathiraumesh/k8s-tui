package commands

import (
	"context"
	"strings"

	k8s "k8s-tui/internal/kubernetes"

	tea "github.com/charmbracelet/bubbletea"
)

// limitRangesView is LimitRanges in More: defaults and bounds for what
// the containers of a namespace ask for. Its own: making the defaults.
func limitRangesView() View {
	return View{
		Type:  k8s.ResourceType{Version: "v1", Resource: "limitranges", Kind: "LimitRange", Namespaced: true},
		Title: "limits",
		Own: []Action{{
			Key: "c", Name: "container defaults", About: "name cpu memory: defaults 500m 256Mi, for containers that ask for none",
			Changes:      true,
			Input:        "name cpu memory",
			InputDefault: "defaults 500m 256Mi",
			Flags:        []Flag{{Name: FlagDryRun}},
			Command:      "kubectl apply -n %s -f - # LimitRange of container defaults " + InputHere,
			Run: func(c *k8s.Client, namespace, input string, flags []Flag) tea.Cmd {
				f := append(strings.Fields(input), "", "", "")
				return Change("Create", namespace+"/"+f[0], flags, func(ctx context.Context) (string, error) {
					return "", c.CreateDefaultLimits(ctx, namespace, f[0], f[1], f[2], Ticked(flags, FlagDryRun))
				})
			},
		}},
		OwnButton:   "+ create",
		OwnTitle:    "Create container defaults",
		DeleteAbout: "remove it; pods made later get no defaults from it, those running keep theirs",
	}
}
