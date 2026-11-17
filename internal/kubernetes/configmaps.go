package kubernetes

import (
	"context"
	"fmt"
	"sort"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation"
)

// What config maps have of their own; the rest (list, describe, edit, ...)
// is the generic calls (generic.go).

// KeyValue is one entry of a config map or secret.
type KeyValue struct {
	Key, Value string
}

// ConfigMapData returns the config map's entries, sorted by key. A binary
// entry's value is "(binary, N bytes)".
func (c *Client) ConfigMapData(ctx context.Context, namespace, name string) ([]KeyValue, error) {
	cm, err := c.clientset.CoreV1().ConfigMaps(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	data := make([]KeyValue, 0, len(cm.Data)+len(cm.BinaryData))
	for k, v := range cm.Data {
		data = append(data, KeyValue{k, v})
	}
	for k, v := range cm.BinaryData {
		data = append(data, KeyValue{k, fmt.Sprintf("(binary, %d bytes)", len(v))})
	}
	sort.Slice(data, func(i, j int) bool { return data[i].Key < data[j].Key })
	return data, nil
}

// parseLiterals reads key=value entries, as `kubectl create configmap
// --from-literal` (and secret) take them. A key may appear once.
func parseLiterals(literals []string) (map[string]string, error) {
	if len(literals) == 0 {
		return nil, fmt.Errorf("no entries given: type key=value, several with spaces")
	}
	data := make(map[string]string, len(literals))
	for _, l := range literals {
		k, v, ok := strings.Cut(l, "=")
		if !ok || k == "" {
			return nil, fmt.Errorf("%q: type key=value", l)
		}
		if errs := validation.IsConfigMapKey(k); len(errs) > 0 {
			return nil, fmt.Errorf("key %q: %s", k, strings.Join(errs, "; "))
		}
		if _, dup := data[k]; dup {
			return nil, fmt.Errorf("key %q is given twice", k)
		}
		data[k] = v
	}
	return data, nil
}

// CreateConfigMap makes a config map from key=value entries, like `kubectl
// create configmap NAME --from-literal=key=value ... -n NAMESPACE`.
func (c *Client) CreateConfigMap(ctx context.Context, namespace, name string, literals []string, dryRun bool) error {
	if errs := validation.IsDNS1123Subdomain(name); len(errs) > 0 {
		return fmt.Errorf("config map name %q: %s", name, strings.Join(errs, "; "))
	}
	data, err := parseLiterals(literals)
	if err != nil {
		return err
	}
	cm := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace}, Data: data}
	_, err = c.clientset.CoreV1().ConfigMaps(namespace).Create(ctx, cm, metav1.CreateOptions{DryRun: dryRunOption(dryRun)})
	return err
}
