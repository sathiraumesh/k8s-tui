package kubernetes

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/duration"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/kubectl/pkg/describe"
	"sigs.k8s.io/yaml"
)

// The calls every kind of resource has, written once for all of them, the
// way kubectl does them for any TYPE it is given:
//
//	kubectl get TYPE              ListTable: the columns the server prints
//	kubectl describe TYPE NAME    Describe: kubectl's own describer
//	kubectl get TYPE NAME -o yaml YAML
//	kubectl label TYPE NAME ...   LabelResource
//	kubectl delete TYPE NAME      DeleteResource
//	kubectl edit TYPE NAME        EditResource
//
// The kinds with code of their own (pods.go, deployments.go, ...) use the
// typed clientset; these use the dynamic client and the server's tables, so
// they work for every kind, also those added later or by a CRD.

// ResourceType is one kind of resource, e.g. configmaps (v1) or
// statefulsets (apps/v1).
type ResourceType struct {
	Group      string // "" for the core group
	Version    string // e.g. "v1"
	Resource   string // plural, lower case, e.g. "statefulsets"
	Kind       string // e.g. "StatefulSet"
	Namespaced bool
}

// GVR is the type's group, version and resource, for the dynamic client.
func (r ResourceType) GVR() schema.GroupVersionResource {
	return schema.GroupVersionResource{Group: r.Group, Version: r.Version, Resource: r.Resource}
}

// APIVersion is the type's group/version, e.g. "apps/v1", or "v1" for the
// core group, as `kubectl api-resources` writes it.
func (r ResourceType) APIVersion() string {
	return r.GVR().GroupVersion().String()
}

// Arg is the type as kubectl takes it: its plural, and its group if it has
// one, e.g. "configmaps" or "statefulsets.apps", which no other API's
// resource of the same name can be mistaken for.
func (r ResourceType) Arg() string {
	if r.Group == "" {
		return r.Resource
	}
	return r.Resource + "." + r.Group
}

// path is where the API serves the type's list, in namespace if it is in
// one and namespace isn't "" (all namespaces).
func (r ResourceType) path(namespace string) string {
	p := "/apis/" + r.Group + "/" + r.Version
	if r.Group == "" {
		p = "/api/" + r.Version
	}
	if r.Namespaced && namespace != "" {
		p += "/namespaces/" + namespace
	}
	return p + "/" + r.Resource
}

// Table is a list as `kubectl get` prints it: its columns' names, and a
// row per object.
type Table struct {
	Columns []string
	Rows    []TableRow
}

// TableRow is one object of a Table: where it is, and its cells, in the
// order of the columns, written as kubectl writes them.
type TableRow struct {
	Namespace string // "" for a type that isn't in a namespace
	Name      string
	Cells     []string
}

// tableAccept asks the server for a list as a table, the way kubectl get
// does: the server knows each kind's columns, also a CRD's.
const tableAccept = "application/json;as=Table;v=v1;g=meta.k8s.io,application/json"

// ListTable returns the objects of type r in namespace ("" for all of
// them), as `kubectl get TYPE` prints them, with the -o wide columns if
// wide. The columns are the server's own, so they are always kubectl's.
func (c *Client) ListTable(ctx context.Context, r ResourceType, namespace string, wide bool) (Table, error) {
	if c.config == nil {
		return Table{}, errors.New("listing needs a cluster connection")
	}
	httpClient, err := rest.HTTPClientFor(c.config)
	if err != nil {
		return Table{}, err
	}
	url := strings.TrimSuffix(c.config.Host, "/") + strings.TrimSuffix(c.config.APIPath, "/") + r.path(namespace) + "?includeObject=Metadata"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return Table{}, err
	}
	req.Header.Set("Accept", tableAccept)
	resp, err := httpClient.Do(req)
	if err != nil {
		return Table{}, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return Table{}, err
	}
	if resp.StatusCode != http.StatusOK {
		var status metav1.Status
		if json.Unmarshal(body, &status) == nil && status.Message != "" {
			return Table{}, errors.New(status.Message)
		}
		return Table{}, fmt.Errorf("%s %s", resp.Status, r.Arg())
	}
	return tableFromJSON(body, wide, time.Now())
}

// tableFromJSON reads the server's table. Columns of priority 0 are the
// ones kubectl get prints; the others only with -o wide. A cell is written
// as kubectl writes it: a date as an age ("5m"), nothing as "<none>".
func tableFromJSON(data []byte, wide bool, now time.Time) (Table, error) {
	var t metav1.Table
	if err := json.Unmarshal(data, &t); err != nil {
		return Table{}, fmt.Errorf("reading the server's table: %w", err)
	}
	var shown []int
	var out Table
	for i, col := range t.ColumnDefinitions {
		if col.Priority == 0 || wide {
			shown = append(shown, i)
			out.Columns = append(out.Columns, strings.ToUpper(col.Name))
		}
	}
	for _, row := range t.Rows {
		r := TableRow{Cells: make([]string, 0, len(shown))}
		var m metav1.PartialObjectMetadata
		if json.Unmarshal(row.Object.Raw, &m) == nil {
			r.Namespace, r.Name = m.Namespace, m.Name
		}
		for _, i := range shown {
			var cell any
			if i < len(row.Cells) {
				cell = row.Cells[i]
			}
			r.Cells = append(r.Cells, cellText(cell, t.ColumnDefinitions[i], now))
		}
		out.Rows = append(out.Rows, r)
	}
	return out, nil
}

// cellText writes a table cell as kubectl does.
func cellText(cell any, col metav1.TableColumnDefinition, now time.Time) string {
	switch v := cell.(type) {
	case nil:
		return "<none>"
	case string:
		if v == "" {
			return "<none>"
		}
		if col.Type == "date" {
			if t, err := time.Parse(time.RFC3339, v); err == nil {
				return duration.HumanDuration(now.Sub(t))
			}
		}
		return v
	case float64:
		if v == float64(int64(v)) {
			return fmt.Sprint(int64(v))
		}
		return fmt.Sprint(v)
	default:
		return fmt.Sprint(v)
	}
}

// Describe returns what `kubectl describe TYPE NAME` prints: kubectl's own
// describer for the kinds it has one for, else its generic one, which
// prints every field and the events.
func (c *Client) Describe(_ context.Context, r ResourceType, namespace, name string, showEvents bool) (string, error) {
	if c.config == nil {
		return "", errors.New("describing needs a cluster connection")
	}
	gvk := r.GVR().GroupVersion().WithKind(r.Kind)
	d, ok := describe.DescriberFor(gvk.GroupKind(), c.config)
	if !ok {
		scope := meta.RESTScopeRoot
		if r.Namespaced {
			scope = meta.RESTScopeNamespace
		}
		if d, ok = describe.GenericDescriberFor(&meta.RESTMapping{Resource: r.GVR(), GroupVersionKind: gvk, Scope: scope}, c.config); !ok {
			return "", fmt.Errorf("kubectl can't describe %s", r.Arg())
		}
	}
	return d.Describe(namespace, name, describe.DescriberSettings{ShowEvents: showEvents, ChunkSize: 500})
}

// resource is the dynamic client for type r in namespace.
func (c *Client) resource(r ResourceType, namespace string) (dynamic.ResourceInterface, error) {
	if c.dynamic == nil {
		return nil, errors.New("this needs a cluster connection")
	}
	if r.Namespaced {
		return c.dynamic.Resource(r.GVR()).Namespace(namespace), nil
	}
	return c.dynamic.Resource(r.GVR()), nil
}

// dynamicObjects is a dynamic client as update and its kin take a typed one.
type dynamicObjects struct{ client dynamic.ResourceInterface }

func (d dynamicObjects) Get(ctx context.Context, name string, opts metav1.GetOptions) (*unstructured.Unstructured, error) {
	return d.client.Get(ctx, name, opts)
}

func (d dynamicObjects) Update(ctx context.Context, obj *unstructured.Unstructured, opts metav1.UpdateOptions) (*unstructured.Unstructured, error) {
	return d.client.Update(ctx, obj, opts)
}

func (d dynamicObjects) Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error {
	return d.client.Delete(ctx, name, opts)
}

// objects is the type's objects in namespace, for update, labelObject,
// labelsAndAnnotations and deleteObject (update.go).
func (c *Client) objects(r ResourceType, namespace string) (dynamicObjects, error) {
	client, err := c.resource(r, namespace)
	return dynamicObjects{client}, err
}

// ResourceLabelsAndAnnotations returns the object's labels and annotations,
// as key=value sorted by key.
func (c *Client) ResourceLabelsAndAnnotations(ctx context.Context, r ResourceType, namespace, name string) (labels, annotations []string, err error) {
	objects, err := c.objects(r, namespace)
	if err != nil {
		return nil, nil, err
	}
	return labelsAndAnnotations[unstructured.Unstructured](ctx, objects, name)
}

// YAML returns the object as `kubectl get TYPE NAME -o yaml` prints it,
// without its managedFields, as kubectl leaves them out.
func (c *Client) YAML(ctx context.Context, r ResourceType, namespace, name string) (string, error) {
	client, err := c.resource(r, namespace)
	if err != nil {
		return "", err
	}
	obj, err := client.Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	obj.SetManagedFields(nil)
	out, err := yaml.Marshal(obj.Object)
	return string(out), err
}

// LabelResource sets and removes labels on the object, like `kubectl label
// TYPE NAME env=prod old-`.
func (c *Client) LabelResource(ctx context.Context, r ResourceType, namespace, name string, changes []string, overwrite, dryRun bool) (Change, error) {
	objects, err := c.objects(r, namespace)
	if err != nil {
		return Change{}, err
	}
	return labelObject[unstructured.Unstructured](ctx, objects, name, changes, overwrite, dryRun)
}

// DeleteResource deletes the object, like `kubectl delete TYPE NAME`.
func (c *Client) DeleteResource(ctx context.Context, r ResourceType, namespace, name string, dryRun bool) error {
	objects, err := c.objects(r, namespace)
	if err != nil {
		return err
	}
	return deleteObject(ctx, objects, name, dryRun)
}

// updateResource reads the object, lets edit change a copy, and saves it
// (update, in update.go), for the actions of one kind that change it, e.g.
// suspending a CronJob.
func (c *Client) updateResource(ctx context.Context, r ResourceType, namespace, name string, dryRun bool, edit func(*unstructured.Unstructured) error) (before, after *unstructured.Unstructured, err error) {
	objects, err := c.objects(r, namespace)
	if err != nil {
		return nil, nil, err
	}
	return update(ctx, objects, name, dryRun, edit)
}

// EditResource returns the command that opens the object in an editor,
// like `kubectl edit TYPE NAME [-n NAMESPACE]`.
func (c *Client) EditResource(r ResourceType, namespace, name string) (*exec.Cmd, error) {
	return kubectl(withContext(append([]string{"edit", r.Arg(), name}, namespaceArgs(r, namespace)...), c.context))
}

// namespaceArgs is -n NAMESPACE for a type in a namespace, else nothing.
func namespaceArgs(r ResourceType, namespace string) []string {
	if !r.Namespaced {
		return nil
	}
	return []string{"-n", namespace}
}
