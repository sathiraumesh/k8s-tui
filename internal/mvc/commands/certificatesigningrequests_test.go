package commands

import "testing"

func TestCertificateCondition(t *testing.T) {
	for condition, want := range map[string]Level{"Approved,Issued": LevelOK, "Pending": LevelWarn, "Denied": LevelBad, "Approved,Failed": LevelBad} {
		if got := CertificateCondition.Level(condition); got != want {
			t.Errorf("%s: %d, want %d", condition, got, want)
		}
	}
	for _, a := range certificateSigningRequestsView().Actions {
		if a.Key == "a" && a.CommandFor("alice", "", nil) != "kubectl certificate approve alice" {
			t.Errorf("approve = %q", a.CommandFor("alice", "", nil))
		}
	}
}
