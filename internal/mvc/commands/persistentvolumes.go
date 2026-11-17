package commands

import (
	"context"
	"fmt"
	"strings"
	"time"

	k8s "k8s-tui/internal/kubernetes"

	tea "github.com/charmbracelet/bubbletea"
	corev1 "k8s.io/api/core/v1"
)

// Commands for persistent volumes (PVs), opened from More: the storage of
// the cluster that claims (PVCs) of a namespace are bound to. A volume
// isn't in a namespace, so its actions are for its name alone. The
// cluster calls are in internal/kubernetes/persistentvolumes.go.

// TabPersistentVolumes is the name of the PV view in More.
const TabPersistentVolumes = "persistentvolumes"

// LoadPersistentVolumes calls the API for the volumes (like `kubectl get
// persistentvolumes`, or with wide, `-o wide`).
func LoadPersistentVolumes(c *k8s.Client, wide bool, _ string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), APITimeout)
		defer cancel()

		volumes, err := c.ListPersistentVolumes(ctx)
		now := time.Now()
		return ItemsLoadedMsg{Tab: TabPersistentVolumes, Rows: persistentVolumeRows(volumes, wide, now), Err: err, At: now, Wide: wide}
	}
}

// persistentVolumeRows turns each volume into one line, with its columns
// lined up, as `kubectl get persistentvolumes` shows them:
//
//	NAME  CAPACITY  ACCESS-MODES  RECLAIM-POLICY  STATUS  CLAIM  STORAGECLASS  REASON  AGE
//
// An empty claim, class or reason is "<none>", so the columns stay in
// their places. With wide, VOLUMEMODE comes after.
func persistentVolumeRows(volumes []k8s.PersistentVolume, wide bool, now time.Time) []string {
	cells := make([][]string, 0, len(volumes))
	for _, v := range volumes {
		row := []string{v.Name, orNone(v.Capacity), orNone(v.AccessModes), orNone(v.ReclaimPolicy), orNone(v.Status),
			orNone(v.Claim), orNone(v.StorageClass), orNone(v.Reason), age(v.CreatedAt, now)}
		if wide {
			row = append(row, v.VolumeMode)
		}
		cells = append(cells, row)
	}
	return AlignColumns(cells)
}

// PersistentVolumeStatus is the STATUS column of the volumes: NAME
// CAPACITY ACCESS-MODES RECLAIM-POLICY STATUS. Bound and Available are
// fine; Released (its claim is gone, the data kept) and Pending are on the
// way; Failed is broken.
var PersistentVolumeStatus = StatusColumn{Column: 4, Level: func(status string) Level {
	switch status {
	case "Bound", "Available":
		return LevelOK
	case "Released", "Pending", "Terminating":
		return LevelWarn
	case "Failed":
		return LevelBad
	}
	return LevelNone
}}

// PersistentVolumeActions are the actions on a volume: enter on one opens
// them. The keys are those of the other tabs for the same action.
func PersistentVolumeActions() []Action {
	return []Action{
		{
			Key: "d", Name: "Describe", About: "everything about the volume: its claim, where its storage is, events",
			Flags:   []Flag{{Name: flagNoEvents}},
			Command: "kubectl describe persistentvolume %s",
			Run: func(c *k8s.Client, target, _ string, flags []Flag) tea.Cmd {
				return Read("Describe", target, func(ctx context.Context) (string, error) {
					return c.DescribePersistentVolume(ctx, target, !Ticked(flags, flagNoEvents))
				})
			},
		},
		{
			Key: "s", Name: "Show", About: "the volume's labels and annotations",
			Command: "kubectl get persistentvolume %s --show-labels",
			Run: func(c *k8s.Client, target, _ string, _ []Flag) tea.Cmd {
				return Read("Show", target, func(ctx context.Context) (string, error) {
					labels, annotations, err := c.PersistentVolumeLabelsAndAnnotations(ctx, target)
					return SectionsText(Section{"Labels", labels}, Section{"Annotations", annotations}), err
				})
			},
		},
		{
			Key: "e", Name: "Edit", About: "the volume's YAML in your editor; a bound one can change little",
			Command: "kubectl edit persistentvolume %s",
			Run: func(c *k8s.Client, target, _ string, _ []Flag) tea.Cmd {
				cmd, err := c.EditPersistentVolume(target)
				return InTerminal("Edit", target, "Edit "+target+": editor closed", false, cmd, err)
			},
		},
		{
			Key: "p", Name: "Reclaim policy", About: "what happens when its claim is deleted: Retain keeps it and its data, Delete deletes it",
			Changes:      true,
			Input:        "policy (Retain or Delete)",
			InputDefault: string(corev1.PersistentVolumeReclaimRetain),
			InputArgs:    reclaimPatch,
			Flags:        []Flag{{Name: FlagDryRun}},
			Command:      "kubectl patch persistentvolume %s",
			Run: func(c *k8s.Client, target, input string, flags []Flag) tea.Cmd {
				return Change("Reclaim policy", target, flags, func(ctx context.Context) (string, error) {
					ch, err := c.SetReclaimPolicy(ctx, target, corev1.PersistentVolumeReclaimPolicy(strings.TrimSpace(input)), Ticked(flags, FlagDryRun))
					return ChangeText("Reclaim policy", ch.Before, ch.After), err
				})
			},
		},
		{
			Key: "a", Name: "Make available", About: "free a Released volume for a new claim, keeping its data",
			Changes: true,
			Flags:   []Flag{{Name: FlagDryRun}},
			Command: `kubectl patch persistentvolume %s --type=json -p '[{"op":"remove","path":"/spec/claimRef"}]'`,
			Run: func(c *k8s.Client, target, _ string, flags []Flag) tea.Cmd {
				return Change("Make available", target, flags, func(ctx context.Context) (string, error) {
					return "", c.MakeAvailable(ctx, target, Ticked(flags, FlagDryRun))
				})
			},
		},
		{
			Key: "l", Name: "Label", About: "set labels (key=value) or remove them (key-); a claim can pick a volume by them",
			Changes: true,
			Input:   "labels",
			Flags:   []Flag{{Name: FlagOverwrite}, {Name: FlagDryRun}},
			Command: "kubectl label persistentvolume %s",
			Run: func(c *k8s.Client, target, input string, flags []Flag) tea.Cmd {
				return Change("Label", target, flags, func(ctx context.Context) (string, error) {
					ch, err := c.LabelPersistentVolume(ctx, target, strings.Fields(input), Ticked(flags, FlagOverwrite), Ticked(flags, FlagDryRun))
					return ChangeText("Labels", ch.Before, ch.After), err
				})
			},
		},
		{
			Key: "r", Name: "Delete", About: "remove the volume; while a claim is bound to it, it waits as Terminating",
			Changes: true,
			Flags:   []Flag{{Name: FlagDryRun}},
			Command: "kubectl delete persistentvolume %s",
			Run: func(c *k8s.Client, target, _ string, flags []Flag) tea.Cmd {
				return Change("Delete", target, flags, func(ctx context.Context) (string, error) {
					return "", c.DeletePersistentVolume(ctx, target, Ticked(flags, FlagDryRun))
				})
			},
		},
	}
}

// reclaimPatch writes the policy as kubectl patch takes it.
func reclaimPatch(policy string) string {
	return fmt.Sprintf(`-p '{"spec":{"persistentVolumeReclaimPolicy":"%s"}}'`, strings.TrimSpace(policy))
}

// CreatePersistentVolumeActions are the own actions of the volumes: making
// a hostPath volume, a directory on the node, for trying things out.
func CreatePersistentVolumeActions() []Action {
	return []Action{{
		Key: "h", Name: "hostPath", About: "a directory on the node, made if missing; for a one-node or kind cluster",
		Changes:      true,
		Input:        "name size path",
		InputDefault: "data 1Gi /mnt/data",
		Flags:        []Flag{{Name: FlagDryRun}},
		Command:      "kubectl create -f - # hostPath PersistentVolume " + InputHere,
		Run: func(c *k8s.Client, _, input string, flags []Flag) tea.Cmd {
			f := strings.Fields(input)
			name := ""
			if len(f) > 0 {
				name = f[0]
			}
			return Change("Create", name, flags, func(ctx context.Context) (string, error) {
				if len(f) != 3 {
					return "", fmt.Errorf("type a name, a size and a path, e.g. data 1Gi /mnt/data")
				}
				return "", c.CreateHostPathVolume(ctx, f[0], f[1], f[2], Ticked(flags, FlagDryRun))
			})
		},
	}}
}
