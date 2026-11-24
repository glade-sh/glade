package testdaemon

import "testing"

func TestDaemonStartupPublicationFollowsOptIn(t *testing.T) {
	root, _ := editSnapshotProject(t)
	for _, optIn := range []string{"", "0", "1"} {
		t.Setenv("GLADE_EXPERIMENTAL_BESIDE_GATE", optIn)
		s, err := NewServer(ServerConfig{Root: root})
		if err != nil {
			t.Fatal(err)
		}
		if got := s.daemon.backgroundDiskPublication; got != (optIn == "1") {
			t.Errorf("opt-in %q: background publication %t", optIn, got)
		}
	}
}
