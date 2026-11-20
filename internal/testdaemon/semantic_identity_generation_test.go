package testdaemon

import (
	"path/filepath"
	"sync"
	"testing"

	"github.com/glade-sh/glade/internal/apextest"
)

func TestDaemonConcurrentSemanticIdentityGeneration(t *testing.T) {
	restore := apextest.EnableDiskCacheForTesting()
	t.Cleanup(restore)
	t.Cleanup(apextest.InvalidateRuntimeCaches)
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/ExampleTest.cls"), `@IsTest private class ExampleTest { @IsTest static void passes() { System.assertEquals(1, 1); } }`)
	d, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			run := d.RunOptions(apextest.Options{})
			if run.Summary().Passed != 1 {
				t.Errorf("summary: %+v", run.Summary())
			}
		}()
	}
	wg.Wait()
	apextest.WaitForRuntimeCacheWork()
}

func TestDaemonSemanticIdentityChangesOnReload(t *testing.T) {
	restore := apextest.EnableDiskCacheForTesting()
	t.Cleanup(restore)
	t.Cleanup(apextest.InvalidateRuntimeCaches)
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	path := filepath.Join(root, "force-app/main/default/classes/ExampleTest.cls")
	writeFile(t, path, `@IsTest private class ExampleTest { @IsTest static void passes() { System.assertEquals(1, 1); } }`)
	d, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	if got := d.RunOptions(apextest.Options{}).Summary(); got.Passed != 1 {
		t.Fatalf("original: %+v", got)
	}
	before := d.snapshotGeneration()
	writeFile(t, path, `@IsTest private class ExampleTest { @IsTest static void passes() { System.assertEquals(1, 2); } }`)
	if err := d.Reload(); err != nil {
		t.Fatal(err)
	}
	after := d.snapshotGeneration()
	if before.identity == after.identity {
		t.Fatal("reload reused identity owner")
	}
	if got := d.RunOptions(apextest.Options{}).Summary(); got.Failed != 1 {
		t.Fatalf("reloaded: %+v", got)
	}
	apextest.WaitForRuntimeCacheWork()
}
