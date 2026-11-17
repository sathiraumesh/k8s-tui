package commands

import (
	"strings"
	"testing"
	"time"

	k8s "k8s-tui/internal/kubernetes"
)

func TestPersistentVolumeClaimRows(t *testing.T) {
	claims := []k8s.PersistentVolumeClaim{
		{Namespace: "apps", Name: "data", Status: "Bound", Volume: "pvc-1", Capacity: "1Gi", AccessModes: "RWO", StorageClass: "standard", VolumeMode: "Filesystem", CreatedAt: podsNow.Add(-3 * time.Hour)},
		{Namespace: "apps", Name: "logs", Status: "Pending", StorageClass: "standard", CreatedAt: podsNow.Add(-5 * time.Minute)},
	}
	got := persistentVolumeClaimRows(claims, false, true, podsNow)
	want := []string{
		"apps   data   Bound     pvc-1    1Gi      RWO      standard   3h",
		"apps   logs   Pending   <none>   <none>   <none>   standard   5m",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("rows =\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	if one := persistentVolumeClaimRows(claims, true, false, podsNow); !strings.HasPrefix(one[0], "data ") || !strings.HasSuffix(one[0], "Filesystem") {
		t.Errorf("one namespace, wide = %q", one[0])
	}
	for status, want := range map[string]Level{"Bound": LevelOK, "Pending": LevelWarn, "Lost": LevelBad} {
		if got := PersistentVolumeClaimStatus.Level(status); got != want {
			t.Errorf("%s: level %d, want %d", status, got, want)
		}
	}
	// In one namespace the status is column 1; with all, the view adds one.
	one := persistentVolumeClaimRows(claims, false, false, podsNow)
	if s, e, _ := FieldSpan(one[0], PersistentVolumeClaimStatus.Column); one[0][s:e] != "Bound" {
		t.Errorf("status column is %q", one[0][s:e])
	}
}

func TestResourceTabPersistentVolumeClaims(t *testing.T) {
	if tab, _, ok := ResourceTab("persistentvolumeclaims   pvc   v1   true   PersistentVolumeClaim"); !ok || tab != TabPersistentVolumeClaims {
		t.Errorf("tab = %q, %v", tab, ok)
	}
}

func TestResizePatch(t *testing.T) {
	if got := resizePatch(" 2Gi "); got != `-p '{"spec":{"resources":{"requests":{"storage":"2Gi"}}}}'` {
		t.Errorf("resizePatch = %q", got)
	}
}
