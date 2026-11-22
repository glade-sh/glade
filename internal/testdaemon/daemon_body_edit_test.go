package testdaemon

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/project"
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
	scope := d.WatchScopeSnapshot(root)
	writeFile(t, path, `public class Util { public static Integer twice(Integer v) { return v + v; } }`)
	change := watch.Change{Path: path, Op: watch.ChangeModified, Kind: watch.FileKindApexClass, Name: "Util"}
	exact, err := d.TryUpdateChanges(context.Background(), []watch.Change{change}, scope)
	if err != nil || !exact {
		t.Fatalf("watch update: exact=%v error=%v", exact, err)
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

func TestDaemonSignatureEditRebuildsGraph(t *testing.T) {
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
	writeFile(t, path, `public class Util { public static Integer twice(Long v) { return 4; } }`)
	change := watch.Change{Path: path, Op: watch.ChangeModified, Kind: watch.FileKindApexClass, Name: "Util"}
	if err := d.UpdateChanges([]watch.Change{change}); err != nil {
		t.Fatal(err)
	}
	if refreshes != 0 {
		t.Fatalf("signature edit used body-only graph refresh %d times", refreshes)
	}
	cold, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	selection := d.SelectAffected([]watch.Change{change})
	if want := cold.SelectAffected([]watch.Change{change}); !reflect.DeepEqual(selection, want) {
		t.Fatalf("selection got %+v, cold %+v", selection, want)
	}
	t.Cleanup(apextest.InvalidateRuntimeCaches)
	for _, noCache := range []bool{true, false} {
		policy := "cache-enabled"
		if noCache {
			policy = "no-cache"
		}
		apextest.InvalidateRuntimeCaches()
		request := NewRunRequestV1(apextest.Options{NoDiskCache: noCache, Parallelism: 1}, "", 0, 0, false)
		run, _, _, err := d.RunRequestV1(context.Background(), request, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		request.ReturnClassShards = true
		request.ShardCount = 2
		_, _, plan, err := d.RunRequestV1(context.Background(), request, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		clearRunDurations(&run)
		data, err := json.Marshal(struct {
			Selection watch.TestSelection
			Plan      *ClassShardPlanV1
			Run       any
		}{selection, plan, run})
		if err != nil {
			t.Fatal(err)
		}
		digest := sha256.Sum256([]byte(strings.ReplaceAll(string(data), root, "<root>")))
		t.Logf("neighbor %s digest: %x", policy, digest)
	}
}

func TestDaemonBodyEditRefreshesChangedReferences(t *testing.T) {
	root := incrementalProject(t)
	path := filepath.Join(root, "force-app/main/default/classes/Util.cls")
	other := filepath.Join(filepath.Dir(path), "Other.cls")
	d, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	change := watch.Change{Path: path, Op: watch.ChangeModified, Kind: watch.FileKindApexClass, Name: "Util"}
	otherChange := watch.Change{Path: other, Op: watch.ChangeModified, Kind: watch.FileKindApexClass, Name: "Other"}
	for i, body := range []string{
		`public class Util { public static Integer twice(Integer v) { if (Other.name() == 'other') return v + v; return 0; } }`,
		`public class Util { public static Integer twice(Integer v) { return v + v; } }`,
	} {
		writeFile(t, path, body)
		if err := d.UpdateChanges([]watch.Change{change}); err != nil {
			t.Fatal(err)
		}
		cold, err := New(root)
		if err != nil {
			t.Fatal(err)
		}
		got, want := d.SelectAffected([]watch.Change{otherChange}), cold.SelectAffected([]watch.Change{otherChange})
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("selection got %+v, cold %+v", got, want)
		}
		if i == 0 && got.Mode != watch.SelectionDirect || i == 1 && got.Mode != watch.SelectionAll {
			t.Fatalf("reference edit %d selected %+v", i, got)
		}
	}
}

func TestDaemonInitializerEditRebuildsGraph(t *testing.T) {
	root := incrementalProject(t)
	path := filepath.Join(root, "force-app/main/default/classes/Util.cls")
	writeFile(t, path, `public class Util { static Integer factor = 2; public static Integer twice(Integer v) { return v * factor; } }`)
	d, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	refreshes := 0
	d.refreshGraphFn = func(graph *watch.RefGraph, index typesys.Index, changes []watch.Change) (*watch.RefGraph, error) {
		refreshes++
		return graph.Refreshed(index, changes), nil
	}
	writeFile(t, path, `public class Util { static Integer factor = 3; public static Integer twice(Integer v) { return v * factor; } }`)
	change := watch.Change{Path: path, Op: watch.ChangeModified, Kind: watch.FileKindApexClass, Name: "Util"}
	if err := d.UpdateChanges([]watch.Change{change}); err != nil {
		t.Fatal(err)
	}
	if refreshes != 0 {
		t.Fatalf("initializer edit used method-body graph refresh %d times", refreshes)
	}
	cold, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := d.SelectAffected([]watch.Change{change}), cold.SelectAffected([]watch.Change{change}); !reflect.DeepEqual(got, want) {
		t.Fatalf("selection got %+v, cold %+v", got, want)
	}
}

func TestDaemonBodyAndMetadataEditRebuildsGraph(t *testing.T) {
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
	metadata := path + "-meta.xml"
	data, err := os.ReadFile(metadata)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, metadata, string(data)+"\n<!-- metadata edit -->\n")
	change := watch.Change{Path: path, Op: watch.ChangeModified, Kind: watch.FileKindApexClass, Name: "Util"}
	if err := d.UpdateChanges([]watch.Change{change}); err != nil {
		t.Fatal(err)
	}
	if refreshes != 0 {
		t.Fatalf("simultaneous metadata edit used method-body graph refresh %d times", refreshes)
	}
	cold, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := d.SelectAffected([]watch.Change{change}), cold.SelectAffected([]watch.Change{change}); !reflect.DeepEqual(got, want) {
		t.Fatalf("selection got %+v, cold %+v", got, want)
	}
}

func TestDaemonBodyEditRejectsChangedIndexFacts(t *testing.T) {
	root := incrementalProject(t)
	path := filepath.Join(root, "force-app/main/default/classes/Util.cls")
	d, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	originalUpdate := d.tryUpdateIndexFn
	updates, refreshes := 0, 0
	d.tryUpdateIndexFn = func(index typesys.Index, changed, deleted []string, p project.Project) (typesys.Index, bool, error) {
		next, exact, err := originalUpdate(index, changed, deleted, p)
		if exact {
			next.SourceNamespaces = []string{"another"}
			updates++
		}
		return next, exact, err
	}
	d.refreshGraphFn = func(graph *watch.RefGraph, index typesys.Index, changes []watch.Change) (*watch.RefGraph, error) {
		refreshes++
		return graph.Refreshed(index, changes), nil
	}
	writeFile(t, path, `public class Util { public static Integer twice(Integer v) { return v + v; } }`)
	change := watch.Change{Path: path, Op: watch.ChangeModified, Kind: watch.FileKindApexClass, Name: "Util"}
	if err := d.UpdateChanges([]watch.Change{change}); err != nil {
		t.Fatal(err)
	}
	if updates != 1 || refreshes != 0 {
		t.Fatalf("changed index facts: exact updates=%d graph refreshes=%d", updates, refreshes)
	}
}
