package testdaemon

import (
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/watch"
)

func incrementalProject(t *testing.T) (root string) {
	t.Helper()
	root = t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"67.0","packageDirectories":[{"path":"force-app","default":true}]}`)
	const meta = `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>67.0</apiVersion><status>Active</status></ApexClass>`
	for name, body := range map[string]string{
		"Util":     `public class Util { public static Integer twice(Integer v) { return v * 2; } }`,
		"Caller":   `public class Caller { public static Integer run() { return Util.twice(2); } }`,
		"Other":    `public class Other { public static String name() { return 'other'; } }`,
		"UtilTest": `@IsTest private class UtilTest { @IsTest static void passes() { System.assertEquals(4, Caller.run()); } }`,
	} {
		path := filepath.Join(root, "force-app/main/default/classes", name+".cls")
		writeFile(t, path, body)
		writeFile(t, path+"-meta.xml", meta)
	}
	return root
}

// After a one-line body edit the daemon's warm re-checks the edited type and
// reuses the rest, and what it reports equals a daemon that starts from the
// edited project.
func TestWarmAfterBodyEditReusesCachedBodyChecks(t *testing.T) {
	t.Cleanup(apextest.InvalidateRuntimeCaches)
	root := incrementalProject(t)
	util := filepath.Join(root, "force-app/main/default/classes/Util.cls")
	d, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := d.Warm(); err != nil {
		t.Fatalf("first warm: %v", err)
	}
	if stats := d.semaState.LastStats(); stats.Incremental {
		t.Fatalf("first warm stats = %+v, want a full run", stats)
	}
	edits := []struct {
		name, source string
		wantErr      bool
	}{
		{"body edit", `public class Util { public static Integer twice(Integer v) { return v + v; } }`, false},
		{"body error", `public class Util { public static Integer twice(Integer v) { return missing; } }`, true},
		{"body repair", `public class Util { public static Integer twice(Integer v) { return v * 2 + 0; } }`, false},
	}
	for _, edit := range edits {
		writeFile(t, util, edit.source)
		if err := d.UpdateChanges([]watch.Change{{Path: util, Op: watch.ChangeModified, Kind: watch.FileKindApexClass}}); err != nil {
			t.Fatal(err)
		}
		apextest.InvalidateRuntimeCaches()
		warmErr := d.Warm()
		stats := d.semaState.LastStats()
		if !stats.Incremental || stats.Reused == 0 || stats.Rechecked == 0 || stats.Rechecked >= stats.Types {
			t.Fatalf("%s: stats = %+v, want a partial recheck", edit.name, stats)
		}
		if (warmErr != nil) != edit.wantErr {
			t.Fatalf("%s: warm error = %v, want error %v", edit.name, warmErr, edit.wantErr)
		}
		apextest.InvalidateRuntimeCaches()
		fresh, err := New(root)
		if err != nil {
			t.Fatal(err)
		}
		freshErr := fresh.Warm()
		if (warmErr == nil) != (freshErr == nil) || (warmErr != nil && warmErr.Error() != freshErr.Error()) {
			t.Fatalf("%s: incremental warm error %v differs from a fresh daemon's %v", edit.name, warmErr, freshErr)
		}
	}
}
