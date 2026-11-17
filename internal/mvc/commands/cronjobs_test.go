package commands

import (
	"slices"
	"testing"
)

func TestSplitCronJob(t *testing.T) {
	for _, tc := range []struct {
		in, name, image, schedule string
		command                   []string
	}{
		{"backup busybox */5 * * * * echo hi", "backup", "busybox", "*/5 * * * *", []string{"echo", "hi"}},
		{"backup busybox @hourly", "backup", "busybox", "@hourly", nil},
		{"backup busybox */5 *", "backup", "busybox", "", nil}, // too few schedule fields
		{"backup", "backup", "", "", nil},
	} {
		n, i, s, c := splitCronJob(tc.in)
		if n != tc.name || i != tc.image || s != tc.schedule || !slices.Equal(c, tc.command) {
			t.Errorf("%q: %q %q %q %q", tc.in, n, i, s, c)
		}
	}
	if got := cronJobArgs("backup busybox */5 * * * * echo hi"); got != "backup --image=busybox --schedule='*/5 * * * *' -- echo hi" {
		t.Errorf("cronJobArgs = %q", got)
	}
}
