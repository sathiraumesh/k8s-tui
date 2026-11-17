package commands

import (
	"strings"
	"testing"
	"time"

	k8s "k8s-tui/internal/kubernetes"
)

func TestPersistentVolumeRows(t *testing.T) {
	volumes := []k8s.PersistentVolume{
		{Name: "data", Capacity: "1Gi", AccessModes: "RWO", ReclaimPolicy: "Retain", Status: "Bound", Claim: "apps/data", StorageClass: "standard", VolumeMode: "Filesystem", CreatedAt: podsNow.Add(-3 * time.Hour)},
		{Name: "free", Capacity: "5Gi", AccessModes: "RWX", ReclaimPolicy: "Delete", Status: "Available", CreatedAt: podsNow.Add(-5 * time.Minute)},
	}
	got := persistentVolumeRows(volumes, false, podsNow)
	want := []string{
		"data   1Gi   RWO   Retain   Bound       apps/data   standard   <none>   3h",
		"free   5Gi   RWX   Delete   Available   <none>      <none>     <none>   5m",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("rows =\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	if wide := persistentVolumeRows(volumes, true, podsNow); !strings.HasSuffix(wide[0], "Filesystem") {
		t.Errorf("wide = %q", wide[0])
	}
	for status, want := range map[string]Level{"Bound": LevelOK, "Available": LevelOK, "Released": LevelWarn, "Failed": LevelBad} {
		if got := PersistentVolumeStatus.Level(status); got != want {
			t.Errorf("%s: level %d, want %d", status, got, want)
		}
	}
	if s, e, _ := FieldSpan(got[0], PersistentVolumeStatus.Column); got[0][s:e] != "Bound" {
		t.Errorf("status column is %q", got[0][s:e])
	}
}

func TestResourceTabPersistentVolumes(t *testing.T) {
	if tab, _, ok := ResourceTab("persistentvolumes   pv   v1   false   PersistentVolume"); !ok || tab != TabPersistentVolumes {
		t.Errorf("tab = %q, %v", tab, ok)
	}
}
