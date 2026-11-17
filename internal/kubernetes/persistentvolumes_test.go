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

// fakeVolume is a 1Gi ReadWriteOnce hostPath volume with phase.
func fakeVolume(name string, phase corev1.PersistentVolumePhase) *corev1.PersistentVolume {
	return &corev1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: corev1.PersistentVolumeSpec{
			Capacity:                      corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("1Gi")},
			AccessModes:                   []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce, corev1.ReadOnlyMany},
			PersistentVolumeReclaimPolicy: corev1.PersistentVolumeReclaimRetain,
			StorageClassName:              "standard",
			PersistentVolumeSource:        corev1.PersistentVolumeSource{HostPath: &corev1.HostPathVolumeSource{Path: "/mnt/" + name}},
		},
		Status: corev1.PersistentVolumeStatus{Phase: phase},
	}
}

// bound is v with the claim apps/data.
func bound(v *corev1.PersistentVolume) *corev1.PersistentVolume {
	v.Spec.ClaimRef = &corev1.ObjectReference{Namespace: "apps", Name: "data"}
	return v
}

func TestListPersistentVolumes(t *testing.T) {
	cs := fake.NewClientset(bound(fakeVolume("b", corev1.VolumeBound)), fakeVolume("a", corev1.VolumeAvailable))
	got, err := NewClientFromClientset(cs).ListPersistentVolumes(context.Background())
	if err != nil || len(got) != 2 || got[0].Name != "a" {
		t.Fatalf("volumes = %+v, err = %v", got, err)
	}
	if v := got[1]; v.Capacity != "1Gi" || v.AccessModes != "RWO,ROX" || v.ReclaimPolicy != "Retain" || v.Status != "Bound" ||
		v.Claim != "apps/data" || v.StorageClass != "standard" || v.VolumeMode != "Filesystem" {
		t.Errorf("volume = %+v", v)
	}
	if got[0].Claim != "" {
		t.Errorf("an available volume has claim %q", got[0].Claim)
	}
}

func TestDescribePersistentVolume(t *testing.T) {
	cs := fake.NewClientset(bound(fakeVolume("data", corev1.VolumeBound)))
	text, err := NewClientFromClientset(cs).DescribePersistentVolume(context.Background(), "data", true)
	if err != nil || !strings.Contains(text, "data") || !strings.Contains(text, "apps/data") || !strings.Contains(text, "/mnt/data") {
		t.Errorf("describe = %q, err = %v", text, err)
	}
}

func TestSetReclaimPolicy(t *testing.T) {
	cs := fake.NewClientset(fakeVolume("data", corev1.VolumeAvailable))
	c := NewClientFromClientset(cs)
	ch, err := c.SetReclaimPolicy(context.Background(), "data", corev1.PersistentVolumeReclaimDelete, false)
	if err != nil || !slices.Equal(ch.Before, []string{"reclaimPolicy=Retain"}) || !slices.Equal(ch.After, []string{"reclaimPolicy=Delete"}) {
		t.Errorf("change = %+v, err = %v", ch, err)
	}
	if _, err := c.SetReclaimPolicy(context.Background(), "data", "Keep", false); err == nil {
		t.Error("policy Keep: err = nil")
	}
}

func TestMakeAvailable(t *testing.T) {
	cs := fake.NewClientset(bound(fakeVolume("old", corev1.VolumeReleased)), bound(fakeVolume("used", corev1.VolumeBound)), fakeVolume("free", corev1.VolumeAvailable))
	c := NewClientFromClientset(cs)
	if err := c.MakeAvailable(context.Background(), "old", false); err != nil {
		t.Fatal(err)
	}
	if pv, _ := cs.CoreV1().PersistentVolumes().Get(context.Background(), "old", metav1.GetOptions{}); pv.Spec.ClaimRef != nil {
		t.Error("the claim is still there")
	}
	if err := c.MakeAvailable(context.Background(), "used", false); err == nil || !strings.Contains(err.Error(), "apps/data") {
		t.Errorf("a bound volume: err = %v, want it refused", err)
	}
	if err := c.MakeAvailable(context.Background(), "free", false); err == nil {
		t.Error("a free volume: err = nil")
	}
}

func TestLabelAndDeletePersistentVolume(t *testing.T) {
	cs := fake.NewClientset(fakeVolume("data", corev1.VolumeAvailable))
	c := NewClientFromClientset(cs)
	if ch, err := c.LabelPersistentVolume(context.Background(), "data", []string{"tier=fast"}, false, false); err != nil || !slices.Equal(ch.After, []string{"tier=fast"}) {
		t.Errorf("label: %+v, %v", ch, err)
	}
	if err := c.DeletePersistentVolume(context.Background(), "data", false); err != nil {
		t.Fatal(err)
	}
	if _, err := cs.CoreV1().PersistentVolumes().Get(context.Background(), "data", metav1.GetOptions{}); err == nil {
		t.Error("still there")
	}
}

func TestCreateHostPathVolume(t *testing.T) {
	cs := fake.NewClientset()
	c := NewClientFromClientset(cs)
	if err := c.CreateHostPathVolume(context.Background(), "data", "2Gi", "/mnt/data/", false); err != nil {
		t.Fatal(err)
	}
	pv, err := cs.CoreV1().PersistentVolumes().Get(context.Background(), "data", metav1.GetOptions{})
	if err != nil || pv.Spec.HostPath.Path != "/mnt/data" || pv.Spec.Capacity.Storage().String() != "2Gi" ||
		pv.Spec.PersistentVolumeReclaimPolicy != corev1.PersistentVolumeReclaimRetain {
		t.Errorf("volume = %+v, err = %v", pv, err)
	}
	for _, bad := range [][3]string{{"Bad_Name", "1Gi", "/mnt"}, {"x", "lots", "/mnt"}, {"x", "0", "/mnt"}, {"x", "1Gi", "mnt"}} {
		if err := c.CreateHostPathVolume(context.Background(), bad[0], bad[1], bad[2], false); err == nil {
			t.Errorf("%q: err = nil", bad)
		}
	}
}
