package kubernetes

import (
	"context"
	"slices"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

// fakeClaim is a claim for 1Gi in namespace, bound to the volume vol-NAME
// unless pending.
func fakeClaim(namespace, name string, pending bool) *corev1.PersistentVolumeClaim {
	class := "standard"
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources:        corev1.VolumeResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("1Gi")}},
			StorageClassName: &class,
		},
		Status: corev1.PersistentVolumeClaimStatus{Phase: corev1.ClaimPending},
	}
	if !pending {
		pvc.Spec.VolumeName = "vol-" + name
		pvc.Status = corev1.PersistentVolumeClaimStatus{
			Phase:       corev1.ClaimBound,
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Capacity:    corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("1Gi")},
		}
	}
	return pvc
}

func TestListPersistentVolumeClaims(t *testing.T) {
	cs := fake.NewClientset(fakeClaim("b", "data", false), fakeClaim("a", "logs", true), fakeClaim("c", "other", false))
	got, err := NewClientFromClientset(cs).ListPersistentVolumeClaims(context.Background(), "")
	if err != nil || len(got) != 3 || got[0].Name != "logs" {
		t.Fatalf("claims = %+v, err = %v", got, err)
	}
	if c := got[1]; c.Status != "Bound" || c.Volume != "vol-data" || c.Capacity != "1Gi" || c.AccessModes != "RWO" || c.StorageClass != "standard" {
		t.Errorf("bound claim = %+v", c)
	}
	if c := got[0]; c.Status != "Pending" || c.Volume != "" || c.Capacity != "" {
		t.Errorf("pending claim = %+v", c)
	}
	inB, _ := NewClientFromClientset(cs).ListPersistentVolumeClaims(context.Background(), "b")
	if len(inB) != 1 || inB[0].Name != "data" {
		t.Errorf("claims in b = %+v", inB)
	}
}

func TestDescribePersistentVolumeClaim(t *testing.T) {
	cs := fake.NewClientset(fakeClaim("apps", "data", false))
	text, err := NewClientFromClientset(cs).DescribePersistentVolumeClaim(context.Background(), "apps", "data", true)
	if err != nil || !strings.Contains(text, "data") || !strings.Contains(text, "vol-data") {
		t.Errorf("describe = %q, err = %v", text, err)
	}
}

func TestPersistentVolumeClaimUsers(t *testing.T) {
	using := fakePod("apps", "web")
	using.Spec.Volumes = []corev1.Volume{{Name: "d", VolumeSource: corev1.VolumeSource{
		PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: "data"}}}}
	other := fakePod("apps", "api")
	cs := fake.NewClientset(fakeClaim("apps", "data", false), using, other)
	users, err := NewClientFromClientset(cs).PersistentVolumeClaimUsers(context.Background(), "apps", "data")
	if err != nil || !slices.Equal(users, []string{"web (Running) on worker1"}) {
		t.Errorf("users = %q, err = %v", users, err)
	}
}

func TestResizePersistentVolumeClaim(t *testing.T) {
	cs := fake.NewClientset(fakeClaim("apps", "data", false))
	c := NewClientFromClientset(cs)
	ch, err := c.ResizePersistentVolumeClaim(context.Background(), "apps", "data", "2Gi", false)
	if err != nil || !slices.Equal(ch.Before, []string{"storage=1Gi"}) || !slices.Equal(ch.After, []string{"storage=2Gi"}) {
		t.Errorf("change = %+v, err = %v", ch, err)
	}
	for _, size := range []string{"1Gi", "500Mi", "big", "0"} {
		if _, err := c.ResizePersistentVolumeClaim(context.Background(), "apps", "data", size, false); err == nil {
			t.Errorf("resize to %q: err = nil", size)
		}
	}
}

func TestCreatePersistentVolumeClaim(t *testing.T) {
	cs := fake.NewClientset()
	c := NewClientFromClientset(cs)
	for _, tc := range []struct {
		name, class string
		want        *string
	}{
		{"default", "", nil},
		{"none", NoStorageClass, new(string)},
		{"fast", "fast", func() *string { s := "fast"; return &s }()},
	} {
		if err := c.CreatePersistentVolumeClaim(context.Background(), "apps", tc.name, "2Gi", tc.class, false); err != nil {
			t.Fatal(err)
		}
		pvc, _ := cs.CoreV1().PersistentVolumeClaims("apps").Get(context.Background(), tc.name, metav1.GetOptions{})
		got := pvc.Spec.StorageClassName
		if (got == nil) != (tc.want == nil) || (got != nil && *got != *tc.want) {
			t.Errorf("%s: class = %v, want %v", tc.name, got, tc.want)
		}
		if q := pvc.Spec.Resources.Requests[corev1.ResourceStorage]; q.String() != "2Gi" {
			t.Errorf("%s: request = %s", tc.name, q.String())
		}
	}
	if err := c.CreatePersistentVolumeClaim(context.Background(), "apps", "Bad_Name", "1Gi", "", false); err == nil {
		t.Error("bad name: err = nil")
	}
}

func TestLabelAndDeletePersistentVolumeClaim(t *testing.T) {
	cs := fake.NewClientset(fakeClaim("apps", "data", false))
	c := NewClientFromClientset(cs)
	if ch, err := c.LabelPersistentVolumeClaim(context.Background(), "apps", "data", []string{"app=web"}, false, false); err != nil || !slices.Equal(ch.After, []string{"app=web"}) {
		t.Errorf("label: %+v, %v", ch, err)
	}
	if err := c.DeletePersistentVolumeClaim(context.Background(), "apps", "data", false); err != nil {
		t.Fatal(err)
	}
	if _, err := cs.CoreV1().PersistentVolumeClaims("apps").Get(context.Background(), "data", metav1.GetOptions{}); err == nil {
		t.Error("still there")
	}
}
