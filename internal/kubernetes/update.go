package kubernetes

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Changing one object the way kubectl label, kubectl set image and the
// like do: read it, change a copy, and save that (a GET then a PUT). If
// the object changed in between, the PUT fails with a conflict instead of
// undoing the other change. This is the same for every kind, so it is
// written once here and used by each kind's file.

// getUpdater is what update needs of a typed client, e.g.
// clientset.CoreV1().Pods(namespace): getting and saving one object.
type getUpdater[T any] interface {
	Get(ctx context.Context, name string, opts metav1.GetOptions) (*T, error)
	Update(ctx context.Context, obj *T, opts metav1.UpdateOptions) (*T, error)
}

// copyable is a pointer to an API object, which can copy itself.
type copyable[T any] interface {
	*T
	DeepCopy() *T
}

// update reads the object called name, lets edit change a copy of it, and
// saves that; with dryRun the cluster only checks it (--dry-run=server).
// It returns the object as it was read, and as the cluster answered: as
// saved, or after a dry run as it would have been saved.
func update[T any, P copyable[T]](ctx context.Context, client getUpdater[T], name string, dryRun bool, edit func(*T) error) (before, after *T, err error) {
	before, err = client.Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, nil, err
	}
	edited := P(before).DeepCopy() // edit changes a copy, so before stays as it was
	if err := edit(edited); err != nil {
		return nil, nil, err
	}
	opts := metav1.UpdateOptions{}
	if dryRun {
		opts.DryRun = []string{metav1.DryRunAll}
	}
	after, err = client.Update(ctx, edited, opts)
	if err != nil {
		return nil, nil, err
	}
	return before, after, nil
}

// labeled is a pointer to an API object with labels: every kind has them.
type labeled[T any] interface {
	copyable[T]
	GetLabels() map[string]string
	SetLabels(map[string]string)
}

// labelObject sets and removes labels on the object called name, like
// `kubectl label KIND NAME env=prod old-` (see parseLabels and
// applyLabels). It returns the labels before and after.
func labelObject[T any, P labeled[T]](ctx context.Context, client getUpdater[T], name string, changes []string, overwrite, dryRun bool) (Change, error) {
	set, remove, err := parseLabels(changes)
	if err != nil {
		return Change{}, err
	}
	before, after, err := update[T, P](ctx, client, name, dryRun, func(obj *T) error {
		labels := P(obj).GetLabels()
		if err := applyLabels(&labels, set, remove, overwrite); err != nil {
			return err
		}
		P(obj).SetLabels(labels)
		return nil
	})
	if err != nil {
		return Change{}, err
	}
	return Change{Before: pairs(P(before).GetLabels()), After: pairs(P(after).GetLabels())}, nil
}
