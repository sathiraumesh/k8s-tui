package kubernetes

import (
	"fmt"
	"sort"
)

// What labelling a node and labelling a pod share. The labels typed are
// read by parseLabels (nodes.go); applyLabels then changes them the same
// way for both.

// Change is what a change did to something (a node, a pod), or with a dry
// run would have done: its labels or taints before and after it.
type Change struct {
	Before, After []string
}

// applyLabels sets and removes labels in *labels, the labels of a node or
// a pod, as `kubectl label` does. A label that already has a different
// value is only changed with overwrite.
func applyLabels(labels *map[string]string, set []label, remove []string, overwrite bool) error {
	for _, l := range set {
		if old, ok := (*labels)[l.key]; ok && old != l.value && !overwrite {
			return fmt.Errorf("label %q already has a value (%s), and --overwrite is false", l.key, old)
		}
	}
	if *labels == nil {
		*labels = map[string]string{}
	}
	for _, l := range set {
		(*labels)[l.key] = l.value
	}
	for _, key := range remove {
		delete(*labels, key) // like kubectl, removing a label that isn't there is fine
	}
	return nil
}

// pairs is a map of labels or annotations as key=value, sorted by key.
func pairs(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k, v := range m {
		out = append(out, k+"="+v)
	}
	sort.Strings(out)
	return out
}
