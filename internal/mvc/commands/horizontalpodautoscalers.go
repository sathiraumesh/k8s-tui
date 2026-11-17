package commands

import (
	"context"
	"fmt"
	"strings"

	k8s "k8s-tui/internal/kubernetes"

	tea "github.com/charmbracelet/bubbletea"
)

// horizontalPodAutoscalersView is HorizontalPodAutoscalers in More: a
// deployment's replicas set by how busy its pods are. Its own: making one
// for a deployment.
func horizontalPodAutoscalersView() View {
	return View{
		Type:  k8s.ResourceType{Group: "autoscaling", Version: "v2", Resource: "horizontalpodautoscalers", Kind: "HorizontalPodAutoscaler", Namespaced: true},
		Title: "hpa",
		Own: []Action{{
			Key: "a", Name: "for a deployment", About: "deployment min max cpu%: web 1 5 80; needs metrics-server and CPU requests",
			Changes:   true,
			Input:     "deployment min max cpu%",
			InputArgs: autoscaleArgs,
			Flags:     []Flag{{Name: FlagDryRun}},
			Command:   "kubectl autoscale deployment " + InputHere + " -n %s",
			Run: func(c *k8s.Client, namespace, input string, flags []Flag) tea.Cmd {
				f := append(strings.Fields(input), "", "", "", "")
				return Change("Autoscale", namespace+"/"+f[0], flags, func(ctx context.Context) (string, error) {
					return "", c.Autoscale(ctx, namespace, f[0], f[1], f[2], f[3], Ticked(flags, FlagDryRun))
				})
			},
		}},
		OwnButton:   "+ create",
		OwnTitle:    "Autoscale a deployment",
		DeleteAbout: "remove it; the deployment keeps the replicas it has now",
	}
}

// autoscaleArgs writes "web 1 5 80" as kubectl takes it:
// "web --min=1 --max=5 --cpu=80%".
func autoscaleArgs(input string) string {
	f := strings.Fields(input)
	if len(f) != 4 {
		return input
	}
	return fmt.Sprintf("%s --min=%s --max=%s --cpu=%s%%", f[0], f[1], f[2], strings.TrimSuffix(f[3], "%"))
}
