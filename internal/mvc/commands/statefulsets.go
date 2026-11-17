package commands

import (
	"context"
	"strings"

	k8s "k8s-tui/internal/kubernetes"

	tea "github.com/charmbracelet/bubbletea"
)

// statefulSetsView is StatefulSets in More: pods with fixed names and each
// its own storage, e.g. a database. Its own: READY colored, Scale,
// Restart and Set image, as for a deployment.
func statefulSetsView() View {
	return View{
		Type:   k8s.ResourceType{Group: "apps", Version: "v1", Resource: "statefulsets", Kind: "StatefulSet", Namespaced: true},
		Title:  "sts",
		Status: &ReadyColumn,
		Actions: []Action{
			scaleAction("statefulsets.apps", "scale down removes the highest-numbered pods; their storage stays",
				(*k8s.Client).ScaleStatefulSet),
			restartAction("statefulsets.apps", "replace its pods one at a time, the highest first",
				(*k8s.Client).RestartStatefulSet),
			setImageAction("statefulset", "its pods are then replaced in order",
				(*k8s.Client).StatefulSetContainers, (*k8s.Client).SetStatefulSetImage),
		},
		DeleteAbout: "remove it and its pods; their storage (claims) stays",
	}
}

// ReadyColumn is a READY column right after the name, e.g. "2/3": all
// ready green, some yellow, none red (see ReadyLevel).
var ReadyColumn = StatusColumn{Column: 1, Level: ReadyLevel}

// The actions the kinds that run pods share, made for one kind: its name
// as kubectl takes it, a word on what it does to that kind, and its call.

// scaleAction is Scale (c): kubectl scale TYPE NAME --replicas=N.
func scaleAction(arg, about string, scale func(*k8s.Client, context.Context, string, string, string, bool) (k8s.Change, error)) Action {
	return Action{
		Key: "c", Name: "Scale", About: "set how many pods it runs; " + about,
		Changes:   true,
		Input:     "replicas",
		InputArgs: func(n string) string { return "--replicas=" + strings.TrimSpace(n) },
		Flags:     []Flag{{Name: FlagDryRun}},
		Command:   "kubectl scale " + arg + " %s",
		Run: func(c *k8s.Client, target, input string, flags []Flag) tea.Cmd {
			namespace, name := SplitTarget(target)
			return Change("Scale", target, flags, func(ctx context.Context) (string, error) {
				ch, err := scale(c, ctx, namespace, name, input, Ticked(flags, FlagDryRun))
				return ChangeText("Replicas", ch.Before, ch.After), err
			})
		},
	}
}

// restartAction is Restart (t): kubectl rollout restart TYPE NAME.
func restartAction(arg, about string, restart func(*k8s.Client, context.Context, string, string, bool) error) Action {
	return Action{
		Key: "t", Name: "Restart", About: about + ", like kubectl rollout restart",
		Changes: true,
		Flags:   []Flag{{Name: FlagDryRun}},
		Command: "kubectl rollout restart " + arg + " %s",
		Run: func(c *k8s.Client, target, _ string, flags []Flag) tea.Cmd {
			namespace, name := SplitTarget(target)
			return Change("Restart", target, flags, func(ctx context.Context) (string, error) {
				return "", restart(c, ctx, namespace, name, Ticked(flags, FlagDryRun))
			})
		},
	}
}

// setImageAction is Set image (i): kubectl set image KIND/NAME C=IMAGE,
// the container picked first.
func setImageAction(kind, about string, containers ContainerLister, set func(*k8s.Client, context.Context, string, string, []string, bool) (k8s.Change, error)) Action {
	return Action{
		Key: "i", Name: "Set image", About: "change a container's image; " + about,
		Changes:    true,
		Input:      "images (CONTAINER=IMAGE)",
		Containers: containers,
		Flags:      []Flag{{Name: FlagDryRun}},
		Command:    "kubectl set image " + kind + "/%s",
		Run: func(c *k8s.Client, target, input string, flags []Flag) tea.Cmd {
			namespace, name := SplitTarget(target)
			return Change("Set image", target, flags, func(ctx context.Context) (string, error) {
				ch, err := set(c, ctx, namespace, name, strings.Fields(input), Ticked(flags, FlagDryRun))
				return ChangeText("Images", ch.Before, ch.After), err
			})
		},
	}
}
