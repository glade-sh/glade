package testdaemon

import (
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/watch"
)

func TestDaemonStartupPublicationIndependentOfBuildOptIn(t *testing.T) {
	root, _ := editSnapshotProject(t)
	for _, optIn := range []string{"", "0", "1"} {
		t.Setenv("GLADE_EXPERIMENTAL_BESIDE_GATE", optIn)
		s, err := NewServer(ServerConfig{Root: root})
		if err != nil {
			t.Fatal(err)
		}
		if err := s.Close(); err != nil {
			t.Fatal(err)
		}
		if got := s.daemon.backgroundDiskPublication; !got {
			t.Errorf("opt-in %q: background publication %t", optIn, got)
		}
	}
}

func TestDaemonBodyEditMentionRecheckIndependentOfBuildOptIn(t *testing.T) {
	for _, optIn := range []string{"", "1"} {
		t.Setenv("GLADE_EXPERIMENTAL_BESIDE_GATE", optIn)
		root := incrementalProject(t)
		d, err := New(root)
		if err != nil {
			t.Fatal(err)
		}
		if err := d.Warm(); err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(root, "force-app/main/default/classes/Util.cls")
		writeFile(t, path, "public class Util { public static Integer twice(Integer v) { return v + v; } }")
		if err := d.UpdateChanges([]watch.Change{{Path: path, Op: watch.ChangeModified, Kind: watch.FileKindApexClass}}); err != nil {
			t.Fatal(err)
		}
		apextest.InvalidateRuntimeCaches()
		if err := d.Warm(); err != nil {
			t.Fatal(err)
		}
		stats := d.semaState.LastStats()
		if !stats.Incremental || stats.Rechecked != 1 {
			t.Errorf("opt-in %q: got %+v, want only the edited type rechecked", optIn, stats)
		}
	}
	t.Cleanup(apextest.InvalidateRuntimeCaches)
}
