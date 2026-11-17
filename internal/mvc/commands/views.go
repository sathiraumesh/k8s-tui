package commands

import (
	"cmp"
	"context"
	"strings"
	"time"

	k8s "k8s-tui/internal/kubernetes"

	tea "github.com/charmbracelet/bubbletea"
)

// Views: the kinds of resource More opens with the generic calls
// (internal/kubernetes/generic.go). A view lists the kind as `kubectl get`
// prints it, with the server's own columns, and has the actions every kind
// has (Describe, YAML, Show, Edit, Label, Delete). Each kind adds what is
// its own: the column colored by status, more actions, and the actions to
// make one. Adding a kind is one View, in its own file, listed in Views.

// View is a kind of resource shown in More.
type View struct {
	Type  k8s.ResourceType
	Title string // how the tab bar shows it after "More › ", e.g. "cm"

	// Status is the column colored by how things are, if one is. Its
	// column must come before any whose cells can have spaces (a CronJob's
	// SCHEDULE), as the row is split at spaces to find it.
	Status *StatusColumn

	// Actions are the kind's own actions, listed before the generic ones.
	// One with the key of a generic action takes its place.
	Actions []Action

	// DeleteAbout, if set, says what Delete does to this kind, e.g. that
	// deleting a namespace deletes everything in it.
	DeleteAbout string

	// Own are the actions of the view itself (making one), with the button
	// that opens them and their title; nil for none.
	Own       []Action
	OwnButton string
	OwnTitle  string
}

// Views are the views More opens, one per kind.
func Views() []View {
	return []View{
		configMapsView(),
		secretsView(),
		namespacesView(),
		serviceAccountsView(),
		statefulSetsView(),
		daemonSetsView(),
		replicaSetsView(),
		jobsView(),
		cronJobsView(),
		ingressesView(),
		storageClassesView(),
		ingressClassesView(),
		networkPoliciesView(),
		horizontalPodAutoscalersView(),
		podDisruptionBudgetsView(),
		priorityClassesView(),
		resourceQuotasView(),
		limitRangesView(),
		eventsView(),
		eventsK8sIOView(),
		rolesView(),
		clusterRolesView(),
		roleBindingsView(),
		clusterRoleBindingsView(),
		endpointsView(),
		endpointSlicesView(),
		leasesView(),
		controllerRevisionsView(),
		podTemplatesView(),
		replicationControllersView(),
		componentStatusesView(),
		apiServicesView(),
		runtimeClassesView(),
		csiDriversView(),
	}
}

// Name is the view's name: its tab, and its rows' key. It is the type as
// kubectl takes it, e.g. "configmaps" or "statefulsets.apps".
func (v View) Name() string {
	return v.Type.Arg()
}

// Command is the kubectl command that lists the kind.
func (v View) Command() string {
	return "kubectl get " + v.Type.Arg()
}

// Load returns the view's loader: the kind's table from the server, each
// row with its namespace first while every namespace is shown.
func (v View) Load(c *k8s.Client, wide bool, namespace string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), APITimeout)
		defer cancel()

		table, err := c.ListTable(ctx, v.Type, namespace, wide)
		withNamespace := v.Type.Namespaced && namespace == AllNamespaces
		// A row's target is its first cells (NAMESPACE NAME); a kind whose
		// table doesn't start with the name, as events (LAST SEEN), gets it.
		withName := len(table.Columns) > 0 && table.Columns[0] != "NAME"
		cells := make([][]string, 0, len(table.Rows))
		for _, row := range table.Rows {
			r := row.Cells
			if withName {
				r = append([]string{row.Name}, r...)
			}
			if withNamespace {
				r = append([]string{row.Namespace}, r...)
			}
			cells = append(cells, r)
		}
		return ItemsLoadedMsg{Tab: v.Name(), Rows: AlignColumns(cells), Err: err, At: time.Now(), Wide: wide, Namespace: namespace}
	}
}

// RowTarget is what a row's actions are for: NAMESPACE/NAME for a kind in
// a namespace, else the name.
func (v View) RowTarget(row, namespace string) string {
	if v.Type.Namespaced {
		return NamespacedTarget(row, namespace)
	}
	return RowName(row)
}

// CommandTarget is the target as kubectl takes it: "NAME -n NAMESPACE"
// for a kind in a namespace, else the name.
func (v View) CommandTarget(target string) string {
	if v.Type.Namespaced {
		return NamespacedArgs(target)
	}
	return target
}

// split is the target's namespace and name.
func (v View) split(target string) (namespace, name string) {
	if v.Type.Namespaced {
		return SplitTarget(target)
	}
	return "", target
}

// AllActions are the view's actions: its own, then the generic ones whose
// keys it hasn't taken.
func (v View) AllActions() []Action {
	taken := map[string]bool{}
	for _, a := range v.Actions {
		taken[a.Key] = true
	}
	all := append([]Action{}, v.Actions...)
	for _, a := range v.genericActions() {
		if !taken[a.Key] {
			all = append(all, a)
		}
	}
	return all
}

// genericActions are the actions every kind has.
func (v View) genericActions() []Action {
	t := v.Type
	return []Action{
		{
			Key: "d", Name: "Describe", About: "everything about it, like kubectl describe",
			Flags:   []Flag{{Name: flagNoEvents}},
			Command: "kubectl describe " + t.Arg() + " %s",
			Run: func(c *k8s.Client, target, _ string, flags []Flag) tea.Cmd {
				namespace, name := v.split(target)
				return Read("Describe", target, func(ctx context.Context) (string, error) {
					return c.Describe(ctx, t, namespace, name, !Ticked(flags, flagNoEvents))
				})
			},
		},
		{
			Key: "y", Name: "YAML", About: "the whole object, as kubectl get -o yaml prints it",
			Command: "kubectl get " + t.Arg() + " %s -o yaml",
			Run: func(c *k8s.Client, target, _ string, _ []Flag) tea.Cmd {
				namespace, name := v.split(target)
				return Read("YAML", target, func(ctx context.Context) (string, error) {
					return c.YAML(ctx, t, namespace, name)
				})
			},
		},
		{
			Key: "s", Name: "Show", About: "its labels and annotations",
			Command: "kubectl get " + t.Arg() + " %s --show-labels",
			Run: func(c *k8s.Client, target, _ string, _ []Flag) tea.Cmd {
				namespace, name := v.split(target)
				return Read("Show", target, func(ctx context.Context) (string, error) {
					labels, annotations, err := c.ResourceLabelsAndAnnotations(ctx, t, namespace, name)
					return SectionsText(Section{"Labels", labels}, Section{"Annotations", annotations}), err
				})
			},
		},
		{
			Key: "e", Name: "Edit", About: "its YAML in your editor; saving applies it",
			Command: "kubectl edit " + t.Arg() + " %s",
			Run: func(c *k8s.Client, target, _ string, _ []Flag) tea.Cmd {
				namespace, name := v.split(target)
				cmd, err := c.EditResource(t, namespace, name)
				return InTerminal("Edit", target, "Edit "+target+": editor closed", false, cmd, err)
			},
		},
		{
			Key: "l", Name: "Label", About: "set labels (key=value) or remove them (key-), several with spaces",
			Changes: true,
			Input:   "labels",
			Flags:   []Flag{{Name: FlagOverwrite}, {Name: FlagDryRun}},
			Command: "kubectl label " + t.Arg() + " %s",
			Run: func(c *k8s.Client, target, input string, flags []Flag) tea.Cmd {
				namespace, name := v.split(target)
				return Change("Label", target, flags, func(ctx context.Context) (string, error) {
					ch, err := c.LabelResource(ctx, t, namespace, name, strings.Fields(input), Ticked(flags, FlagOverwrite), Ticked(flags, FlagDryRun))
					return ChangeText("Labels", ch.Before, ch.After), err
				})
			},
		},
		{
			Key: "r", Name: "Delete", About: cmp.Or(v.DeleteAbout, "remove it"),
			Changes: true,
			Flags:   []Flag{{Name: FlagDryRun}},
			Command: "kubectl delete " + t.Arg() + " %s",
			Run: func(c *k8s.Client, target, _ string, flags []Flag) tea.Cmd {
				namespace, name := v.split(target)
				return Change("Delete", target, flags, func(ctx context.Context) (string, error) {
					return "", c.DeleteResource(ctx, t, namespace, name, Ticked(flags, FlagDryRun))
				})
			},
		},
	}
}

// ViewFor is the view of the kind apiVersion/kind, if there is one, in that
// version: a view is for its group and kind, and the cluster may serve
// another version than the one it was written for (a v1beta1 before it
// became v1).
func ViewFor(apiVersion, kind string) (View, bool) {
	group, version, ok := strings.Cut(apiVersion, "/")
	if !ok {
		group, version = "", apiVersion // the core group: "v1"
	}
	for _, v := range Views() {
		if v.Type.Group == group && v.Type.Kind == kind {
			v.Type.Version = version
			return v, true
		}
	}
	return View{}, false
}
