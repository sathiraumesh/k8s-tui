package commands

import (
	"context"
	"fmt"
	"strings"
	"time"

	k8s "k8s-tui/internal/kubernetes"

	tea "github.com/charmbracelet/bubbletea"
)

// Commands for persistent volume claims (PVCs), opened from More: requests
// for storage in a namespace, each bound to a persistent volume
// (persistentvolumes.go). The cluster calls are in
// internal/kubernetes/persistentvolumeclaims.go.

// TabPersistentVolumeClaims is the name of the claims' view in More.
const TabPersistentVolumeClaims = "persistentvolumeclaims"

// LoadPersistentVolumeClaims calls the API for the claims (like `kubectl get
// persistentvolumeclaims --all-namespaces`, or with wide, `-o wide`).
func LoadPersistentVolumeClaims(c *k8s.Client, wide bool, namespace string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), APITimeout)
		defer cancel()

		claims, err := c.ListPersistentVolumeClaims(ctx, namespace) // "": all namespaces
		now := time.Now()
		rows := persistentVolumeClaimRows(claims, wide, namespace == AllNamespaces, now)
		return ItemsLoadedMsg{Tab: TabPersistentVolumeClaims, Rows: rows, Err: err, At: now, Wide: wide, Namespace: namespace}
	}
}

// persistentVolumeClaimRows turns each claim into one line, with its
// columns lined up, as `kubectl get persistentvolumeclaims -A` shows them:
//
//	NAMESPACE  NAME  STATUS  VOLUME  CAPACITY  ACCESS-MODES  STORAGECLASS  AGE
//
// A claim that isn't bound yet has "<none>" for its volume, capacity and
// access modes, so the columns stay in their places. With withNamespace
// false NAMESPACE is left out; with wide, VOLUMEMODE comes after.
func persistentVolumeClaimRows(claims []k8s.PersistentVolumeClaim, wide, withNamespace bool, now time.Time) []string {
	cells := make([][]string, 0, len(claims))
	for _, c := range claims {
		row := []string{c.Name, orNone(c.Status), orNone(c.Volume), orNone(c.Capacity), orNone(c.AccessModes), orNone(c.StorageClass), age(c.CreatedAt, now)}
		if withNamespace {
			row = append([]string{c.Namespace}, row...)
		}
		if wide {
			row = append(row, c.VolumeMode)
		}
		cells = append(cells, row)
	}
	return AlignColumns(cells)
}

// PersistentVolumeClaimStatus is the STATUS column of the claims: NAME
// STATUS. Bound is fine; Pending (no volume yet, or waiting for a pod with
// WaitForFirstConsumer) is on the way; Lost (its volume is gone) is broken.
var PersistentVolumeClaimStatus = StatusColumn{Column: 1, Level: func(status string) Level {
	switch status {
	case "Bound":
		return LevelOK
	case "Pending", "Terminating":
		return LevelWarn
	case "Lost":
		return LevelBad
	}
	return LevelNone
}}

// PersistentVolumeClaimActions are the actions on a claim: enter on one
// opens them. The keys are those of the other tabs for the same action.
func PersistentVolumeClaimActions() []Action {
	return []Action{
		{
			Key: "d", Name: "Describe", About: "everything about the claim: its volume, the pods using it, events",
			Flags:   []Flag{{Name: flagNoEvents}},
			Command: "kubectl describe persistentvolumeclaim %s",
			Run: func(c *k8s.Client, target, _ string, flags []Flag) tea.Cmd {
				namespace, name := SplitTarget(target)
				return Read("Describe", target, func(ctx context.Context) (string, error) {
					return c.DescribePersistentVolumeClaim(ctx, namespace, name, !Ticked(flags, flagNoEvents))
				})
			},
		},
		{
			Key: "s", Name: "Show", About: "the claim's labels and annotations",
			Command: "kubectl get persistentvolumeclaim %s --show-labels",
			Run: func(c *k8s.Client, target, _ string, _ []Flag) tea.Cmd {
				namespace, name := SplitTarget(target)
				return Read("Show", target, func(ctx context.Context) (string, error) {
					labels, annotations, err := c.PersistentVolumeClaimLabelsAndAnnotations(ctx, namespace, name)
					return SectionsText(Section{"Labels", labels}, Section{"Annotations", annotations}), err
				})
			},
		},
		{
			Key: "u", Name: "Used by", About: "the pods that mount the claim; while one does, it can't be deleted for good",
			Command: "kubectl describe persistentvolumeclaim %s | grep 'Used By'",
			Run: func(c *k8s.Client, target, _ string, _ []Flag) tea.Cmd {
				namespace, name := SplitTarget(target)
				return Read("Used by", target, func(ctx context.Context) (string, error) {
					pods, err := c.PersistentVolumeClaimUsers(ctx, namespace, name)
					return SectionsText(Section{"Pods using it", pods}), err
				})
			},
		},
		{
			Key: "c", Name: "Resize", About: "ask for more storage, e.g. 2Gi; only to grow, if its storage class allows it",
			Changes:   true,
			Input:     "size",
			InputArgs: resizePatch,
			Flags:     []Flag{{Name: FlagDryRun}},
			Command:   "kubectl patch persistentvolumeclaim %s",
			Run: func(c *k8s.Client, target, size string, flags []Flag) tea.Cmd {
				namespace, name := SplitTarget(target)
				return Change("Resize", target, flags, func(ctx context.Context) (string, error) {
					ch, err := c.ResizePersistentVolumeClaim(ctx, namespace, name, size, Ticked(flags, FlagDryRun))
					return ChangeText("Request", ch.Before, ch.After), err
				})
			},
		},
		{
			Key: "e", Name: "Edit", About: "the claim's YAML in your editor; little of a bound claim can change",
			Command: "kubectl edit persistentvolumeclaim %s",
			Run: func(c *k8s.Client, target, _ string, _ []Flag) tea.Cmd {
				namespace, name := SplitTarget(target)
				cmd, err := c.EditPersistentVolumeClaim(namespace, name)
				return InTerminal("Edit", target, "Edit "+target+": editor closed", false, cmd, err)
			},
		},
		{
			Key: "l", Name: "Label", About: "set labels (key=value) or remove them (key-), several with spaces",
			Changes: true,
			Input:   "labels",
			Flags:   []Flag{{Name: FlagOverwrite}, {Name: FlagDryRun}},
			Command: "kubectl label persistentvolumeclaim %s",
			Run: func(c *k8s.Client, target, input string, flags []Flag) tea.Cmd {
				namespace, name := SplitTarget(target)
				return Change("Label", target, flags, func(ctx context.Context) (string, error) {
					ch, err := c.LabelPersistentVolumeClaim(ctx, namespace, name, strings.Fields(input), Ticked(flags, FlagOverwrite), Ticked(flags, FlagDryRun))
					return ChangeText("Labels", ch.Before, ch.After), err
				})
			},
		},
		{
			Key: "r", Name: "Delete", About: "remove the claim; its volume then goes by its reclaim policy (Delete or Retain)",
			Changes: true,
			Flags:   []Flag{{Name: FlagDryRun}},
			Command: "kubectl delete persistentvolumeclaim %s",
			Run: func(c *k8s.Client, target, _ string, flags []Flag) tea.Cmd {
				namespace, name := SplitTarget(target)
				return Change("Delete", target, flags, func(ctx context.Context) (string, error) {
					return "", c.DeletePersistentVolumeClaim(ctx, namespace, name, Ticked(flags, FlagDryRun))
				})
			},
		},
	}
}

// resizePatch writes the size as kubectl patch takes it.
func resizePatch(size string) string {
	return fmt.Sprintf(`-p '{"spec":{"resources":{"requests":{"storage":"%s"}}}}'`, strings.TrimSpace(size))
}

// CreatePersistentVolumeClaimActions are the own actions of the claims:
// making one, in the namespace picked (or default). What is typed is a
// name, a size and, if not the default, a storage class: "data 1Gi", "data
// 1Gi standard", or "data 1Gi -" for no class, to bind to a volume made by
// hand, such as a hostPath one from More › pv.
func CreatePersistentVolumeClaimActions() []Action {
	return []Action{{
		Key: "c", Name: "claim", About: "name size [class]: class - for none, to bind to a hostPath volume",
		Changes:      true,
		Input:        "name size [class]",
		InputDefault: "data 1Gi",
		Flags:        []Flag{{Name: FlagDryRun}},
		Command:      "kubectl create -f - -n %s # PersistentVolumeClaim " + InputHere,
		Run: func(c *k8s.Client, namespace, input string, flags []Flag) tea.Cmd {
			f := strings.Fields(input)
			name := ""
			if len(f) > 0 {
				name = f[0]
			}
			return Change("Create", namespace+"/"+name, flags, func(ctx context.Context) (string, error) {
				if len(f) < 2 || len(f) > 3 {
					return "", fmt.Errorf("type a name, a size and maybe a class, e.g. data 1Gi")
				}
				class := ""
				if len(f) == 3 {
					class = f[2]
				}
				return "", c.CreatePersistentVolumeClaim(ctx, namespace, f[0], f[1], class, Ticked(flags, FlagDryRun))
			})
		},
	}}
}
