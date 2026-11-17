package commands

import "testing"

func TestJobArgs(t *testing.T) {
	for in, want := range map[string]string{
		"hello busybox echo hi": "hello --image=busybox -- echo hi",
		"hello busybox":         "hello --image=busybox",
		"hello":                 "hello",
	} {
		if got := jobArgs(in); got != want {
			t.Errorf("jobArgs(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestJobStatus(t *testing.T) {
	for status, want := range map[string]Level{"Complete": LevelOK, "Running": LevelWarn, "Suspended": LevelWarn, "Failed": LevelBad} {
		if got := JobStatus.Level(status); got != want {
			t.Errorf("%s: %d, want %d", status, got, want)
		}
	}
}
