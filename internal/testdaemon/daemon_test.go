package testdaemon

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/testreport"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/watch"
)

func TestDaemonUpdateChangesDoesNotPublishFailedFallback(t *testing.T) {
	root := t.TempDir()
	manifestPath := filepath.Join(root, "sfdx-project.json")
	classPath := filepath.Join(root, "force-app/main/default/classes/Stable.cls")
	writeFile(t, manifestPath, `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, classPath, "public class Stable { public void beforeEdit() {} }")
	daemon, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	daemon.mu.RLock()
	beforeIndex := daemon.index
	beforeGraph := watch.BuildReferenceGraph(beforeIndex)
	daemon.mu.RUnlock()

	writeFile(t, manifestPath, "{")
	writeFile(t, classPath, "public class Stable {")
	err = daemon.UpdateChanges([]watch.Change{{Path: classPath, Op: watch.ChangeModified, Kind: watch.FileKindApexClass, Name: "Stable"}})
	if err == nil {
		t.Fatal("UpdateChanges succeeded after authoritative fallback failed")
	}

	daemon.mu.RLock()
	afterIndex := daemon.index
	afterGraph := daemon.graph
	daemon.mu.RUnlock()
	if !reflect.DeepEqual(afterIndex, beforeIndex) {
		t.Errorf("daemon published index after failed fallback:\nafter: %#v\nbefore: %#v", afterIndex, beforeIndex)
	}
	if !reflect.DeepEqual(afterGraph, beforeGraph) {
		t.Errorf("daemon refreshed graph after failed fallback:\nafter: %#v\nbefore: %#v", afterGraph, beforeGraph)
	}
}

func TestDaemonReadsRemainAvailableWhileUpdateBuildIsBlocked(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	d, err := New(root)
	if err != nil {
		t.Fatal(err)
	}

	updateStarted := make(chan struct{})
	releaseUpdate := make(chan struct{})
	d.updateIndexFn = func(previous typesys.Index, _, _ []string) (typesys.Index, error) {
		close(updateStarted)
		<-releaseUpdate
		return previous, nil
	}
	updateDone := make(chan error, 1)
	go func() {
		updateDone <- d.UpdateChanges([]watch.Change{{
			Path: filepath.Join(root, "force-app/main/default/classes/Blocked.cls"),
			Op:   watch.ChangeModified,
			Kind: watch.FileKindApexClass,
			Name: "Blocked",
		}})
	}()
	<-updateStarted

	runDone := make(chan struct{})
	go func() {
		d.RunSelectionContext(context.Background(), apextest.Options{}, watch.TestSelection{Mode: watch.SelectionNone})
		close(runDone)
	}()
	selectDone := make(chan struct{})
	go func() {
		d.SelectAffected(nil)
		close(selectDone)
	}()

	completed := 0
	deadline := time.After(2 * time.Second)
	for completed < 2 {
		select {
		case <-runDone:
			runDone = nil
			completed++
		case <-selectDone:
			selectDone = nil
			completed++
		case <-deadline:
			close(releaseUpdate)
			<-updateDone
			t.Fatalf("daemon reads blocked behind candidate build: completed=%d/2", completed)
		}
	}
	close(releaseUpdate)
	if err := <-updateDone; err != nil {
		t.Fatal(err)
	}
}

func TestDaemonReloadAndUpdateChangesSerializeWriters(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	d, err := New(root)
	if err != nil {
		t.Fatal(err)
	}

	d.mu.RLock()
	updated := d.index
	reloaded := d.index
	d.mu.RUnlock()
	updated.Project.Namespace = "updated"
	reloaded.Project.Namespace = "reloaded"

	updateStarted := make(chan struct{})
	releaseUpdate := make(chan struct{})
	d.updateIndexFn = func(typesys.Index, []string, []string) (typesys.Index, error) {
		close(updateStarted)
		<-releaseUpdate
		return updated, nil
	}
	reloadStarted := make(chan struct{})
	d.loadIndexFn = func(string) (typesys.Index, error) {
		close(reloadStarted)
		return reloaded, nil
	}

	updateDone := make(chan error, 1)
	go func() {
		updateDone <- d.UpdateChanges([]watch.Change{{
			Path: filepath.Join(root, "force-app/main/default/classes/Changed.cls"),
			Op:   watch.ChangeModified,
			Kind: watch.FileKindApexClass,
			Name: "Changed",
		}})
	}()
	<-updateStarted
	reloadDone := make(chan error, 1)
	go func() { reloadDone <- d.Reload() }()

	reloadStartedEarly := false
	select {
	case <-reloadStarted:
		reloadStartedEarly = true
	case <-time.After(200 * time.Millisecond):
	}
	close(releaseUpdate)
	if err := <-updateDone; err != nil {
		t.Fatal(err)
	}
	if !reloadStartedEarly {
		select {
		case <-reloadStarted:
		case <-time.After(2 * time.Second):
			t.Fatal("serialized reload did not start after update completed")
		}
	}
	if err := <-reloadDone; err != nil {
		t.Fatal(err)
	}
	if reloadStartedEarly {
		t.Error("Reload entered its load operation while UpdateChanges was still active")
	}

	d.mu.RLock()
	finalIndex := d.index
	finalGraph := d.graph
	d.mu.RUnlock()
	if !reflect.DeepEqual(finalIndex, reloaded) {
		t.Errorf("last serialized writer did not win:\nfinal: %#v\nwant reload: %#v", finalIndex, reloaded)
	}
	if wantGraph := watch.BuildReferenceGraph(reloaded); !reflect.DeepEqual(finalGraph, wantGraph) {
		t.Errorf("final graph does not match last serialized index:\nfinal: %#v\nwant: %#v", finalGraph, wantGraph)
	}
}

func TestDaemonRunsFilterAgainstWarmProject(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/WarmOneTest.cls"), `
@isTest
private class WarmOneTest {
  @isTest static void passes() { System.assertEquals(2, 1 + 1); }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/WarmTwoTest.cls"), `
@isTest
private class WarmTwoTest {
  @isTest static void passes() { System.assertEquals(3, 1 + 2); }
}
`)

	daemon, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	first := daemon.RunFilter("WarmOneTest")
	if got := first.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("first summary = %#v", got)
	}
	second := daemon.RunFilter("WarmTwoTest")
	if got := second.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("second summary = %#v", got)
	}
}

func TestDaemonRunSelectionNarrowsMultipleDirectClasses(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/WarmOneTest.cls"), `
@isTest
private class WarmOneTest {
  @isTest static void passes() { System.assertEquals(2, 1 + 1); }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/WarmTwoTest.cls"), `
@isTest
private class WarmTwoTest {
  @isTest static void passes() { System.assertEquals(3, 1 + 2); }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/WarmThreeTest.cls"), `
@isTest
private class WarmThreeTest {
  @isTest static void passes() { System.assertEquals(4, 2 + 2); }
}
`)

	daemon, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	run := daemon.RunSelectionContext(context.Background(), apextest.Options{}, watch.TestSelection{
		Mode:        watch.SelectionDirect,
		TestClasses: []string{"WarmOneTest", "WarmTwoTest"},
	})
	if got := run.Summary(); got.Total != 2 || got.Passed != 2 {
		t.Fatalf("summary = %#v run=%#v", got, run)
	}
	if runHasClass(run, "WarmThreeTest") {
		t.Fatalf("run included unselected class WarmThreeTest: %#v", run)
	}
}

func runHasClass(run testreport.Run, className string) bool {
	for _, suite := range run.Suites {
		for _, testCase := range suite.Cases {
			if testCase.ClassName == className {
				return true
			}
		}
	}
	return false
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
