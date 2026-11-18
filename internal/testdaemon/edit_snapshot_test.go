package testdaemon

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/testreport"
)

func runProblems(run testreport.Run) string {
	var out []string
	for _, suite := range run.Suites {
		for _, c := range suite.Cases {
			if c.Problem != nil {
				out = append(out, c.Problem.Message+" "+c.Problem.Detail)
			}
		}
	}
	return strings.Join(out, "\n")
}

func editSnapshotProject(t *testing.T) (root, classPath string) {
	t.Helper()
	root = t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"67.0","packageDirectories":[{"path":"force-app","default":true}]}`)
	classPath = filepath.Join(root, "force-app/main/default/classes/EditTest.cls")
	writeFile(t, classPath, `@IsTest private class EditTest { @IsTest static void passes() { System.assertEquals(2, 1 + 1); } }`)
	writeFile(t, classPath+"-meta.xml", `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>67.0</apiVersion><status>Active</status></ApexClass>`)
	return root, classPath
}

// A save can land after the daemon captured its generation and before the
// watcher applies it. A request in that window used to fail with "source
// snapshot mismatch" because the run checks the captured digests against the
// files on disk. The daemon now applies the drift and runs again.
func TestRunBeforeWatcherAppliesSaveDoesNotReportSnapshotMismatch(t *testing.T) {
	t.Cleanup(apextest.InvalidateRuntimeCaches)
	root, classPath := editSnapshotProject(t)
	d, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	opts := apextest.Options{NoDiskCache: true, Parallelism: 1}
	if run := d.RunOptions(opts); run.Summary().Passed != 1 {
		t.Fatalf("baseline run = %#v", run.Summary())
	}
	writeFile(t, classPath, `@IsTest private class EditTest { /* edit */ @IsTest static void passes() { System.assertEquals(2, 1 + 1); } }`)
	run := d.RunOptions(opts)
	if summary := run.Summary(); summary.Total != 1 || summary.Passed != 1 {
		t.Fatalf("run summary = %#v; problems: %s", summary, runProblems(run))
	}
	// Two saves in a row, the second a body edit, then an immediate request.
	writeFile(t, classPath, `@IsTest private class EditTest { @IsTest static void passes() { System.assertEquals(3, 1 + 1); } }`)
	run = d.RunOptions(opts)
	summary := run.Summary()
	if summary.CompileErrors != 0 || summary.Failed != 1 {
		t.Fatalf("body edit run = %#v; problems: %s", summary, runProblems(run))
	}
	if strings.Contains(runProblems(run), "source snapshot mismatch") {
		t.Fatalf("snapshot mismatch surfaced: %s", runProblems(run))
	}
}

// An unchanged project still reports a real compile error as one.
func TestRunKeepsRealCompileErrorWithoutDrift(t *testing.T) {
	t.Cleanup(apextest.InvalidateRuntimeCaches)
	root, classPath := editSnapshotProject(t)
	writeFile(t, classPath, `@IsTest private class EditTest { @IsTest static void passes() { Integer x = missing; } }`)
	d, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	run := d.RunOptions(apextest.Options{NoDiskCache: true, Parallelism: 1})
	if run.Summary().CompileErrors == 0 {
		t.Fatalf("expected a compile error, got %#v", run.Summary())
	}
	if _, stale := d.refreshStaleGeneration(d.snapshotGeneration()); stale {
		t.Fatal("generation reported stale without any edit")
	}
}
