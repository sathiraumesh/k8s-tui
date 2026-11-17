package commands

import "testing"

func TestCanTheyArgs(t *testing.T) {
	for in, want := range map[string]string{
		"user:alice create pods apps":     "create pods -n apps --as=alice",
		"serviceaccount:apps:ci get pods": "get pods --as=system:serviceaccount:apps:ci",
	} {
		if got := canTheyArgs(in); got != want {
			t.Errorf("canTheyArgs(%q) = %q, want %q", in, got, want)
		}
	}
}
