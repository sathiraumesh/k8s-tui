package kubernetes

import (
	"context"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/rest"
)

var configMaps = ResourceType{Version: "v1", Resource: "configmaps", Kind: "ConfigMap", Namespaced: true}
var statefulSets = ResourceType{Group: "apps", Version: "v1", Resource: "statefulsets", Kind: "StatefulSet", Namespaced: true}
var storageClasses = ResourceType{Group: "storage.k8s.io", Version: "v1", Resource: "storageclasses", Kind: "StorageClass"}

func TestResourceTypeNames(t *testing.T) {
	for _, tc := range []struct {
		r                     ResourceType
		arg, apiVersion, path string
	}{
		{configMaps, "configmaps", "v1", "/api/v1/namespaces/apps/configmaps"},
		{statefulSets, "statefulsets.apps", "apps/v1", "/apis/apps/v1/namespaces/apps/statefulsets"},
		{storageClasses, "storageclasses.storage.k8s.io", "storage.k8s.io/v1", "/apis/storage.k8s.io/v1/storageclasses"},
	} {
		if tc.r.Arg() != tc.arg || tc.r.APIVersion() != tc.apiVersion || tc.r.path("apps") != tc.path {
			t.Errorf("%s: arg %q, apiVersion %q, path %q", tc.r.Resource, tc.r.Arg(), tc.r.APIVersion(), tc.r.path("apps"))
		}
	}
	if p := configMaps.path(""); p != "/api/v1/configmaps" {
		t.Errorf("all namespaces: path %q", p)
	}
}

// tableJSON is a table as the API server sends it for configmaps.
const tableJSON = `{
  "kind": "Table", "apiVersion": "meta.k8s.io/v1",
  "columnDefinitions": [
    {"name": "Name", "type": "string", "priority": 0},
    {"name": "Data", "type": "integer", "priority": 0},
    {"name": "Age", "type": "date", "priority": 0},
    {"name": "Note", "type": "string", "priority": 1}
  ],
  "rows": [
    {"cells": ["app-config", 2, "2026-11-17T09:00:00Z", ""],
     "object": {"kind": "PartialObjectMetadata", "apiVersion": "meta.k8s.io/v1", "metadata": {"name": "app-config", "namespace": "apps"}}}
  ]
}`

func TestTableFromJSON(t *testing.T) {
	now := time.Date(2026, 11, 17, 12, 0, 0, 0, time.UTC)
	table, err := tableFromJSON([]byte(tableJSON), false, now)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(table.Columns, []string{"NAME", "DATA", "AGE"}) {
		t.Errorf("columns = %q", table.Columns)
	}
	if r := table.Rows[0]; r.Namespace != "apps" || r.Name != "app-config" || !slices.Equal(r.Cells, []string{"app-config", "2", "3h"}) {
		t.Errorf("row = %+v", r)
	}
	wide, _ := tableFromJSON([]byte(tableJSON), true, now)
	if len(wide.Columns) != 4 || wide.Rows[0].Cells[3] != "<none>" {
		t.Errorf("wide = %+v", wide)
	}
}

// ListTable asks the server for a table, at the type's path.
func TestListTable(t *testing.T) {
	var gotPath, gotAccept string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotAccept = r.URL.Path, r.Header.Get("Accept")
		w.Write([]byte(tableJSON))
	}))
	defer srv.Close()
	c, err := NewClientFromConfig(&rest.Config{Host: srv.URL}, "")
	if err != nil {
		t.Fatal(err)
	}
	table, err := c.ListTable(context.Background(), configMaps, "apps", false)
	if err != nil || len(table.Rows) != 1 {
		t.Fatalf("table = %+v, err = %v", table, err)
	}
	if gotPath != "/api/v1/namespaces/apps/configmaps" || !strings.Contains(gotAccept, "as=Table") {
		t.Errorf("asked %q with Accept %q", gotPath, gotAccept)
	}
}

func TestListTableError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"kind":"Status","apiVersion":"v1","status":"Failure","message":"configmaps is forbidden"}`))
	}))
	defer srv.Close()
	c, _ := NewClientFromConfig(&rest.Config{Host: srv.URL}, "")
	if _, err := c.ListTable(context.Background(), configMaps, "", false); err == nil || err.Error() != "configmaps is forbidden" {
		t.Errorf("err = %v, want the server's message", err)
	}
}

// dynamicClient is a client whose dynamic client holds a ConfigMap
// apps/app-config labelled app=web.
func dynamicClient() *Client {
	cm := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1", "kind": "ConfigMap",
		"metadata": map[string]any{"name": "app-config", "namespace": "apps", "labels": map[string]any{"app": "web"},
			"managedFields": []any{map[string]any{"manager": "kubectl"}}},
		"data": map[string]any{"mode": "fast"},
	}}
	dyn := dynamicfake.NewSimpleDynamicClient(runtime.NewScheme(), cm)
	return NewClientFromClientsets(nil, dyn)
}

func TestResourceLabelsAndYAML(t *testing.T) {
	c := dynamicClient()
	labels, _, err := c.ResourceLabelsAndAnnotations(context.Background(), configMaps, "apps", "app-config")
	if err != nil || !slices.Equal(labels, []string{"app=web"}) {
		t.Errorf("labels = %q, err = %v", labels, err)
	}
	yaml, err := c.YAML(context.Background(), configMaps, "apps", "app-config")
	if err != nil || !strings.Contains(yaml, "mode: fast") || strings.Contains(yaml, "managedFields") {
		t.Errorf("yaml = %q, err = %v", yaml, err)
	}
}

func TestLabelAndDeleteResource(t *testing.T) {
	c := dynamicClient()
	ch, err := c.LabelResource(context.Background(), configMaps, "apps", "app-config", []string{"env=prod", "app-"}, false, false)
	if err != nil || !slices.Equal(ch.After, []string{"env=prod"}) {
		t.Errorf("label: %+v, %v", ch, err)
	}
	if err := c.DeleteResource(context.Background(), configMaps, "apps", "app-config", false); err != nil {
		t.Fatal(err)
	}
	if _, err := c.dynamic.Resource(configMaps.GVR()).Namespace("apps").Get(context.Background(), "app-config", metav1.GetOptions{}); err == nil {
		t.Error("still there")
	}
}

func TestEditResourceArgs(t *testing.T) {
	pretendKubectl(t, "/bin/kubectl")
	c := &Client{context: "lab"}
	cmd, err := c.EditResource(statefulSets, "apps", "db")
	if want := []string{"/bin/kubectl", "edit", "statefulsets.apps", "db", "-n", "apps", "--context=lab"}; err != nil || !slices.Equal(cmd.Args, want) {
		t.Errorf("args = %q, err = %v", cmd.Args, err)
	}
	cmd, _ = c.EditResource(storageClasses, "", "fast")
	if want := []string{"/bin/kubectl", "edit", "storageclasses.storage.k8s.io", "fast", "--context=lab"}; !slices.Equal(cmd.Args, want) {
		t.Errorf("args = %q", cmd.Args)
	}
}

func TestGenericNeedsConnection(t *testing.T) {
	c := NewClientFromClientset(nil)
	if _, err := c.ListTable(context.Background(), configMaps, "", false); err == nil {
		t.Error("ListTable without a connection: err = nil")
	}
	if _, err := c.YAML(context.Background(), configMaps, "apps", "x"); err == nil {
		t.Error("YAML without a dynamic client: err = nil")
	}
}
