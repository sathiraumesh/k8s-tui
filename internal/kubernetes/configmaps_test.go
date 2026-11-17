package kubernetes

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestConfigMapData(t *testing.T) {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Namespace: "apps", Name: "cfg"},
		Data:       map[string]string{"mode": "fast", "a.yaml": "x: 1\n"},
		BinaryData: map[string][]byte{"logo.png": {1, 2, 3}},
	}
	data, err := NewClientFromClientset(fake.NewClientset(cm)).ConfigMapData(context.Background(), "apps", "cfg")
	want := []KeyValue{{"a.yaml", "x: 1\n"}, {"logo.png", "(binary, 3 bytes)"}, {"mode", "fast"}}
	if err != nil || len(data) != 3 || data[0] != want[0] || data[1] != want[1] || data[2] != want[2] {
		t.Errorf("data = %q, err = %v", data, err)
	}
}

func TestCreateConfigMap(t *testing.T) {
	cs := fake.NewClientset()
	c := NewClientFromClientset(cs)
	if err := c.CreateConfigMap(context.Background(), "apps", "cfg", []string{"mode=fast", "url=http://x?a=b"}, false); err != nil {
		t.Fatal(err)
	}
	cm, err := cs.CoreV1().ConfigMaps("apps").Get(context.Background(), "cfg", metav1.GetOptions{})
	if err != nil || cm.Data["mode"] != "fast" || cm.Data["url"] != "http://x?a=b" {
		t.Errorf("config map = %+v, err = %v", cm, err)
	}
	for _, bad := range [][]string{nil, {"novalue"}, {"=x"}, {"a=1", "a=2"}, {"bad key=1"}} {
		if err := c.CreateConfigMap(context.Background(), "apps", "other", bad, false); err == nil {
			t.Errorf("%q: err = nil", bad)
		}
	}
	if err := c.CreateConfigMap(context.Background(), "apps", "Bad_Name", []string{"a=1"}, false); err == nil {
		t.Error("bad name: err = nil")
	}
}
