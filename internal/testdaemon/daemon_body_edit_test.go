package testdaemon

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/watch"
)

func TestDaemonBodyEditRefreshesGraph(t *testing.T) {
	t.Cleanup(apextest.InvalidateRuntimeCaches)
	root := incrementalProject(t)
	path := filepath.Join(root, "force-app/main/default/classes/Util.cls")
	d, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	refreshes := 0
	d.refreshGraphFn = func(graph *watch.RefGraph, index typesys.Index, changes []watch.Change) (*watch.RefGraph, error) {
		refreshes++
		return graph.Refreshed(index, changes), nil
	}
	writeFile(t, path, `public class Util { public static Integer twice(Integer v) { return v + v; } }`)
	change := watch.Change{Path: path, Op: watch.ChangeModified, Kind: watch.FileKindApexClass, Name: "Util"}
	if err := d.UpdateChanges([]watch.Change{change}); err != nil {
		t.Fatal(err)
	}
	cold, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, source := range []string{"Util", "Caller", "Other", "UtilTest"} {
		input := []watch.Change{{Path: filepath.Join(filepath.Dir(path), source+".cls"), Op: watch.ChangeModified, Kind: watch.FileKindApexClass, Name: source}}
		if got, want := d.SelectAffected(input), cold.SelectAffected(input); !reflect.DeepEqual(got, want) {
			t.Fatalf("selection for %s: got %+v, cold %+v", source, got, want)
		}
	}
	if refreshes != 1 {
		t.Fatalf("body edit refreshed graph %d times, want one incremental refresh", refreshes)
	}
}
