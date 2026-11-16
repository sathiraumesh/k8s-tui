package commands

import "testing"

func TestPodStatusLevel(t *testing.T) {
	for status, want := range map[string]Level{
		"Running": LevelOK, "Completed": LevelOK,
		"Pending": LevelWarn, "ContainerCreating": LevelWarn, "Terminating": LevelWarn, "Init:1/2": LevelWarn, "NotReady": LevelWarn,
		"CrashLoopBackOff": LevelBad, "ImagePullBackOff": LevelBad, "Error": LevelBad, "OOMKilled": LevelBad,
		"Evicted": LevelBad, "Init:Error": LevelBad, "ExitCode:3": LevelBad,
		"": LevelNone,
	} {
		if got := PodStatusLevel(status); got != want {
			t.Errorf("%q: level %d, want %d", status, got, want)
		}
	}
}

func TestNodeStatusLevel(t *testing.T) {
	for status, want := range map[string]Level{
		"Ready": LevelOK, "Ready,SchedulingDisabled": LevelWarn, "Unknown": LevelWarn,
		"NotReady": LevelBad, "NotReady,SchedulingDisabled": LevelBad, "loading...": LevelNone,
	} {
		if got := NodeStatusLevel(status); got != want {
			t.Errorf("%q: level %d, want %d", status, got, want)
		}
	}
}

func TestReadyLevel(t *testing.T) {
	for ready, want := range map[string]Level{
		"3/3": LevelOK, "1/3": LevelWarn, "0/3": LevelBad, "0/0": LevelNone, "web": LevelNone, "": LevelNone,
	} {
		if got := ReadyLevel(ready); got != want {
			t.Errorf("%q: level %d, want %d", ready, got, want)
		}
	}
}

func TestFieldSpan(t *testing.T) {
	row := "default   web   1/1   Running   0   3h"
	if s, e, ok := FieldSpan(row, 3); !ok || row[s:e] != "Running" {
		t.Errorf("field 3 = %q, ok = %v", row[s:e], ok)
	}
	if s, e, ok := FieldSpan(row, 0); !ok || row[s:e] != "default" {
		t.Errorf("field 0 = %q", row[s:e])
	}
	if _, _, ok := FieldSpan("loading...", 1); ok {
		t.Error("a row with one field has a field 1")
	}
}
