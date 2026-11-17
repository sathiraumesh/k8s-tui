package commands

import (
	"context"
	"fmt"
	"strings"

	k8s "k8s-tui/internal/kubernetes"

	tea "github.com/charmbracelet/bubbletea"
)

// jobsView is Jobs in More: pods that run until they are done, e.g. a
// backup or a migration. Its own: STATUS colored, Logs of its pods,
// Suspend and Resume, and making one.
func jobsView() View {
	return View{
		Type:   k8s.ResourceType{Group: "batch", Version: "v1", Resource: "jobs", Kind: "Job", Namespaced: true},
		Title:  "job",
		Status: &JobStatus,
		Actions: []Action{
			{
				Key: "o", Name: "Logs", About: fmt.Sprintf("the last %d lines of each of its pods, oldest first", logTail),
				Command: fmt.Sprintf("kubectl logs job/%%s --all-containers --tail=%d", logTail),
				Run: func(c *k8s.Client, target, _ string, _ []Flag) tea.Cmd {
					namespace, name := SplitTarget(target)
					return ReadToEnd("Logs", target, func(ctx context.Context) (string, error) {
						return c.JobLogs(ctx, namespace, name, logTail)
					})
				},
			},
			suspendAction("job", "Suspend", "stop its pods and start no more", true, (*k8s.Client).SuspendJob),
			suspendAction("job", "Resume", "let it go on", false, (*k8s.Client).SuspendJob),
		},
		Own: []Action{{
			Key: "c", Name: "job", About: "name image [command ...]: hello busybox echo hi (no quotes: words split at spaces)",
			Changes:   true,
			Input:     "name image [command ...]",
			InputArgs: jobArgs,
			Flags:     []Flag{{Name: FlagDryRun}},
			Command:   "kubectl create job " + InputHere + " -n %s",
			Run: func(c *k8s.Client, namespace, input string, flags []Flag) tea.Cmd {
				f := strings.Fields(input)
				name, image := "", ""
				if len(f) > 0 {
					name = f[0]
				}
				if len(f) > 1 {
					image = f[1]
				}
				var command []string
				if len(f) > 2 {
					command = f[2:]
				}
				return Change("Create", namespace+"/"+name, flags, func(ctx context.Context) (string, error) {
					return "", c.CreateJob(ctx, namespace, name, image, command, Ticked(flags, FlagDryRun))
				})
			},
		}},
		OwnButton:   "+ create",
		OwnTitle:    "Create a job",
		DeleteAbout: "remove it and its pods",
	}
}

// JobStatus is the STATUS column of the jobs: NAME STATUS. Complete is
// fine; Running and Suspended are on their way; Failed is broken.
var JobStatus = StatusColumn{Column: 1, Level: func(status string) Level {
	switch status {
	case "Complete", "SuccessCriteriaMet":
		return LevelOK
	case "Running", "Suspended", "Terminating":
		return LevelWarn
	case "Failed", "FailureTarget":
		return LevelBad
	}
	return LevelNone
}}

// suspendAction is Suspend (p) or Resume (u) of a kind with spec.suspend:
// a job or a cronjob.
func suspendAction(kind, name, about string, suspend bool, set func(*k8s.Client, context.Context, string, string, bool, bool) error) Action {
	key := "u"
	if suspend {
		key = "p"
	}
	return Action{
		Key: key, Name: name, About: about,
		Changes: true,
		Flags:   []Flag{{Name: FlagDryRun}},
		Command: fmt.Sprintf(`kubectl patch %s %%s -p '{"spec":{"suspend":%t}}'`, kind, suspend),
		Run: func(c *k8s.Client, target, _ string, flags []Flag) tea.Cmd {
			namespace, n := SplitTarget(target)
			return Change(name, target, flags, func(ctx context.Context) (string, error) {
				return "", set(c, ctx, namespace, n, suspend, Ticked(flags, FlagDryRun))
			})
		},
	}
}

// jobArgs writes "pi perl perl -e 1" as kubectl takes it:
// "pi --image=perl -- perl -e 1".
func jobArgs(input string) string {
	f := strings.Fields(input)
	if len(f) < 2 {
		return input
	}
	args := f[0] + " --image=" + f[1]
	if len(f) > 2 {
		args += " -- " + strings.Join(f[2:], " ")
	}
	return args
}
