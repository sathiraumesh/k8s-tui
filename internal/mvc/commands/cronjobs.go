package commands

import (
	"context"
	"strings"

	k8s "k8s-tui/internal/kubernetes"

	tea "github.com/charmbracelet/bubbletea"
)

// cronJobsView is CronJobs in More: jobs made on a schedule. Its own: Run
// now, Suspend and Resume, and making one.
func cronJobsView() View {
	return View{
		Type:  k8s.ResourceType{Group: "batch", Version: "v1", Resource: "cronjobs", Kind: "CronJob", Namespaced: true},
		Title: "cj",
		Actions: []Action{
			{
				Key: "n", Name: "Run now", About: "make a job from it now, without waiting for its schedule",
				Changes: true,
				Flags:   []Flag{{Name: FlagDryRun}},
				Command: "kubectl create job --from=cronjob/%s",
				Run: func(c *k8s.Client, target, _ string, flags []Flag) tea.Cmd {
					namespace, name := SplitTarget(target)
					return Change("Run now", target, flags, func(ctx context.Context) (string, error) {
						job, err := c.RunCronJob(ctx, namespace, name, Ticked(flags, FlagDryRun))
						return "Job " + namespace + "/" + job + ": see it in More › job", err
					})
				},
			},
			suspendAction("cronjob", "Suspend", "make no more jobs; those running go on", true, (*k8s.Client).SuspendCronJob),
			suspendAction("cronjob", "Resume", "make jobs on its schedule again", false, (*k8s.Client).SuspendCronJob),
		},
		Own: []Action{{
			Key: "c", Name: "cronjob", About: "name image schedule [command ...]: backup busybox */5 * * * * echo hi (or @hourly)",
			Changes:   true,
			Input:     "name image schedule [command ...]",
			InputArgs: cronJobArgs,
			Flags:     []Flag{{Name: FlagDryRun}},
			Command:   "kubectl create cronjob " + InputHere + " -n %s",
			Run: func(c *k8s.Client, namespace, input string, flags []Flag) tea.Cmd {
				name, image, schedule, command := splitCronJob(input)
				return Change("Create", namespace+"/"+name, flags, func(ctx context.Context) (string, error) {
					return "", c.CreateCronJob(ctx, namespace, name, image, schedule, command, Ticked(flags, FlagDryRun))
				})
			},
		}},
		OwnButton:   "+ create",
		OwnTitle:    "Create a cronjob",
		DeleteAbout: "remove it and the jobs it made",
	}
}

// splitCronJob reads "NAME IMAGE SCHEDULE [COMMAND ...]": the schedule is
// one word if it starts with @ (@hourly), else the five fields of cron.
func splitCronJob(input string) (name, image, schedule string, command []string) {
	f := strings.Fields(input)
	if len(f) < 3 {
		if len(f) > 0 {
			name = f[0]
		}
		if len(f) > 1 {
			image = f[1]
		}
		return name, image, "", nil
	}
	name, image, rest := f[0], f[1], f[2:]
	n := 5
	if strings.HasPrefix(rest[0], "@") {
		n = 1
	}
	if len(rest) < n {
		return name, image, "", nil
	}
	return name, image, strings.Join(rest[:n], " "), rest[n:]
}

// cronJobArgs writes what was typed as kubectl takes it:
// "backup --image=busybox --schedule='*/5 * * * *' -- echo hi".
func cronJobArgs(input string) string {
	name, image, schedule, command := splitCronJob(input)
	if schedule == "" {
		return input
	}
	args := name + " --image=" + image + " --schedule='" + schedule + "'"
	if len(command) > 0 {
		args += " -- " + strings.Join(command, " ")
	}
	return args
}
